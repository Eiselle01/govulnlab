package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Eiselle01/govulnlab/internal/reqctx"
	"github.com/Eiselle01/govulnlab/internal/session"
)

// index 首页：展示项目定位与漏洞清单。
func (h *Handlers) index(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "index.html", "首页", nil)
}

func (h *Handlers) loginPage(w http.ResponseWriter, r *http.Request) {
	notice := ""
	switch r.URL.Query().Get("changed") {
	case "1":
		notice = "密码已修改，请使用新密码重新登录。"
	}
	switch r.URL.Query().Get("registered") {
	case "1":
		notice = "注册成功，请登录。"
	}

	h.render(w, r, http.StatusOK, "login.html", "登录", map[string]any{
		"Next":   sanitizeNext(r.URL.Query().Get("next")),
		"Notice": notice,
		"Error":  nil,
	})
}

func (h *Handlers) registerPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "register.html", "注册", map[string]any{"Error": nil})
}

// loginSubmit 登录：校验凭证 → 轮换会话 → 写 Cookie → 审计 → 跳转。
func (h *Handlers) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "login.html", "登录", map[string]any{
			"Error": "表单解析失败", "Next": "/profile",
		})
		return
	}

	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	next := sanitizeNext(r.PostFormValue("next"))
	ctx := r.Context()

	user, err := h.Auth.Login(ctx, reqctx.ClientIP(ctx), username, password)
	if err != nil {
		// 失败也要留痕：这是排查撞库/暴力破解的关键数据。
		h.Audit.Record(ctx, 0, "login_failure", fmt.Sprintf("用户名=%s 原因=%s", username, errText(err)))
		// 用 error 自带的状态码：密码错误 → 401，触发锁定 → 429，
		// 而不是一律 401（锁定时客户端应该知道这是限流而不是凭证错误）。
		h.render(w, r, statusOf(err), "login.html", "登录", map[string]any{
			"Error": errText(err),
			"Next":  next,
		})
		return
	}

	// 会话固定防护：登录前如果已经有会话 ID，先销毁再签发新的。
	if old, ok := h.Sess.SID(r); ok {
		_ = h.Sess.Destroy(ctx, old)
	}
	sid, _, err := h.Sess.Create(ctx, session.Data{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
	if err != nil {
		h.Log.Error("创建会话失败", "err", err, "user_id", user.ID)
		h.render(w, r, http.StatusInternalServerError, "login.html", "登录", map[string]any{
			"Error": "会话创建失败，请重试", "Next": next,
		})
		return
	}
	h.Sess.SetCookie(w, sid)
	h.Audit.Record(ctx, user.ID, "login", "登录成功")

	http.Redirect(w, r, next, http.StatusFound)
}

func (h *Handlers) registerSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "register.html", "注册", map[string]any{"Error": "表单解析失败"})
		return
	}

	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	password2 := r.PostFormValue("password2")

	if password != password2 {
		h.render(w, r, http.StatusBadRequest, "register.html", "注册", map[string]any{"Error": "两次输入的密码不一致"})
		return
	}

	if _, err := h.Auth.Register(r.Context(), username, password); err != nil {
		h.render(w, r, http.StatusBadRequest, "register.html", "注册", map[string]any{"Error": errText(err)})
		return
	}

	http.Redirect(w, r, "/login?registered=1", http.StatusFound)
}

// logout 注销：服务端删会话 + 清 Cookie，两者缺一不可。
func (h *Handlers) logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	if sid, ok := h.Sess.SID(r); ok {
		if err := h.Sess.Destroy(ctx, sid); err != nil {
			h.Log.Warn("销毁会话失败", "err", err)
		}
	}
	h.Sess.ClearCookie(w)

	if s != nil {
		h.Audit.Record(ctx, s.UserID, "logout", "退出登录")
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handlers) profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	data := map[string]any{}
	if user, err := h.Store.GetUserByID(ctx, s.UserID); err == nil {
		data["User"] = user
	}
	h.render(w, r, http.StatusOK, "profile.html", "个人中心", data)
}

func (h *Handlers) passwordPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "password.html", "修改密码", nil)
}

// changePassword 修改密码：成功后旧会话立即失效（强制重新登录）。
func (h *Handlers) changePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, _ := reqctx.Session(ctx)

	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "password.html", "修改密码", map[string]any{"Error": "表单解析失败"})
		return
	}

	err := h.Auth.ChangePassword(ctx, s.UserID, r.PostFormValue("oldpwd"), r.PostFormValue("newpwd"))
	if err != nil {
		h.render(w, r, http.StatusBadRequest, "password.html", "修改密码", map[string]any{"Error": errText(err)})
		return
	}

	// 权限变更后作废旧会话，防止被盗的旧 Cookie 继续可用。
	if sid, ok := h.Sess.SID(r); ok {
		_ = h.Sess.Destroy(ctx, sid)
	}
	h.Sess.ClearCookie(w)
	h.Audit.Record(ctx, s.UserID, "change_password", "修改密码成功，旧会话已失效")

	http.Redirect(w, r, "/login?changed=1", http.StatusFound)
}
