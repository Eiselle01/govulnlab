package handler

import (
	"fmt"
	"net/http"

	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)

func AuditLogPageHandler(w http.ResponseWriter, r *http.Request) {
	//鉴权，必须登录
	session, ok := pkg.GetSessionFromCtx(r.Context())
	if !ok {
		http.Redirect(w, r, "/static/login.html", http.StatusFound)
		return
	}

	msgList, err := model.QueryAuditLogByUser(session.Username)
	if err != nil {
		fmt.Fprintf(w, "读取操作日志失败：%v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	html := `<h2>我的操作审计日志</h2><hr/>`
	for _, v := range msgList {
		html += `<div>
			<span>` + v.CreateTime.Format("2006-01-02 15:04:05") + `</span>
			<span> ｜ ` + v.Action + `</span>
		</div><br/>`
	}
	fmt.Fprint(w, html)
}
