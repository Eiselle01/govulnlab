package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// Recover 兜住 handler 里的 panic，避免一个请求把整个进程带崩。
// 关键点：日志里要有堆栈，响应里绝不能有堆栈。
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("请求处理 panic",
						"request_id", reqctx.RequestID(r.Context()),
						"method", r.Method,
						"path", r.URL.Path,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					httpx.JSON(w, http.StatusInternalServerError, httpx.Envelope{
						Code:      "internal_error",
						Msg:       "服务器内部错误，请稍后重试",
						RequestID: reqctx.RequestID(r.Context()),
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
