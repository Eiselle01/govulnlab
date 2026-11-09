package handler

import (
	"fmt"
	"net/http"

	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)
	func GuestbookHandler(w http.ResponseWriter, r *http.Request) {
	//获取get参数 msg，浏览器访问 /guestbook?msg=xxx
	// msg := r.URL.Query().Get("msg")
	//鉴权，必须登录才能留言
	// session, ok := pkg.GetSessionFromCtx(r.Context())
	// if  !ok {
	// 	http.Redirect(w, r, "/static/login.html", http.StatusFound)
	// 	return
	// }
		session, ok := pkg.MustGetSession(w, r)
		if !ok {
			return
		}
	if r.Method == http.MethodPost {
		//POST提交留言：CSRF校验
		err := r.ParseForm()
		if err != nil {
			fmt.Fprintf(w, "解析表单失败")
			return
		}
		serverToken := session.CsrfToken
		clientToken := r.FormValue("csrf_token")
		if serverToken != clientToken {
			fmt.Fprintf(w, "❌CSRF校验失败")
			return
		}

		content := r.FormValue("content")
		if content == "" {
			fmt.Fprintf(w, "留言内容不能为空")
			return
		}
		//写入数据库，漏洞版本：直接原样存储用户输入，不做转义
		err = model.AddMessage(session.Username, content)
		if err != nil {
			fmt.Fprintf(w, "留言失败：%v", err)
			return
		}
		//提交留言成功，记录审计
		_ = model.InsertAuditLog(session.UserID, "提交留言","提交成功", r.RemoteAddr)

		//提交完成刷新页面
		http.Redirect(w, r, "/guestbook", http.StatusFound)
		return
	}

	//GET请求：渲染页面，展示全部留言
	msgList, err := model.ListAllMessage()
	if err != nil {
		fmt.Fprintf(w, "读取留言失败：%v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	//输出表单，埋入csrf_token
	html := `<h2>留言板（存储XSS演示）</h2>
<form method="POST" action="/guestbook">
	<input type="hidden" name="csrf_token" value="` + session.CsrfToken + `">
	<textarea name="content" placeholder="写下你的留言"></textarea>
	<button type="submit">提交留言</button>
</form>
<hr/>`

	//直接把数据库拿出来的content拼进HTML，漏洞点：用户的<script>会直接执行
	for _, item := range msgList {
		html += `<div>
			<b>` + item.Username + `</b>：` + item.Content + `
		</div><br/>`
	}
	fmt.Fprint(w, html)
}

// //关键点：html特殊字符转义
// safeMsg := safehtml.HTMLEscaped(msg).String()
// fmt.Fprintf(w, `<h2>简易留言板（已转义，安全）</h2>`)
// fmt.Fprintf(w, `<p>你的留言：%s</p>`, safeMsg)
// fmt.Fprintf(w, `<p><a href="/">返回首页</a></p>`)
// ⚠️危险！直接把用户输入msg输出到html，没有转义，产生XSS
// fmt.Fprintf(w, `<h2>简易留言板（存在XSS漏洞）</h2>`)
// fmt.Fprintf(w, `<p>你的留言：%s</p>`, msg)
// fmt.Fprintf(w, `<p><a href="/">返回首页</a></p>`)