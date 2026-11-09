package middleware

import (
	"net/http"
	"net/url"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// RequireAuth 拦截未登录请求。
func RequireAuth(mode Mode) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := reqctx.Session(r.Context()); !ok {
				rejectAuth(w, r, mode, http.StatusUnauthorized, "请先登录")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin 在 RequireAuth 之后使用，做角色校验（RBAC 的最简形态）。
func RequireAdmin(mode Mode) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, ok := reqctx.Session(r.Context())
			if !ok {
				rejectAuth(w, r, mode, http.StatusUnauthorized, "请先登录")
				return
			}
			if !s.IsAdmin() {
				if mode == ModeAPI {
					httpx.JSON(w, http.StatusForbidden, httpx.Envelope{
						Code:      "forbidden",
						Msg:       "需要管理员权限",
						RequestID: reqctx.RequestID(r.Context()),
					})
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`<h1>403 需要管理员权限</h1><p><a href="/">返回首页</a></p>`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectAuth(w http.ResponseWriter, r *http.Request, mode Mode, status int, msg string) {
	if mode == ModeAPI {
		httpx.JSON(w, status, httpx.Envelope{
			Code:      "unauthorized",
			Msg:       msg,
			RequestID: reqctx.RequestID(r.Context()),
		})
		return
	}
	// 带上 next 参数，登录后可以回到原页面；登录处理侧会做「只允许站内路径」的白名单校验。
	next := url.QueryEscape(r.URL.RequestURI())
	http.Redirect(w, r, "/login?next="+next, http.StatusFound)
}
