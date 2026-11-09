package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// SecurityHeaders 统一下发安全响应头。
//
// CSP 使用「每次请求随机 nonce」而不是 'unsafe-inline'：
// 允许本页内联 <script nonce="...">，但注入进来的 <script> 拿不到 nonce 就不会执行，
// 这是 XSS 的最后一道纵深防线（第一道仍是输出转义）。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := randomNonce()

		csp := "default-src 'self'; " +
			"script-src 'self' 'nonce-" + nonce + "'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; " +
			"connect-src 'self'; " +
			"form-action 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'none'; " +
			"object-src 'none'"

		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		// 教学靶场不希望被搜索引擎/扫描器收录，这里只是礼貌性声明，真正的访问控制靠 lab 路由的守卫。
		h.Set("X-Robots-Tag", "noindex, nofollow")

		ctx := reqctx.WithNonce(r.Context(), nonce)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func randomNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// nonce 只是加固手段，真拿不到随机数也不应阻断请求；退化为空 nonce（CSP 仍会拦截注入脚本）。
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(buf)
}
