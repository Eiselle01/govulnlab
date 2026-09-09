package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
	// "github.com/google/safehtml"
	// "github.com/redis/go-redis/v9"
	// "golang.org/x/crypto/bcrypt"
)

//根页面
func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
	<h1>安全练习靶场</h1>
	<a href="/static/register.html">前往注册页面</a>
	<a href="/static/login.html">前往登录页面</a>
	<a href="/static/login_bad.html">前往漏洞登录页面(SQL注入)</a>
	<a href="/static/login_bad2.html">前往高危SQL注入页面</a>
	<a href="/static/login_bad2.html">高危漏洞登录（可完整绕过）</a> <br/>
	<a href="/changePwd">修改密码(有CSRF漏洞)</a> <br/>
	`)
}

// 注册接口
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(w, "请使用post")
		return
	}
	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, "解析表单失败")
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		fmt.Fprintf(w, "用户名或密码不能为空")
		return
	}
	err = model.CreateUser(username, password)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "注册失败，用户名可能已被占用", http.StatusConflict)
		return
	}

	// // bcrypt对密码进行哈希处理，存储哈希值而不是明文密码
	// hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	// if err != nil {
	// 	http.Error(w, "密码处理失败", http.StatusInternalServerError)
	// 	return
	// }
	// hashPwd := string(hashBytes)

	// _, err = pkg.DB.Exec(`INSERT INTO users(username,password) VALUES(?, ?)`, username, hashPwd)
	// if err != nil {
	// 	// 插入失败，一般是用户名重复（UNIQUE约束）
	// 	http.Error(w, "注册失败，用户名可能已被占用", http.StatusConflict)
	// 	return
	// }
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "请前往登录页面登录：<a href=\"/static/login.html\">登录</a>")
	return
}

// 登录接口
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fmt.Fprintf(w, "请使用POST方式提交登录")
		return
	}
	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, "表单解析失败")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		fmt.Fprintf(w, "用户名密码不能为空")
		return
	}

	// 去数据库查询这条用户记录，?占位符填入用户名
	user, err := model.QueryUserByName(username)
	if err != nil {
		// 查询不到这个用户
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "登录失败，用户名或密码错误", http.StatusUnauthorized)
		return
	}

	// 判断密码是否相等
	// err = bcrypt.CompareHashAndPassword([]byte(hashPwd), []byte(password))
	if !model.CheckPassword(user.Password, password) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "登录失败，用户名或密码错误", http.StatusUnauthorized)
		return
	}

	sid, err := pkg.GenSessionID()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "会话ID创建失败", http.StatusInternalServerError)
		return
	}
	csrfToken, err := pkg.GenSessionID()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "会话csrf-token创建失败", http.StatusInternalServerError)
		return
	}

	sessionData := pkg.SessionData{
		Username:  username,
		CsrfToken: csrfToken,
	}

	jsonstr, err := json.Marshal(sessionData)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "会话序列化失败", http.StatusInternalServerError)
		return
	}

	err = pkg.RedisCli.Set(pkg.GlobalCtx, sid, jsonstr, 2*time.Hour).Err() // 设置过期时间为2小时
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.Error(w, "会话存储失败", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "mysession",
		Value:    sid,
		Path:     "/", // 整个网站都携带这个cookie
		Domain:   "localhost",
		HttpOnly: true,
		// SameSite: http.SameSiteLaxMode,
	})
	// w.Header().Set("Content-Type", "text/html; charset=utf-8")
	//登录成功，写审计日志
	_ = model.InsertAuditLog(username, "用户登录")
	http.Redirect(w, r, "/profile", http.StatusFound)
	return
	// 设置完cookie、redis写入完成之后
	// w.Header().Set("Content-Type", "text/plain")
	// w.WriteHeader(http.StatusOK)
	// _, _ = w.Write([]byte("ok"))
	// return
	// fmt.Fprintf(w, "✅登录成功！欢迎 %s <br><a href=\"/profile\">访问个人中心</a>", username)
}

// func PwdPageHandler(w http.ResponseWriter, r *http.Request) {
// 	// 读取cookie sid，从redis拿到sessionData
// 	sessionData, ok := getSessionFromRedis(r)
// 	if !ok {
// 		http.Redirect(w, r, "/login.html", http.StatusFound)
// 		return
// 	}
// 	myCsrf := sessionData["csrf_token"]

// 	html := `
// <!DOCTYPE html>
// <html>
// <body>
// <h2>修改密码</h2>
// <form method="POST" action="/changePwd">
// 	<input type="hidden" name="csrf_token" value="` + myCsrf + `">
// 	旧密码:<input type="password" name="oldpwd"><br>
// 	新密码:<input type="password" name="newpwd"><br>
// 	<button type="submit">提交修改</button>
// </form>
// </body>
// </html>
// `
// 	w.Header().Set("Content-Type", "text/html;charset=utf-8")
// 	w.Write([]byte(html))
// }

