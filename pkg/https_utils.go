package pkg

import (
	"net/http"
)

// HTML 统一设置html响应头
func HTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content‑Type", "text/html; charset=utf‑8")
	_, _ = w.Write([]byte(html))
}

// RedirectLogin 未登录，跳转到登录页
func RedirectLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/login.html", http.StatusFound)
}

// Forbidden 返回403无权访问（用于越权拦截）
func Forbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content‑Type", "text/html; charset=utf‑8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("<h3>❌" + msg + "</h3>"))
}

// ErrorHtml 返回错误页面
func ErrorHtml(w http.ResponseWriter, msg string) {
	HTML(w, `<h3>错误：`+msg+`</h3>`)
}

//Error返回错误json
func ErrorJson(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":false,"msg":"` + msg + `"}`))
}

// MustGetSession 获取session，未登录直接跳转登录，返回 (session,ok)
func MustGetSession(w http.ResponseWriter, r *http.Request) (*SessionData, bool) {
	session, ok := GetSessionFromCtx(r.Context())
	if !ok {
		RedirectLogin(w, r)
		return nil, false
	}
	return session, true
}
