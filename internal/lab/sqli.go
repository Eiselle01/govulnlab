package lab

import (
	"errors"
	"net/http"

	"github.com/Eiselle01/govulnlab/internal/store"
)

// sqliPage 展示注入表单。
func (h *Handlers) sqliPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "lab_sqli.html", "SQL 注入", map[string]any{
		"Username": "",
		"Password": "",
	})
}

// sqliSubmit 演示 SQL 注入。
//
// 这里就是把用户输入直接拼进 SQL —— 与 store 层参数化查询形成鲜明对照。
// 教学点：只要输入能改变 SQL 结构，就有注入；不是“有没有过滤关键字”的问题。
func (h *Handlers) sqliSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "lab_sqli.html", "SQL 注入", map[string]any{
			"Error": "表单解析失败",
		})
		return
	}

	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	// ⚠️ 故意拼接（仅靶场）。生产代码必须用 ? 占位符。
	rawSQL := "SELECT id, username, role FROM lab_users WHERE username = '" +
		username + "' AND password = '" + password + "' LIMIT 1"

	data := map[string]any{
		"SQL":      rawSQL,
		"Username": username,
		"Password": password,
	}

	user, err := h.Store.LabQueryUserRaw(r.Context(), rawSQL)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			data["Error"] = "查询无结果：用户名或密码不匹配（也可能被 LIMIT 截断了）"
		} else {
			// 不把数据库原始报错抛给页面，避免泄露表结构；真实环境更不该回显 SQL 错误。
			data["Error"] = "查询无结果或 SQL 语法错误"
		}
		h.render(w, r, http.StatusOK, "lab_sqli.html", "SQL 注入", data)
		return
	}
	data["Result"] = user
	h.render(w, r, http.StatusOK, "lab_sqli.html", "SQL 注入", data)
}
