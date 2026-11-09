package middleware

import (
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// BodyLimit 限制请求体大小，防止大 Body 打爆内存（DoS）。
// 先看 Content-Length 做快速拒绝，再用 MaxBytesReader 兜住 chunked / 谎报长度的请求。
func BodyLimit(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				httpx.JSON(w, http.StatusRequestEntityTooLarge, httpx.Envelope{
					Code:      "payload_too_large",
					Msg:       "请求体过大",
					RequestID: reqctx.RequestID(r.Context()),
				})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
