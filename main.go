package main

import (
	"log"
	"net/http"

	"github.com/Eiselle01/go-security-lab/handler"
	mdw "github.com/Eiselle01/go-security-lab/middleware"
	"github.com/Eiselle01/go-security-lab/model"

	//"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)

func main() {
	// 1.初始化数据库
	// _, err := pkg.InitDB()
	// if err != nil {
	// 	log.Fatal("数据库初始化失败", err)
	// }

	// model.InitDB(pkg.DB) // 注入db实例，model层拿到sqlite连接

	// err = pkg.InitRedis()
	// if err != nil {
	// 	log.Fatal("redis初始化失败", err)
	// }

	//1.加载配置
	err := pkg.InitConfig()
	if err != nil {
		panic(err)
	}

	//2.初始化数据库
	sqlDB, err := pkg.InitDB()
	if err != nil {
		panic(err)
	}
	model.InitDB(sqlDB)

	//3.初始化redis
	err = pkg.InitRedis()
	if err != nil {
		panic(err)
	}

	// fmt.Printf("debug db = %#v\n", pkg.DB) // 打印db对象
	// // defer db.Close() // main函数结束前关闭数据库
	// err = model.CreateGuestBookTable(pkg.DB)
	// if err != nil {
	// 	log.Fatal("留言板表创建失败：", err)
	// }

	// err = model.CreateAuditLogTable(pkg.DB)
	// if err != nil {
	// 	log.Fatal("审计日志表创建失败：", err)
	// }
	// 2.注册路由
	http.HandleFunc("/", mdw.LogMiddleware(handler.RootHandler))
	http.HandleFunc("/register", mdw.LogMiddleware(handler.RegisterHandler))
	http.HandleFunc("/login", mdw.LogMiddleware(handler.LoginHandler))
	http.HandleFunc("/loginBad", mdw.LogMiddleware(handler.LoginBadHandler))
	http.HandleFunc("/profile", mdw.LogMiddleware(mdw.AuthMiddleware(handler.ProfileHandler)))     //个人中心
	http.HandleFunc("/logout", mdw.LogMiddleware(mdw.AuthMiddleware(handler.LogoutHandler)))       //退出登录
	http.HandleFunc("/guestbook", mdw.LogMiddleware(mdw.AuthMiddleware(handler.GuestbookHandler))) //留言板
	http.HandleFunc("/pwd", mdw.LogMiddleware(mdw.AuthMiddleware(handler.PwdPageHandler)))         //csrf测试
	http.HandleFunc("/changePwd", mdw.LogMiddleware(mdw.AuthMiddleware(handler.ChangePwdHandler)))
	http.HandleFunc("/audit", mdw.LogMiddleware(mdw.AuthMiddleware(handler.AuditLogPageHandler)))
	//订单相关
	http.HandleFunc("/order/create", mdw.LogMiddleware(mdw.AuthMiddleware(handler.CreateOrderHandler)))
	http.HandleFunc("/order/list", mdw.LogMiddleware(mdw.AuthMiddleware(handler.OrderListHandler)))
	http.HandleFunc("/order/detail", mdw.LogMiddleware(mdw.AuthMiddleware(handler.OrderDetailHandler)))
	//管理员相关操作
	http.HandleFunc("/admin/order", mdw.LogMiddleware(mdw.AuthMiddleware(handler.AdminOrderHandler)))
	http.HandleFunc("/admin/audit", mdw.LogMiddleware(mdw.AuthMiddleware(handler.AdminAuditHandler)))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	log.Println("服务启动 :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
