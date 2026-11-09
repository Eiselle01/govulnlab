package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
	"github.com/Eiselle01/govulnlab/internal/session"
)

// LoadSession 从 Cookie 还原会话放进 context。
//
// 注意：它只负责“加载”，不负责“拦截”。拦截交给 RequireAuth，
// 这样公开页面（首页、登录页）也能知道自己是否已登录，从而渲染不同的导航。
func LoadSession(mgr *session.Manager, log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sid, ok := mgr.SID(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			data, err := mgr.Get(r.Context(), sid)
			if err != nil {
				// 会话过期是正常现象，不用 error 级别刷日志。
				if !errors.Is(err, session.ErrNotFound) {
					log.Warn("加载会话失败", "err", err, "request_id", reqctx.RequestID(r.Context()))
				}
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(reqctx.WithSession(r.Context(), data)))
		})
	}
}
