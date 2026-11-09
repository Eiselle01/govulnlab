package mdw

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Eiselle01/go-security-lab/pkg"
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
		fmt.Println("【Auth中间件】进入")
		// 1.读取cookie
		cookie, err := r.Cookie("mysession")
		if err != nil {
			fmt.Println("【Auth中间件】读取cookie失败 err=", err)
			http.Redirect(w, r, "/static/login.html", http.StatusFound)
			return
		}
		sid := cookie.Value
		fmt.Println("【Auth中间件】sid=", sid)

		// 2.redis拿session
		val, err := pkg.RedisCli.Get(pkg.GlobalCtx, sid).Result()
		if err != nil {
			fmt.Println("【Auth中间件】读取redis失败 err=", err)
			http.Redirect(w, r, "/static/login.html", http.StatusFound)
			return
		}
		fmt.Println("【Auth中间件】redis获取到原始值:", val)

		var sessionData pkg.SessionData
		err = json.Unmarshal([]byte(val), &sessionData)
		if err != nil {
			fmt.Println("【Auth中间件】json失败", err)

			http.Redirect(w, r, "/static/login.html", http.StatusFound)
			return
		}

		fmt.Printf("【Auth中间件】解析成功 username=%s\n", sessionData.Username)
		// 把解析出来的session信息放入context，传给后面handler

		fmt.Printf("【Auth中间件】解析成功 username=%s, userID=%d\n", sessionData.Username, sessionData.UserID)

		// 直接把解析好的结构体地址放进 ctx，不要再手工拷贝字段
		// // 使用带新ctx的request继续往下走
		ctx := context.WithValue(r.Context(), pkg.SessionCtxKey, &sessionData)
		next(w, r.WithContext(ctx))

	}
}
