package pkg

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// LogMiddleware 请求日志中间件，打印请求信息
func LogMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		clientIP := r.RemoteAddr
		method := r.Method
		path := r.URL.Path

		log.Printf("[REQUEST] ip=%s method=%s path=%s", clientIP, method, path)
		//执行真正的handler业务逻辑
		next(w, r)
		cost := time.Since(start)
		log.Printf("[RESPONSE] path=%s cost=%v", path, cost)
	}
}

// AuthMiddleware 鉴权中间件，检查用户是否登录
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1.读取cookie
		cookie, err := r.Cookie("mysession")
		if err != nil {
			http.Redirect(w, r, "/static/login.html", http.StatusFound)
			return
		}
		sid := cookie.Value

		// 2.redis拿session
		val, err := RedisCli.Get(GlobalCtx, sid).Result()
		if err != nil || err == context.DeadlineExceeded {
			http.Redirect(w, r, "/static/login.html", http.StatusFound)
			return
		}

		var sessionData *SessionData
		err = json.Unmarshal([]byte(val), &sessionData)
		if err != nil {
			http.Redirect(w, r, "/static/login.html", http.StatusFound)
			return
		}

		// 把解析出来的session信息放入context，传给后面handler
		session := SessionData{
			Username:  sessionData.Username,
			CsrfToken: sessionData.CsrfToken,
		}
		ctx := context.WithValue(r.Context(), SessionCtxKey, session)
		// 使用带新ctx的request继续往下走
		next(w, r.WithContext(ctx))
	}
}

// GetSessionFromCtx 在handler内部获取session信息
func GetSessionFromCtx(ctx context.Context) (*SessionData, bool) {
	s, ok := ctx.Value(SessionCtxKey).(SessionData)
	if !ok {
		return nil, false
	}
	return &s, true
}
