package main

import (
	"log"
	"net/http"

	"github.com/Eiselle01/go-security-lab/handler"
	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)

// var db *sql.DB

// initDB 初始化数据库

// func registerHandler(w http.ResponseWriter, r *http.Request) {

// func loginHandler(w http.ResponseWriter, r *http.Request) {

// func profileHandler(w http.ResponseWriter, r *http.Request) {

// func logoutHandler(w http.ResponseWriter, r *http.Request) {

func main() {
	// 1.初始化数据库
	_, err := pkg.InitDB()
	if err != nil {
		log.Fatal("数据库初始化失败", err)
	}

	model.InitDB(pkg.DB) // 注入db实例，model层拿到sqlite连接

	err = pkg.InitRedis()
	if err != nil {
		log.Fatal("redis初始化失败", err)
	}
	// fmt.Printf("debug db = %#v\n", pkg.DB) // 打印db对象
	// // defer db.Close() // main函数结束前关闭数据库
	err = model.CreateGuestBookTable(pkg.DB)
	if err != nil {
		log.Fatal("留言板表创建失败：", err)
	}

	err = model.CreateAuditLogTable(pkg.DB)
	if err != nil {
		log.Fatal("审计日志表创建失败：", err)
	}
	// 2.注册路由
	http.HandleFunc("/", pkg.LogMiddleware(handler.RootHandler))
	http.HandleFunc("/register", pkg.LogMiddleware(handler.RegisterHandler))
	http.HandleFunc("/login", pkg.LogMiddleware(handler.LoginHandler))
	http.HandleFunc("/loginBad", pkg.LogMiddleware(handler.LoginBadHandler))
	http.HandleFunc("/profile", pkg.LogMiddleware(pkg.AuthMiddleware(handler.ProfileHandler)))     //个人中心
	http.HandleFunc("/logout", pkg.LogMiddleware(pkg.AuthMiddleware(handler.LogoutHandler)))       //退出登录
	http.HandleFunc("/guestbook", pkg.LogMiddleware(pkg.AuthMiddleware(handler.GuestbookHandler))) //留言板
	http.HandleFunc("/pwd", pkg.LogMiddleware(pkg.AuthMiddleware(handler.PwdPageHandler)))         //csrf测试
	http.HandleFunc("/changePwd", pkg.LogMiddleware(pkg.AuthMiddleware(handler.ChangePwdHandler)))
	http.HandleFunc("/audit", pkg.LogMiddleware(pkg.AuthMiddleware(handler.AuditLogPageHandler)))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// fmt.Println("服务器启动成功,访问localhost:8080 查看")
	// err := http.ListenAndServe(":8080", nil)
	// if err != nil {
	// 	fmt.Printf("服务器启动失败： %v\n", err)
	// }
	log.Println("服务启动 :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
