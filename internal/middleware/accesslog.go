package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// statusRecorder 记录真实状态码和响应字节数，供访问日志使用。
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap 让 http.ResponseController 能拿到底层 ResponseWriter（Flush 等）。
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// AccessLog 输出结构化访问日志。
// 只记录必要字段，绝不打印 Cookie、Authorization、请求体（这些最容易泄露凭证）。
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 健康检查由负载均衡高频调用，降到 debug，避免淹没有效日志。
			if strings.HasPrefix(r.URL.Path, "/healthz") || strings.HasPrefix(r.URL.Path, "/readyz") {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Info("http_request",
				"request_id", reqctx.RequestID(r.Context()),
				"ip", reqctx.ClientIP(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", rec.bytes,
				"cost_ms", time.Since(start).Milliseconds(),
				"ua", truncate(r.UserAgent(), 120),
			)
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
