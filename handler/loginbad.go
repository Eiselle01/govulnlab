package handler
import (
	"fmt"
	"net/http"
	"github.com/Eiselle01/go-security-lab/pkg"
)

// loginBad 【危险！SQL注入漏洞演示】字符串拼接SQL，不要用于真实项目
func LoginBadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fmt.Fprintf(w, "请使用POST提交")
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

	// ==========危险！直接字符串拼接SQL！产生注入漏洞==========
	sqlStr := "SELECT password FROM users WHERE username = '" + username + "'"
	row := pkg.DB.QueryRow(sqlStr)

	var dbPwd string
	err = row.Scan(&dbPwd)
	if err != nil {
		fmt.Fprintf(w, "登录失败：用户名不存在")
		return
	}

	if dbPwd == password {
		fmt.Fprintf(w, "✅【危险接口】登录成功！欢迎 %s", username)
	} else {
		fmt.Fprintf(w, "❌【危险接口】密码错误")
	}
}

// loginBadHandler2 真正完整可绕过的SQL注入演示
func LoginBad2Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fmt.Fprintf(w, "请使用POST提交")
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

	// 危险！用户名、密码全部直接字符串拼接进SQL
	sqlStr := "SELECT * FROM users WHERE username = '" + username + "' AND password = '" + password + "'"
	row := pkg.DB.QueryRow(sqlStr)

	var id int
	var dbUser, dbPwd string
	err = row.Scan(&id, &dbUser, &dbPwd)
	if err != nil {
		fmt.Fprintf(w, "登录失败，用户名或密码错误")
		return
	}
	fmt.Fprintf(w, "✅【高危漏洞】完全绕过成功！欢迎 %s", dbUser)
}
