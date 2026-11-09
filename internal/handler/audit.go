package handler

import (
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

func (h *Handlers) guestbookPage(w http.ResponseWriter, r *http.Request) {
	h.renderGuestbook(w, r, http.StatusOK, "")
}

func (h *Handlers) renderGuestbook(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	ctx := r.Context()
	p := httpx.ParsePage(r)

	result, err := h.Guestbook.List(ctx, p.Offset, p.Size)
	if err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}
	data := map[string]any{"Result": result}
	if errMsg != "" {
		data["Error"] = errMsg
	}
	h.render(w, r, status, "guestbook.html", "留言板", data)
}

func (h *Handlers) guestbookSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	if err := r.ParseForm(); err != nil {
		h.renderGuestbook(w, r, http.StatusBadRequest, "表单解析失败")
		return
	}
	if err := h.Guestbook.Add(ctx, s.UserID, r.PostFormValue("content")); err != nil {
		h.renderGuestbook(w, r, statusOf(err), errText(err))
		return
	}
	h.Audit.Record(ctx, s.UserID, "add_note", "发布留言")
	http.Redirect(w, r, "/guestbook", http.StatusFound)
}

func (h *Handlers) auditPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)
	p := httpx.ParsePage(r)

	result, err := h.Audit.ListMine(ctx, s.UserID, p.Offset, p.Size)
	if err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}
	h.render(w, r, http.StatusOK, "audit.html", "操作审计", map[string]any{"Result": result})
}

func (h *Handlers) adminOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)
	p := httpx.ParsePage(r)

	result, err := h.Orders.ListAll(ctx, p.Offset, p.Size)
	if err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}
	h.Audit.Record(ctx, s.UserID, "admin_view_orders", "查看全量订单")
	h.render(w, r, http.StatusOK, "admin_orders.html", "全部订单", map[string]any{"Result": result})
}

func (h *Handlers) adminAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)
	p := httpx.ParsePage(r)

	result, err := h.Audit.ListAll(ctx, p.Offset, p.Size)
	if err != nil {
		h.Tpl.Error(w, r, statusOf(err), errText(err))
		return
	}
	h.Audit.Record(ctx, s.UserID, "admin_view_audit", "查看全局审计日志")
	h.render(w, r, http.StatusOK, "admin_audit.html", "全局审计", map[string]any{"Result": result})
}
