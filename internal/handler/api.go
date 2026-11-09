package handler

import (
	"net/http"
	"time"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
	"github.com/Eiselle01/govulnlab/internal/service"
	"github.com/Eiselle01/govulnlab/internal/store"
)

// orderDTO 是对外暴露的订单结构。
// 与数据库结构解耦：不把 UserID、remark 这类内部字段泄露给前端，也方便以后改表不改接口。
type orderDTO struct {
	ID         int64     `json:"id"`
	OrderNo    string    `json:"order_no"`
	GoodsName  string    `json:"goods_name"`
	AmountCent int64     `json:"amount_cent"`
	AmountYuan string    `json:"amount_yuan"`
	Status     int       `json:"status"`
	StatusText string    `json:"status_text"`
	CreatedAt  time.Time `json:"created_at"`
}

func toDTO(o *store.Order) orderDTO {
	return orderDTO{
		ID:         o.ID,
		OrderNo:    o.OrderNo,
		GoodsName:  o.GoodsName,
		AmountCent: o.AmountCent,
		AmountYuan: service.AmountToYuan(o.AmountCent),
		Status:     o.Status,
		StatusText: o.StatusText(),
		CreatedAt:  o.CreatedAt,
	}
}

type createOrderReq struct {
	GoodsName string `json:"goods_name"`
	Amount    string `json:"amount"`
}

func (h *Handlers) apiCreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	var req createOrderReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	o, err := h.Orders.Create(ctx, s.UserID, req.GoodsName, req.Amount)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.Audit.Record(ctx, s.UserID, "create_order_api", "通过 API 创建订单 "+o.OrderNo)
	httpx.Created(w, r, toDTO(o))
}

func (h *Handlers) apiListOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)
	p := httpx.ParsePage(r)

	result, err := h.Orders.ListMine(ctx, s.UserID, p.Offset, p.Size)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items := make([]orderDTO, 0, len(result.Items))
	for _, o := range result.Items {
		items = append(items, toDTO(o))
	}
	httpx.OK(w, r, map[string]any{
		"items":       items,
		"total":       result.Total,
		"page":        result.Page,
		"size":        result.Size,
		"total_pages": result.TotalPages(),
	})
}

func (h *Handlers) apiOrderDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	id, err := parseID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	o, err := h.Orders.Detail(ctx, s.UserID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, r, toDTO(o))
}

func (h *Handlers) apiCancelOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	id, err := parseID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.Orders.Cancel(ctx, s.UserID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.Audit.Record(ctx, s.UserID, "cancel_order_api", "通过 API 取消订单")
	httpx.OK(w, r, map[string]any{"id": id, "status": store.OrderStatusCanceled})
}
