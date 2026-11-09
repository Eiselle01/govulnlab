package handler

import (
	"fmt"
	"net"
	"net/http"
	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)

// 提取客户端IP（分离IP和端口）


func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// 在CreateOrderHandler里面埋审计日志

func AuditLogPageHandler(w http.ResponseWriter, r *http.Request) {
	//鉴权，必须登录
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		return
	}

	msgList, err := model.QueryAuditLogByUser(session.UserID)
	if err != nil {
		fmt.Fprintf(w, "读取操作日志失败：%v", err)
		fmt.Printf("当前登录用户ID：%d\n", session.UserID) //打印session里的userid
		return
	}
	// // 新增这两行！
	// fmt.Println("session.UserID = ", session.UserID)
	// fmt.Println("查询出来日志数量 len(logList) = ", len(msgList))
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	html := `<h2>我的操作审计日志</h2><hr/>`
	for _, v := range msgList {
		html += `<div>
			<span>` + v.CreateTime.Format("2006-01-02 15:04:05") + `</span>
			<span> ｜ ` + v.Detail + `</span>
		</div><br/>`
	}
	if pkg.IsAdmin(session) {
	html += `<a href="/admin/audit">管理员-全局审计日志</a>  `
}
	fmt.Fprint(w, html)
}
