package lab

import (
	"net/http"
	"strings"
)

// brutePage 展示尝试次数与登录表单。
func (h *Handlers) brutePage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "lab_bruteforce.html", "暴力破解", map[string]any{
		"Attempts": h.bruteAttempts.Load(),
	})
}

// bruteSubmit 漏洞点：失败不计数、不锁定、无延迟，攻击者可以无限试。
// 对照安全实现：service.AuthService.Login（Redis 计数 + 锁定 + 统一话术）。
func (h *Handlers) bruteSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "lab_bruteforce.html", "暴力破解", map[string]any{
			"Error": "表单解析失败", "Attempts": h.bruteAttempts.Load(),
		})
		return
	}

	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	h.bruteAttempts.Add(1)

	data := map[string]any{
		"Username": username,
		"Attempts": h.bruteAttempts.Load(),
	}

	user, err := h.Store.LabLogin(r.Context(), username, password)
	if err != nil {
		data["Error"] = "登录失败：用户名或口令不匹配"
		h.render(w, r, http.StatusOK, "lab_bruteforce.html", "暴力破解", data)
		return
	}
	data["User"] = user
	h.render(w, r, http.StatusOK, "lab_bruteforce.html", "暴力破解", data)
}
