package handler

import (
	"fmt"
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

func (h *Handlers) ordersPage(w http.ResponseWriter, r *http.Request) {
	h.renderOrders(w, r, http.StatusOK, "")
}

// renderOrders 渲染订单列表页；错误直接回填到同一页面，用户不用跳来跳去。
func (h *Handlers) renderOrders(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)
	p := httpx.ParsePage(r)

	result, err := h.Orders.ListMine(ctx, s.UserID, p.Offset, p.Size)
	if err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}

	data := map[string]any{"Result": result}
	if errMsg != "" {
		data["Error"] = errMsg
	}
	h.render(w, r, status, "orders.html", "我的订单", data)
}

func (h *Handlers) createOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	if err := r.ParseForm(); err != nil {
		h.renderOrders(w, r, http.StatusBadRequest, "表单解析失败")
		return
	}

	o, err := h.Orders.Create(ctx, s.UserID, r.PostFormValue("goodsname"), r.PostFormValue("amount"))
	if err != nil {
		h.renderOrders(w, r, statusOf(err), errText(err))
		return
	}

	h.Audit.Record(ctx, s.UserID, "create_order_ui", fmt.Sprintf("订单 %s 创建成功", o.OrderNo))
	http.Redirect(w, r, "/orders", http.StatusFound)
}

func (h *Handlers) orderDetailPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	id, err := parseID(r, "id")
	if err != nil {
		h.Tpl.Error(w, r, http.StatusBadRequest, errText(err))
		return
	}
	o, err := h.Orders.Detail(ctx, s.UserID, id)
	if err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}
	h.render(w, r, http.StatusOK, "order_detail.html", "订单详情", map[string]any{"Order": o})
}

func (h *Handlers) cancelOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	id, err := parseID(r, "id")
	if err != nil {
		h.Tpl.Error(w, r, http.StatusBadRequest, errText(err))
		return
	}
	if err := h.Orders.Cancel(ctx, s.UserID, id); err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}
	h.Audit.Record(ctx, s.UserID, "cancel_order_ui", fmt.Sprintf("取消订单 id=%d", id))
	http.Redirect(w, r, "/orders", http.StatusFound)
}
