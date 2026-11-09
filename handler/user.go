package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
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
		pkg.ErrorJson(w, "请使用post方式提交")
		return
	}
	err := r.ParseForm()
	if err != nil {
		pkg.ErrorJson(w, "解析表单失败")
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		pkg.ErrorJson(w, "用户名或密码不能为空")
		return
	}
	_, err = model.CreateUser(username, password)
	if err != nil {
		pkg.ErrorJson(w, "注册失败，用户名可能已被占用")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `{"ok":true,"msg":"注册成功"}`)
	return
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		pkg.ErrorJson(w, "请使用POST方式提交登录")
		return
	}
	err := r.ParseForm()
	if err != nil {
		pkg.ErrorJson(w, "解析表单失败")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	fmt.Printf("[DEBUG] 收到登录: username=%s, password=%s\n", username, password) //新增打印

	if username == "" || password == "" {
		pkg.ErrorJson(w, "用户名或密码不能为空")
		return

	}

	user, err := model.QueryUserByName(username)
	if err != nil {
		fmt.Println("[DEBUG] 查询用户失败，", err)

		pkg.ErrorJson(w, "用户不存在")
		return
	}
	fmt.Println("[DEBUG] 查询用户成功，数据库用户：", user.Username)

	if !model.CheckPassword(user.Password, password) {
		fmt.Println("[DEBUG] 密码校验失败")
		pkg.ErrorJson(w, "登录失败，密码校验失败，用户名或密码错误")
		return
	}
	fmt.Println("[DEBUG] 密码校验通过，准备session")
	// 后面你原来生成sid、设置cookie代码不动
	sid, err := pkg.GenSessionID()
	fmt.Println("【登录生成sid】sid=", sid)
	if err != nil {

		pkg.ErrorJson(w, "会话id创建失败")
		return
	}
	csrfToken, err := pkg.GenSessionID()
	if err != nil {
		pkg.ErrorJson(w, "会话csrf-token创建失败")
		return
	}

	sessionData := pkg.SessionData{
		UserID:    user.ID,
		Username:  username,
		CsrfToken: csrfToken,
		Role:      user.Role,
	}

	jsonstr, err := json.Marshal(sessionData)
	if err != nil {
		pkg.ErrorJson(w, "会话序列化失败")
		return
	}

	err = pkg.RedisCli.Set(pkg.GlobalCtx, sid, jsonstr, 2*time.Hour).Err() // 设置过期时间为2小时
	fmt.Printf("【登录保存session】sid=%s, setErr=%v\n", sid, err)
	if err != nil {
		pkg.ErrorJson(w, "会话存储失败")
		return
	}
	// 登录成功之后，写审计日志
	err = model.InsertAuditLog(sessionData.UserID, "用户登录", "登陆成功", r.RemoteAddr)
	log.Printf("【审计日志调试】准备写入audit，userID=%d", user.ID)
	if err != nil {
		log.Printf("【审计日志ERROR】err = %v", err)
		log.Printf("警告：审计日志写入失败，但登录成功，不阻断流程")
		// w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// w.Write([]byte("登陆操作写入审计日志失败"))
		// ❗重点：这里不要return、不要弹窗！直接往下走，设置cookie跳转页面
	}
	// 设置Cookie，跳转首页
	http.SetCookie(w, &http.Cookie{
		Name:     "mysession",
		Value:    sid,
		Path:     "/", // 整个网站都携带这个cookie
		Domain:   "localhost",
		HttpOnly: true,
		// SameSite: http.SameSiteLaxMode,
	})
	// http.Redirect(w, r, "/profile", http.StatusFound)
	// 密码不对的时候
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.Write([]byte(`{"ok":true,"msg":"登陆成功"}`))
	return

}

