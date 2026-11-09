// Package lab 是「故意保留漏洞」的靶场模块，用于教学与复现。
//
// 安全边界（务必理解，别把它当成普通业务代码）：
//  1. 所有路由默认关闭（lab.enabled=false）或仅允许本机访问（lab.allow_remote=false）；
//  2. 靶场数据表与业务表完全隔离，注入/脏数据不会污染真实数据；
//  3. 生产环境部署必须关闭该模块，见 docs/DEPLOY.md 的检查清单。
package lab

import (
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/Eiselle01/govulnlab/internal/config"
	"github.com/Eiselle01/govulnlab/internal/middleware"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
	"github.com/Eiselle01/govulnlab/internal/store"
	"github.com/Eiselle01/govulnlab/internal/webui"
)

type Deps struct {
	Cfg   *config.Config
	Log   *slog.Logger
	Store *store.Store
	Tpl   *webui.Renderer
}

// Handlers 持有靶场页面的依赖与运行时计数。
type Handlers struct {
	Deps
	bruteAttempts atomic.Int64
}

func New(d Deps) *Handlers {
	return &Handlers{Deps: d}
}

// Mount 注册靶场路由。即使 lab 关闭也照样注册，由 guard 统一返回 404，
// 这样路由表是静态可预测的，不依赖启动顺序。
func (h *Handlers) Mount(mux *http.ServeMux) {
	mux.Handle("GET /lab", h.guard(http.HandlerFunc(h.index)))

	mux.Handle("GET /lab/sqli", h.guard(http.HandlerFunc(h.sqliPage)))
	mux.Handle("POST /lab/sqli", h.guard(http.HandlerFunc(h.sqliSubmit)))

	mux.Handle("GET /lab/xss", h.guard(http.HandlerFunc(h.xssPage)))
	mux.Handle("POST /lab/xss", h.guard(http.HandlerFunc(h.xssSubmit)))

	// CSRF 靶场必须登录（要有受害者会话），但故意不加 CSRF Token 校验。
	authPage := middleware.RequireAuth(middleware.ModePage)
	mux.Handle("GET /lab/csrf", h.guard(middleware.Chain(http.HandlerFunc(h.csrfPage), authPage)))
	mux.Handle("POST /lab/csrf/profile", h.guard(middleware.Chain(http.HandlerFunc(h.csrfSubmit), authPage)))
	mux.Handle("GET /lab/csrf/attacker", h.guard(http.HandlerFunc(h.attackerPage)))

	mux.Handle("GET /lab/idor", h.guard(middleware.Chain(http.HandlerFunc(h.idorPage), authPage)))

	mux.Handle("GET /lab/bruteforce", h.guard(http.HandlerFunc(h.brutePage)))
	mux.Handle("POST /lab/bruteforce", h.guard(http.HandlerFunc(h.bruteSubmit)))
}

// guard 是靶场的总开关：
//   - disabled → 404，连“这里有个靶场”都不暴露；
//   - 非本机访问 → 403（避免靶场被公网扫描器当练习目标）。
func (h *Handlers) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.Cfg.Lab.Enabled {
			http.NotFound(w, r)
			return
		}
		if !h.Cfg.Lab.AllowRemote {
			if ip := reqctx.ClientIP(r.Context()); !isLoopback(ip) {
				h.Log.Warn("拒绝非本机访问靶场", "ip", ip, "path", r.URL.Path)
				h.Tpl.Error(w, r, http.StatusForbidden, "靶场仅允许本机访问（127.0.0.1 / ::1）")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopback(ip string) bool {
	if ip == "" || ip == "localhost" {
		return true
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

func (h *Handlers) render(w http.ResponseWriter, r *http.Request, status int, name, title string, data map[string]any) {
	page := webui.NewPage(r, title)
	for k, v := range data {
		page.Data[k] = v
	}
	h.Tpl.Render(w, status, name, page)
}

// index 靶场首页。
func (h *Handlers) index(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "lab_index.html", "漏洞靶场", nil)
}
