// Package webui 封装 html/template 渲染。
//
// 安全要点：整个项目只用 html/template（而不是 text/template + 手工拼接），
// 它会根据上下文自动做 HTML/属性/URL/JS 转义，是防 XSS 的第一道也是最有效的一道防线。
// 靶场为了演示漏洞，会显式使用 template.HTML 绕过转义——那是唯一允许的例外，且必须写注释说明。
package webui

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"html/template"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
	"github.com/Eiselle01/govulnlab/internal/service"
	"github.com/Eiselle01/govulnlab/internal/session"
	"github.com/Eiselle01/govulnlab/web"
)

// Page 是所有页面模板的根数据。
type Page struct {
	Title   string
	Nonce   string
	Session *session.Data
	CSRF    string
	Data    map[string]any
}

type Renderer struct {
	tpl *template.Template
}

// New 解析全部模板并注册模板函数。
func New() (*Renderer, error) {
	funcs := template.FuncMap{
		"money": service.AmountToYuan,
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
	tpl, err := template.New("root").Funcs(funcs).ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("解析页面模板失败: %w", err)
	}
	return &Renderer{tpl: tpl}, nil
}

// NewPage 构造页面数据；handler 只需再往 Data 里塞业务字段。
func NewPage(r *http.Request, title string) *Page {
	p := &Page{
		Title: title,
		Nonce: reqctx.Nonce(r.Context()),
		Data:  map[string]any{},
	}
	if s, ok := reqctx.Session(r.Context()); ok {
		p.Session = s
		p.CSRF = s.CSRFToken
	}
	return p
}

// Render 渲染模板并写出响应。
// 先渲染到 buffer 再写 header，保证模板出错时还能返回一个正常的 500，而不是半个页面。
func (r *Renderer) Render(w http.ResponseWriter, status int, name string, page *Page) {
	var buf bytes.Buffer
	if err := r.tpl.ExecuteTemplate(&buf, name, page); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// Error 渲染统一错误页。
func (r *Renderer) Error(w http.ResponseWriter, rq *http.Request, status int, msg string) {
	page := NewPage(rq, "出错了")
	page.Data["Status"] = status
	page.Data["Message"] = msg
	r.Render(w, status, "error.html", page)
}
