package handler

import (
	"fmt"
	"net/http"

	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)

//查看全部订单
func AdminOrderHandler(w http.ResponseWriter, r *http.Request) {
	// 鉴权
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	// 权限校验：不是管理员直接拦截
	if !pkg.IsAdmin(session) {
		fmt.Fprintf(w, "无权访问，需要管理员权限 <a href='/'>返回首页</a>")
		return
	}
	// ========== 事务：查询全部订单 + 写入管理员审计记录 ==========
	tx, err := pkg.DB.Begin()
	if err != nil {
		fmt.Fprintf(w, "事务开启失败：%v", err)
		return
	}
	defer tx.Rollback()

	// 查询所有订单（不按userid过滤）
	list, err := model.QueryAllOrder()
	if err != nil {
		fmt.Fprintf(w, "查询全部订单失败：%v", err)
		return
	}
	// 写入审计日志：管理员查看全局订单
	actionDetail := fmt.Sprintf("管理员操作：查看系统全部订单列表")
	_, err = tx.Exec(`INSERT INTO audit(user_id, action) VALUES(?,?)`, session.UserID, actionDetail)
	if err != nil {
		fmt.Fprintf(w, "管理员查看全部订单操作写入审计日志失败：%v", err)
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		fmt.Fprintf(w, "事务提交失败：%v", err)
		return
	}
	// ==========================================================

	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	html := `<h2>【管理员】全部订单</h2>
	<a href="/order/list">我的订单</a> | <a href="/admin/audit">全部审计日志</a>
	<hr/>`
	if len(list) == 0 {
		html += `<p>暂无订单</p>`
	} else {
		html += `<table border="1">
		<tr>
			<th>订单ID</th>
			<th>所属用户ID</th>
			<th>订单号</th>
			<th>商品</th>
			<th>金额</th>
			<th>状态</th>
		</tr>`
		for _, o := range list {
			statusText := "正常"
			if o.Status == 2 {
				statusText = "已取消"
			}
			html += fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
				<td>%f</td>
				<td>%s</td>
			</tr>`, o.ID, o.UserID, o.OrderNo, o.GoodsName, o.Amount, statusText)
		}
		html += `</table>`
	}
	fmt.Fprint(w, html)
}

//查看全局审计日志
func AdminAuditHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if !pkg.IsAdmin(session) {
		fmt.Fprintf(w, "无权访问，需要管理员权限 <a href='/'>返回首页</a>")
		return
	}
	// ========== 事务：查询全部订单 + 写入管理员审计记录 ==========
	tx, err := pkg.DB.Begin()
	if err != nil {
		fmt.Fprintf(w, "事务开启失败：%v", err)
		return
	}
	defer tx.Rollback()
	list, err := model.QueryAllAuditLog()
	if err != nil {
		fmt.Fprintf(w, "查询全部审计日志失败：%v", err)
		return
	}
	actionDetail := fmt.Sprintf("管理员操作：查看系统全局审计日志")
	_, err = tx.Exec(`INSERT INTO audit(user_id, action) VALUES(?,?)`, session.UserID, actionDetail)
	if err != nil {
		fmt.Fprintf(w, "管理员查看全局日志操作写入审计日志失败：%v", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fmt.Fprintf(w, "事务提交失败：%v", err)
		return
	}
	// ==========================================================

	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	html := `<h2>【管理员】全局审计日志</h2>
	<a href="/order/list">我的订单</a> | <a href="/admin/order">全部订单</a>
	<hr/>`
	html += `<table border="1">
	<tr>
		<th>日志ID</th>
		<th>用户ID</th>
		<th>操作内容</th>
		<th>操作时间</th>
	</tr>`
	for _, v := range list {
		html += fmt.Sprintf(`
		<tr>
			<td>%d</td>
			<td>%d</td>
			<td>%s</td>
			<td>%s</td>
		</tr>`, v.ID, v.UserID, v.Action, v.CreateTime.Format("2006-01-02 15:04:05"))
	}
	html += `</table>`
	fmt.Fprint(w, html)
}
