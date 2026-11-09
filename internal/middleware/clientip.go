package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// RequestID 给每个请求一个唯一 ID，回写到响应头和日志里。
// 线上排障时，用户截图里的 request_id 能直接定位到这条日志和审计记录。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			buf := make([]byte, 8)
			if _, err := rand.Read(buf); err == nil {
				id = hex.EncodeToString(buf)
			} else {
				id = "unknown"
			}
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(reqctx.WithRequestID(r.Context(), id)))
	})
}

// ClientIPMiddleware 解析客户端 IP 与 UA 写入上下文。
//
// 只有显式配置 trust_proxy 时才信任 X-Forwarded-For，
// 否则任何人都能伪造该头绕过基于 IP 的限流与审计。
func ClientIPMiddleware(trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := directIP(r.RemoteAddr)
			if trustProxy {
				if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
					first := strings.TrimSpace(strings.Split(xff, ",")[0])
					if first != "" {
						ip = first
					}
				}
			}
			ctx := reqctx.WithClientIP(r.Context(), ip)
			ctx = reqctx.WithUserAgent(ctx, r.UserAgent())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func directIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
