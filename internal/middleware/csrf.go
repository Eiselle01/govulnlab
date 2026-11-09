package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// CSRFGuard 校验 CSRF Token：表单里放 csrf_token 字段，AJAX 用 X-CSRF-Token 头。
//
// 为什么 Token 方案比单靠 SameSite 更可靠：
// SameSite 只是浏览器行为，老浏览器、跨站 GET、以及子域场景都可能失效；
// Token 是服务端可以强校验的东西——攻击者站点读不到受害者的 Token，就伪造不出请求。
//
// 另外做一层 Origin 校验（存在时比对 Host），属于纵深防御。
func CSRFGuard(mode Mode) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			s, ok := reqctx.Session(r.Context())
			if !ok {
				rejectAuth(w, r, mode, http.StatusUnauthorized, "请先登录")
				return
			}

			if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
				rejectCSRF(w, r, mode, "跨站请求被拒绝（Origin 不匹配）")
				return
			}

			token := r.Header.Get("X-CSRF-Token")
			if token == "" && isFormContentType(r.Header.Get("Content-Type")) {
				if err := r.ParseForm(); err == nil {
					token = r.PostFormValue("csrf_token")
				}
			}
			if !tokensEqual(token, s.CSRFToken) {
				rejectCSRF(w, r, mode, "CSRF Token 校验失败，请刷新页面重试")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectCSRF(w http.ResponseWriter, r *http.Request, mode Mode, msg string) {
	if mode == ModeAPI {
		httpx.JSON(w, http.StatusForbidden, httpx.Envelope{
			Code:      "csrf_failed",
			Msg:       msg,
			RequestID: reqctx.RequestID(r.Context()),
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<h1>403 ` + htmlEscape(msg) + `</h1><p><a href="/">返回首页</a></p>`))
}

// tokensEqual 使用常量时间比较，避免通过响应时间侧信道猜 Token。
// 长度不同直接判失败（常量时间比较也要求等长）。
func tokensEqual(a, b string) bool {
	if a == "" || b == "" || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func isFormContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "application/x-www-form-urlencoded") ||
		strings.HasPrefix(ct, "multipart/form-data")
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

// htmlEscape 只用于 403 提示文案，避免把请求里的内容原样反射回去（反射型 XSS）。
func htmlEscape(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(s)
}
