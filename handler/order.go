package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"encoding/json"
	"strings"

	"github.com/Eiselle01/go-security-lab/model"
	"github.com/Eiselle01/go-security-lab/pkg"
)

// 统一返回结构体
type Result struct {
	Code int         `json:"code"` // 0成功，非0失败
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// 订单创建请求结构体
type CreateOrderReq struct {
	GoodsName string  `json:"goodsname"`
	Amount    float64 `json:"amount"`
}

func CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	// 设置返回类型为 json
	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	//鉴权
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		json.NewEncoder(w).Encode(Result{Code: 401, Msg: "未登录"})
		return
	}

	// 只接受POST请求
	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(Result{Code: 405, Msg: "请求方法错误"})
		return
	}

	// 解析前端传来的JSON
	var req CreateOrderReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		json.NewEncoder(w).Encode(Result{Code: 400, Msg: "参数解析失败"})
		return
	}

	// 参数校验
	req.GoodsName = strings.TrimSpace(req.GoodsName)
	if req.GoodsName == "" || req.Amount <= 0 {
		json.NewEncoder(w).Encode(Result{Code: 400, Msg: "商品名不能为空，金额必须大于0"})
		return
	}

	// 创建订单
	order, err := model.CreateOrder(session.UserID, req.GoodsName, req.Amount)
	if err != nil {
		json.NewEncoder(w).Encode(Result{Code: 500, Msg: "创建订单失败：" + err.Error()})
		return
	}
	ip := getClientIP(r)
	err = model.InsertAuditLog(session.UserID, "create_order", fmt.Sprintf("创建订单，商品:%s,数量:%f", req.GoodsName, req.Amount), ip)
	if err != nil {
		// 审计日志写入失败，打印日志，不要阻断主订单业务
		fmt.Printf("创建订单操作写入审计日志失败: %v\n", err)
	}
	// 成功返回JSON，没有任何HTML！
	json.NewEncoder(w).Encode(Result{
		Code: 0,
		Msg:  "订单创建成功",
		Data: order,
	})
}

// CreateOrderHandler 创建订单
/* func CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html;charset=utf-8")

	// 从context拿登录session，AuthMiddleware已经完成登录校验
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		http.Redirect(w,r,"/login",http.StatusFound)
		return
	}
if r.Method == http.MethodPost {
		// 获取表单参数
		err := r.ParseForm()
		if err != nil {
			fmt.Fprintf(w, "参数解析失败：%v", err)
			return
		}
	goodsName := strings.TrimSpace(r.Form.Get("goodsname"))
		amountStr := strings.TrimSpace(r.Form.Get("amount"))

		// 参数校验
		if goodsName == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "商品名称不能为空")
			return
		}
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "金额必须是合法整数")
			return
		}
		if amount <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "金额必须大于0")
			return
		}


		// 调用创建订单函数
		order, err := model.CreateOrder(session.UserID, goodsName, amount)
		if err != nil {
			fmt.Fprintf(w, "创建订单失败：%v", err)
			return
		}
		// 创建成功，跳转到订单列表
		fmt.Fprintf(w, "订单创建成功！订单号：%s <a href='/order/list'>查看订单</a>", order.OrderNo)
		return
	}

	// GET：渲染表单页面
	html := `
	<h2>新建订单</h2>
	<form method="post">
	商品名称：<input name="goodsname"><br/>
	金额：<input name="amount"><br/>
	<button type="submit">提交创建</button>
	</form>
	`
	fmt.Fprint(w, html)

}
*/

// OrderListHandler 当前用户的订单列表
func OrderListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html;charset=utf-8")

	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// 查询该用户自己所有订单
	list, err := model.ListUserOrder(session.UserID)
	if err != nil {
		pkg.ErrorHtml(w, "查询订单失败："+err.Error())
		return
	}

	html := `<h2>我的订单列表</h2>
	<a href="/order/create">新建订单</a> | <a href="/audit">查看操作审计日志</a>
	<hr/>`
	// for _, item := range list {
	// 	html += fmt.Sprintf(`
	// 	<div>
	// 		id:%d｜%s｜金额：%d元
	// 		<a href="/order/detail?id=%d">查看详情</a>
	// 	</div>
	// 	<br/>
	// 	`, item.ID, item.GoodsName, item.Amount, item.UserID)
	// }if len(orderList) == 0 {
	html += `<p>暂无订单</p>`
	if len(list) == 0 {
		html += `<p>暂无订单</p>`
	} else {
		html += `<table border="1">
		<tr>
			<th>ID</th>
			<th>订单号</th>
			<th>商品名称</th>
			<th>金额</th>
			<th>状态</th>
		</tr>`
		for _, o := range list {
			statusText := "正常"
			if o.Status != 1 {
				statusText = "已取消"
			}
			html += fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
				<td>%d</td>
				<td>%s</td>
			</tr>`, o.ID, o.OrderNo, o.GoodsName, o.Amount, statusText)
		}
		html += `</table>`
		fmt.Fprint(w, html)
	}
}

// OrderDetailHandler 【存在横向越权漏洞】订单详情
func OrderDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html;charset=utf-8")

	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		return
	}

	// 获取url query参数 ?id=xx
	idStr := r.URL.Query().Get("id")
	oid, err := strconv.Atoi(idStr)
	if err != nil {
		pkg.ErrorHtml(w, "订单ID格式错误")
		return
	}

	// ⚠️漏洞点：只根据id查询订单，**没有判断这个订单属于哪个用户**
	order, err := model.GetOrderByID(oid)
	if err != nil {
		fmt.Fprintf(w, "订单不存在")
		return
	}

	//增加归属校验，修复横向越权
	if order.UserID != session.UserID {
		pkg.Forbidden(w, "无权访问该订单")
	}
	fmt.Fprintf(w, `
	<h2>订单详情页面</h2>
	<p>订单ID：%d</p>
	<p>归属用户：%d</p>
	<p>商品：%s</p>
	<p>金额：%f</p>
	`, order.ID, order.UserID, order.GoodsName, order.Amount)
}
func CancelOrderHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := pkg.MustGetSession(w, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if r.Method != http.MethodPost {
		fmt.Fprintf(w, "仅支持POST")
		return
	}
	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, "参数错误：%v", err)
		return
	}
	orderIDStr := r.Form.Get("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		fmt.Fprintf(w, "订单ID格式错误")
		return
	}

	err = model.CancelOrder(session.UserID, orderID)
	if err != nil {
		fmt.Fprintf(w, "取消失败：%v", err)
		return
	}
	http.Redirect(w, r, "/order/list", http.StatusFound)
}
