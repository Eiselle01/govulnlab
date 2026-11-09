// Package handler 是 HTTP 层：解析请求、调用 service、渲染页面或返回 JSON。
// 它不写 SQL、不做业务判断，只负责协议转换与状态码语义。
package handler

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/config"
	"github.com/Eiselle01/govulnlab/internal/lab"
	"github.com/Eiselle01/govulnlab/internal/middleware"
	"github.com/Eiselle01/govulnlab/internal/service"
	"github.com/Eiselle01/govulnlab/internal/session"
	"github.com/Eiselle01/govulnlab/internal/store"
	"github.com/Eiselle01/govulnlab/internal/webui"
	"github.com/Eiselle01/govulnlab/web"
)

// Deps 是所有 handler 依赖的集合，由 main 组装后注入。
// 显式依赖注入而不是包级全局变量：测试时可以替换任意一层。
type Deps struct {
	Cfg       *config.Config
	Log       *slog.Logger
	Store     *store.Store
	Sess      *session.Manager
	Tpl       *webui.Renderer
	Auth      *service.AuthService
	Orders    *service.OrderService
	Guestbook *service.GuestbookService
	Audit     *service.AuditService
}

type Handlers struct {
	*Deps
}

// NewRouter 注册所有路由。
//
// 使用 Go 1.22+ 的 ServeMux 方法/路径模式（"POST /orders/{id}/cancel"），
// 不再需要自己解析 path、也不会有“GET 请求打到写接口”的问题。
func NewRouter(d *Deps) http.Handler {
	h := &Handlers{Deps: d}
	mux := http.NewServeMux()

	// 静态资源。fs.Sub 把 web.Static 根下的 static 目录作为文件服务根。
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic("初始化静态资源失败: " + err.Error())
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	authPage := middleware.RequireAuth(middleware.ModePage)
	authAPI := middleware.RequireAuth(middleware.ModeAPI)
	csrfPage := middleware.CSRFGuard(middleware.ModePage)
	csrfAPI := middleware.CSRFGuard(middleware.ModeAPI)
	adminPage := middleware.RequireAdmin(middleware.ModePage)

	// 登录接口单独用一个更严格的令牌桶：约 5 次/分钟。
	// 桶容量刻意大于 「失败锁定阈值」：让「账号级锁定」先触发（更精确的提示），
	// 限流只作为整体兜底，防止有人换账号名把 Redis 和 bcrypt 打满。
	loginLimiter := middleware.NewRateLimiter(d.Store.RDB, d.Log, "login",
		float64(d.Cfg.Security.LoginMaxFail)/60.0, d.Cfg.Security.LoginMaxFail*3)

	// ---- 公开页面 ----
	mux.Handle("GET /{$}", http.HandlerFunc(h.index))
	mux.Handle("GET /login", http.HandlerFunc(h.loginPage))
	mux.Handle("GET /register", http.HandlerFunc(h.registerPage))

	// ---- 认证 ----
	mux.Handle("POST /auth/login", middleware.Chain(http.HandlerFunc(h.loginSubmit), loginLimiter.Middleware()))
	mux.Handle("POST /auth/register", middleware.Chain(http.HandlerFunc(h.registerSubmit), loginLimiter.Middleware()))
	mux.Handle("POST /auth/logout", middleware.Chain(http.HandlerFunc(h.logout), authPage, csrfPage))

	// ---- 个人中心 ----
	mux.Handle("GET /profile", middleware.Chain(http.HandlerFunc(h.profile), authPage))
	mux.Handle("GET /account/password", middleware.Chain(http.HandlerFunc(h.passwordPage), authPage))
	mux.Handle("POST /account/password", middleware.Chain(http.HandlerFunc(h.changePassword), authPage, csrfPage))

	// ---- 订单（页面形态）----
	mux.Handle("GET /orders", middleware.Chain(http.HandlerFunc(h.ordersPage), authPage))
	mux.Handle("POST /orders", middleware.Chain(http.HandlerFunc(h.createOrder), authPage, csrfPage))
	mux.Handle("GET /orders/{id}", middleware.Chain(http.HandlerFunc(h.orderDetailPage), authPage))
	mux.Handle("POST /orders/{id}/cancel", middleware.Chain(http.HandlerFunc(h.cancelOrder), authPage, csrfPage))

	// ---- 订单（JSON API，供脚本/前端联调）----
	mux.Handle("GET /api/orders", middleware.Chain(http.HandlerFunc(h.apiListOrders), authAPI))
	mux.Handle("POST /api/orders", middleware.Chain(http.HandlerFunc(h.apiCreateOrder), authAPI, csrfAPI))
	mux.Handle("GET /api/orders/{id}", middleware.Chain(http.HandlerFunc(h.apiOrderDetail), authAPI))
	mux.Handle("POST /api/orders/{id}/cancel", middleware.Chain(http.HandlerFunc(h.apiCancelOrder), authAPI, csrfAPI))

	// ---- 留言板 ----
	mux.Handle("GET /guestbook", middleware.Chain(http.HandlerFunc(h.guestbookPage), authPage))
	mux.Handle("POST /guestbook", middleware.Chain(http.HandlerFunc(h.guestbookSubmit), authPage, csrfPage))

	// ---- 审计 ----
	mux.Handle("GET /audit", middleware.Chain(http.HandlerFunc(h.auditPage), authPage))

	// ---- 管理后台 ----
	mux.Handle("GET /admin/orders", middleware.Chain(http.HandlerFunc(h.adminOrders), authPage, adminPage))
	mux.Handle("GET /admin/audit", middleware.Chain(http.HandlerFunc(h.adminAudit), authPage, adminPage))

	// ---- 探针 ----
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)

	// ---- 漏洞靶场（自带守卫，可整体关闭）----
	lab.New(lab.Deps{
		Cfg:   d.Cfg,
		Log:   d.Log,
		Store: d.Store,
		Tpl:   d.Tpl,
	}).Mount(mux)

	return mux
}

func (h *Handlers) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// readyz 检查依赖是否可用；K8s/负载均衡用它决定是否把流量打进来。
func (h *Handlers) readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.Ping(r.Context()); err != nil {
		h.Log.Warn("readyz 失败", "err", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("dependency unavailable"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}
