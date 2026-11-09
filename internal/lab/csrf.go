package lab

import (
	"net/http"
	"strings"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// csrfPage 展示当前靶场昵称并提供「无 Token」的修改表单。
func (h *Handlers) csrfPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	nickname, err := h.Store.LabGetNickname(ctx, s.UserID)
	if err != nil {
		h.render(w, r, http.StatusInternalServerError, "lab_csrf.html", "CSRF", map[string]any{
			"Error": "读取昵称失败",
		})
		return
	}
	h.render(w, r, http.StatusOK, "lab_csrf.html", "CSRF", map[string]any{
		"Nickname": nickname,
		"OK":       r.URL.Query().Get("ok") == "1",
	})
}

// csrfSubmit 漏洞点：只依赖 Cookie 鉴权，不校验 CSRF Token、不校验 Origin、不校验 Referer。
//
// 对照安全实现：POST /account/password 由 CSRFGuard 中间件保护，
// 缺 Token 直接 403（见 middleware/csrf.go）。
func (h *Handlers) csrfSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "lab_csrf.html", "CSRF", map[string]any{"Error": "表单解析失败"})
		return
	}

	nickname := strings.TrimSpace(r.PostFormValue("nickname"))
	if nickname == "" {
		h.render(w, r, http.StatusBadRequest, "lab_csrf.html", "CSRF", map[string]any{"Error": "昵称不能为空"})
		return
	}
	if len([]rune(nickname)) > 64 {
		h.render(w, r, http.StatusBadRequest, "lab_csrf.html", "CSRF", map[string]any{"Error": "昵称过长"})
		return
	}

	if err := h.Store.LabSetNickname(ctx, s.UserID, nickname); err != nil {
		h.render(w, r, http.StatusInternalServerError, "lab_csrf.html", "CSRF", map[string]any{"Error": "更新失败"})
		return
	}
	h.Log.Info("CSRF 靶场：昵称被修改", "user_id", s.UserID, "nickname", nickname)
	http.Redirect(w, r, "/lab/csrf?ok=1", http.StatusFound)
}

// attackerPage 渲染攻击者页面（自动提交表单的 PoC）。
func (h *Handlers) attackerPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "lab_attacker.html", "攻击者页面", nil)
}