//个人中心
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	session, ok := pkg.GetSessionFromCtx(r.Context())
	if !ok {
		http.Redirect(w, r, "/static/login.html", http.StatusFound)
		return
	}

	fmt.Fprintf(w, `
	<h2>👤个人中心</h2>
	<p>欢迎用户：%s</p>
	<p><a href="/pwd">修改密码</a></p>
	<p><a href="/logout">退出登录</a></p>
	<p><a href="/guestbook">留言板</a></p>
	<p><a href="/audit">查看我的操作日志</a></p>
	<form method="POST" action="/logout">
		<input type="hidden" name="csrf_token" value="%s">
		<button type="submit">退出登录</button>
	</form>
	`, session.Username, session.CsrfToken)
	//注意：原来的 redisCli → pkg.RedisCli ；globalCtx → pkg.GlobalCtx
	// 读取浏览器带过来的cookie
	// cookie, err := r.Cookie("mysession")
	// if err != nil {
	// 	// cookie不存在，未登录
	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	fmt.Fprintf(w, "🔒未登录，请先<a href=\"/static/login.html\">登录</a>")
	// 	return
	// }
	// sid := cookie.Value
	// 去session map查询
	// val, err := pkg.RedisCli.Get(pkg.GlobalCtx, sid).Result()
	// if err == redis.Nil {
	// 	// key不存在：会话过期 / 登出
	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	fmt.Fprintf(w, "🔒会话无效，请重新<a href=\"/static/login.html\">登录</a>")
	// 	return
	// }
	// if err != nil {
	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	fmt.Fprintf(w, "Redis查询错误")
	// 	return
	// }

	// // json反序列化取出用户名、csrfToken
	// var data map[string]string
	// err = json.Unmarshal([]byte(val), &data)
	// if err != nil {
	// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	fmt.Fprintf(w, "会话解析错误")
	// 	return
	// }
	// username := data["username"]
	// csrfToken := data["csrf_token"]
	// // sessionMutex.Lock()
	// // data, ok := sessionStore[sid]
	// // sessionMutex.Unlock()

	// // if !ok {
	// // 	fmt.Fprintf(w, "🔒会话无效，请重新<a href=\"/static/login.html\">登录</a>")
	// // 	return
	// // }
	// // username := data.Username
	// // // 走到这里：合法已登录用户
	// w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// fmt.Fprintf(w, `<h2>个人中心页面</h2>
	// <p>你已经登录，当前用户名：%s</p>
	// <p>CSRF Token: %s</p>
	// <p><a href="/logout">退出登录</a></p>`, username, csrfToken)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {

	// cookie, err := r.Cookie("mysession")
	// if err == nil {
	// 	sid := cookie.Value

	session, err := pkg.GetSessionFromCtx(r.Context())
	if !err   {
		http.Redirect(w, r, "/static/login.html", http.StatusFound)
		return
	}

	clientCsrf := r.URL.Query().Get("csrf") // 从url获取前端传过来的csrf
	serverCsrf := session.CsrfToken         // 从redis拿到服务端保存的csrf
	if clientCsrf != serverCsrf {
		fmt.Fprintf(w, "❌CSRF‑Token校验失败，拒绝请求！")
		return
	}
	// pkg.RedisCli.Del(pkg.GlobalCtx, sid).Err() // 删除redis中的会话，key为sid
	//从cookie拿到原始sid，redis的key就是sid
	cookie, er := r.Cookie("mysession")
	if er == nil {
		sid := cookie.Value
		//删除redis中的会话，key为sid
		_ = pkg.RedisCli.Del(pkg.GlobalCtx, sid).Err()
	}
	// 	// 删除后端session记录
	// 	sessionMutex.Lock()
	// 	delete(sessionStore, sid)
	// 	sessionMutex.Unlock()
	// }

	// 删除Redis中的会话
	// err = pkg.RedisCli.Del(pkg.GlobalCtx, sid).Err()

	// 清空浏览器端cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "mysession",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	// w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// fmt.Fprintf(w, "已退出登录 <a href=\"/\">返回首页</a>")
	//销毁redis会话之后
	_ = model.InsertAuditLog(session.Username, "退出登录")
	http.Redirect(w, r, "/static/login.html", http.StatusFound)
}

func PwdPageHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := pkg.GetSessionFromCtx(r.Context())
	if !ok {
		http.Redirect(w, r, "/static/login.html", http.StatusFound)
		return
	}

	html := `
<!DOCTYPE html>
<html>
<body>
<h2>修改密码</h2>
<form method="POST" action="/changePwd">
	<input type="hidden" name="csrf_token" value="` + session.CsrfToken + `">
	旧密码:<input type="password" name="oldpwd"><br>
	新密码:<input type="password" name="newpwd"><br>
	<button type="submit">提交修改</button>
</form>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	w.Write([]byte(html))
}

//更改用户密码，危险版😒
// changePwdHandler 修改密码，漏洞版：没有CSRF校验
func ChangePwdHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		fmt.Fprintf(w, "禁止GET访问，请通过表单POST提交")
		return
	}
	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, "解析表单失败")
		return
	}

	// 1.鉴权：读取cookie拿到sid
	session, ok := pkg.GetSessionFromCtx(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "🔒未登录 <a href=\"/static/login.html\">去登录</a>")
		return
	}
	// CSRF校验，从POST表单拿csrf_token
	serverCsrf := session.CsrfToken
	clientCsrf := r.FormValue("csrf_token")
	if clientCsrf != serverCsrf {
		fmt.Fprintf(w, "❌CSRF‑Token校验失败，拒绝请求！")
		return
	}

	username := session.Username
	oldPwd := r.FormValue("oldpwd")
	newPwd := r.FormValue("newpwd")

	if oldPwd == "" || newPwd == "" {
		fmt.Fprintf(w, "旧密码、新密码不能为空")
		return
	}

	// 1.查询该用户
	user, err := model.QueryUserByName(username)
	if err != nil {
		fmt.Fprintf(w, "用户查询失败")
		return
	}
	//校验旧密码是否正确
	if !model.CheckPassword(user.Password, oldPwd) {
		fmt.Fprintf(w, "旧密码错误")
		return
	}

	// 2.调用model完成密码更新（内部做bcrypt哈希）
	err = model.UpdateUserPwd(username, newPwd)
	if err != nil {
		fmt.Fprintf(w, "密码修改失败：%v", err)
		return
	}
	fmt.Fprintf(w, "✅密码修改成功！<br><a href=\"/profile\">个人中心</a>")
	//写审计日志
	_ = model.InsertAuditLog(username, "修改密码")
	fmt.Fprintf(w, "✅密码修改成功！<br><a href=\"/profile\">个人中心</a>")
	// sessionMutex.Lock()
	// data, ok := sessionStore[sid]
	// sessionMutex.Unlock()
	// if !ok {
	// 	fmt.Fprintf(w, "🔒会话失效 <a href=\"/static/login.html\">去登录</a>")
	// 	return
	// }
	// val, err := pkg.RedisCli.Get(pkg.GlobalCtx, sid).Result()
	// if err == redis.Nil {
	// 	fmt.Fprintf(w, "🔒会话失效 <a href=\"/static/login.html\">去登录</a>")
	// 	return
	// }
	// if err != nil {
	// 	fmt.Fprintf(w, "redis查询错误")
	// 	return
	// }
	// //核心：要用csrf验证用户
	// // if data.CsrfToken != r.URL.Query().Get("csrf") {
	// // 	fmt.Fprintf(w, "🔒CSRF验证失败，请不要随意访问此链接")
	// // 	return
	// // }
	// var data map[string]string
	// err = json.Unmarshal([]byte(val), &data)
	// if err != nil {
	// 	fmt.Fprintf(w, "会话解析错误")
	// 	return
	// }
	// username := data["username"]
	// dbCsrf := data["csrf_token"] // 从redis拿到服务端保存的csrf

	// // 从url查询参数拿浏览器传过来的csrf
	// getCsrf := r.URL.Query().Get("csrf")
	// if getCsrf != dbCsrf {
	// 	fmt.Fprintf(w, "❌CSRF‑Token校验失败，拒绝请求！")
	// 	return
	// }

	// // 获取GET参数 newpwd
	// newPwd := r.URL.Query().Get("newpwd")
	// if newPwd == "" {
	// 	fmt.Fprintf(w, `
	// 	<h2>修改密码</h2>
	// 	<p>当前用户：%s</p>
	// 	<p>正确访问示例 /changePwd?newpwd=123456&csrf=%s 即可把密码改成123456</p>
	// 	`, username, dbCsrf)
	// 	return
	// }

	// // 更新数据库用户密码
	// _, err = pkg.DB.Exec(`UPDATE users SET password = ? WHERE username = ?`, newPwd, username)
	// if err != nil {
	// 	fmt.Fprintf(w, "修改失败：%v", err)
	// 	return
	// }
	// fmt.Fprintf(w, "✅密码修改成功！新密码：%s <br><a href=\"/profile\">个人中心</a>", newPwd)
}
