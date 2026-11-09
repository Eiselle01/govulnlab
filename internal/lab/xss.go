package lab

import (
	htmltemplate "html/template"
	"net/http"
	"strings"
)

// xssPage 同时展示「漏洞版」与「安全版」的渲染结果，便于直观对照。
func (h *Handlers) xssPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	notes, err := h.Store.LabListNotes(ctx, 20)
	if err != nil {
		h.render(w, r, http.StatusInternalServerError, "lab_xss.html", "存储型 XSS", map[string]any{
			"Error": "读取靶场留言失败",
		})
		return
	}

	// 漏洞版：把字符串强转 template.HTML，等于告诉模板引擎“这是可信 HTML，别转义”。
	// #nosec G203 —— 这是靶场的教学演示，生产代码禁止这么写。
	unsafe := make([]htmltemplate.HTML, 0, len(notes))
	for _, n := range notes {
		unsafe = append(unsafe, htmltemplate.HTML(n.Content))
	}

	h.render(w, r, http.StatusOK, "lab_xss.html", "存储型 XSS", map[string]any{
		"Notes":  notes,
		"Unsafe": unsafe,
	})
}

func (h *Handlers) xssSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, http.StatusBadRequest, "lab_xss.html", "存储型 XSS", map[string]any{
			"Error": "表单解析失败",
		})
		return
	}

	content := strings.TrimSpace(r.PostFormValue("content"))
	if content == "" {
		h.render(w, r, http.StatusBadRequest, "lab_xss.html", "存储型 XSS", map[string]any{
			"Error": "内容不能为空",
		})
		return
	}

	// 原样入库：XSS 的根因在“输出时未转义”，而不是“入库时没过滤”。
	// 很多团队的错误做法是入库时做 HTML 过滤，结果把正常内容也改坏了。
	if _, err := h.Store.LabAddNote(r.Context(), "attacker", content); err != nil {
		h.render(w, r, http.StatusInternalServerError, "lab_xss.html", "存储型 XSS", map[string]any{
			"Error": "写入失败",
		})
		return
	}
	http.Redirect(w, r, "/lab/xss", http.StatusFound)
}
