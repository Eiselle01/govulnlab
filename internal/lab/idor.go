package lab

import (
	"net/http"
	"strconv"
)

// idorPage 漏洞点：只按 id 查询订单，不判断订单是否属于当前登录用户。
//
// 对照安全实现：service.OrderService.Detail 会校验归属，不匹配返回 404。
func (h *Handlers) idorPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idRaw := r.URL.Query().Get("id")

	data := map[string]any{"OrderID": idRaw}
	if idRaw != "" {
		id, err := strconv.ParseInt(idRaw, 10, 64)
		if err != nil || id <= 0 {
			data["Error"] = "id 必须是正整数"
			h.render(w, r, http.StatusBadRequest, "lab_idor.html", "水平越权", data)
			return
		}
		order, err := h.Store.LabGetOrder(ctx, id)
		if err != nil {
			data["Error"] = "订单不存在"
		} else {
			data["Order"] = order
		}
	}
	h.render(w, r, http.StatusOK, "lab_idor.html", "水平越权", data)
}
