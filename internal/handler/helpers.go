package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Eiselle01/govulnlab/internal/apperr"
	"github.com/Eiselle01/govulnlab/internal/webui"
)

// parseID 解析路径参数（{id}）或查询参数 ?id=。
func parseID(r *http.Request, key string) (int64, error) {
	raw := r.PathValue(key)
	if raw == "" {
		raw = r.URL.Query().Get(key)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("ID 参数不合法")
	}
	return id, nil
}

// sanitizeNext 校验登录后的跳转地址，只允许站内绝对路径。
// 不做这一步就是开放重定向漏洞（CWE-601），攻击者可拿它做钓鱼跳板。
func sanitizeNext(next string) string {
	const fallback = "/profile"
	if next == "" {
		return fallback
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return fallback
	}
	if strings.ContainsAny(next, "\\\r\n") {
		return fallback
	}
	return next
}

// render 统一渲染入口，自动带上 Title / Session / CSRF / Nonce。
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, status int, name, title string, data map[string]any) {
	page := webui.NewPage(r, title)
	for k, v := range data {
		page.Data[k] = v
	}
	h.Tpl.Render(w, status, name, page)
}

// errText 把业务错误转成可展示文案；非业务错误给通用文案，避免泄露内部细节。
func errText(err error) string {
	if ae, ok := apperr.As(err); ok {
		return ae.Msg
	}
	return "服务器内部错误，请稍后重试"
}

// statusOf 取业务错误对应的 HTTP 状态码，缺省 500。
func statusOf(err error) int {
	if ae, ok := apperr.As(err); ok {
		return ae.Status
	}
	return http.StatusInternalServerError
}