//个人中心
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	// fmt.Println("✅ 我成功进到ProfileHandler了！！")

	w.Header().Set("Content-Type", "text/html;charset=utf-8")

	// rawVal := r.Context().Value(pkg.SessionCtxKey)
	// fmt.Printf("【DEBUG】ctx取出原始值类型: %T, 值=%v\n", rawVal, rawVal)
	session, ok := pkg.GetSessionFromCtx(r.Context())
	// fmt.Printf("【DEBUG】ok=%v, session=%v\n", ok, session)
	if !ok {
		fmt.Println("【DEBUG】!!!! 进入跳转分支")
		http.Redirect(w, r, "/static/login.html", http.StatusFound)
		return
	}
	err := model.InsertAuditLog(session.UserID, "进入个人中心", "查看成功", r.RemoteAddr)
	if err != nil {
		log.Printf("【个人中心审计写入失败】err=%v", err)
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
}
func LogoutHandler(w http.ResponseWriter, r *http.Request) {

	// cookie, err := r.Cookie("mysession")
	// if err == nil {
	// 	sid := cookie.Value

	session, err := pkg.GetSessionFromCtx(r.Context())
	if !err {
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
	if er != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("读取cookie失败"))
		return
	}
	sid := cookie.Value
	//删除redis中的会话，key为sid
	ok := pkg.RedisCli.Del(pkg.GlobalCtx, sid).Err()
	if ok != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("会话删除失败"))
		return
	}
	// 清空浏览器端cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "mysession",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	fmt.Fprintf(w, "已退出登录 <a href=\"/\">返回首页</a>")

	//销毁redis会话之后
	er = model.InsertAuditLog(session.UserID, "退出登录", "退出成功", r.RemoteAddr)
	if er != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("删除会话操作写入日志失败"))
		return
	}
	http.Redirect(w, r, "/static/login.html", http.StatusFound)
}

func PwdPageHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := pkg.GetSessionFromCtx(r.Context())
	if !ok {
		http.Redirect(w, r, "/static/login.html", http.StatusFound)
		return
	}

	// 2. 加载模板文件
	tpl, err := template.ParseFiles("static/pwd.html")
	if err != nil {
		http.Error(w, "模板加载失败"+err.Error(), 500)
		return
	}

	// 3. 往模板传入数据，渲染页面
	data := map[string]any{
		"CsrfToken": session.CsrfToken,
	}
	err = tpl.Execute(w, data)
	if err != nil {
		http.Error(w, "渲染页面失败"+err.Error(), 500)
		return

	}
}

//更改用户密码，危险版😒
// changePwdHandler 修改密码，漏洞版：没有CSRF校验
func ChangePwdHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		pkg.ErrorHtml(w, "禁止GET访问，请通过表单POST提交")
		return
	}
	err := r.ParseForm()
	if err != nil {
		pkg.ErrorHtml(w, "解析表单失败")
		return
	}

	// 1.鉴权：读取cookie拿到sid
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		pkg.ErrorHtml(w, "鉴权失败，未查询到session")
	}
	// CSRF校验，从POST表单拿csrf_token
	serverCsrf := session.CsrfToken
	clientCsrf := r.FormValue("csrf_token")
	if clientCsrf != serverCsrf {
		pkg.ErrorHtml(w, "❌CSRF‑Token校验失败，拒绝请求！")
	}

	username := session.Username
	oldPwd := r.FormValue("oldpwd")
	newPwd := r.FormValue("newpwd")

	if oldPwd == "" || newPwd == "" {
		pkg.ErrorHtml(w, "新旧密码不能为空")
	}

	// 1.查询该用户
	user, err := model.QueryUserByName(username)
	if err != nil {
		pkg.ErrorHtml(w, "未查询到该用户")
	}
	//校验旧密码是否正确
	if !model.CheckPassword(user.Password, oldPwd) {
		pkg.ErrorHtml(w, "旧密码错误")
	}

	// 2.调用model完成密码更新（内部做bcrypt哈希）
	err = model.UpdateUserPwd(username, newPwd)
	if err != nil {
		pkg.ErrorHtml(w, "修改密码失败")
	}
	fmt.Fprintf(w, "✅密码修改成功！<br><a href=\"/profile\">个人中心</a>")
	//写审计日志
	err = model.InsertAuditLog(session.UserID, "修改密码", "修改成功", r.RemoteAddr)
	if err != nil {
		pkg.ErrorHtml(w, "修改密码操作写入审计日志失败")
	}
	fmt.Fprintf(w, "✅密码修改成功！<br><a href=\"/profile\">个人中心</a>")

}
