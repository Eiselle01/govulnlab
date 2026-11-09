package webui

import "testing"

// TestTemplatesParse 保证所有页面模板都能被解析。
// 模板写错（少个 end、引用不存在的字段）在编译期发现不了，必须靠这个测试兜住。
func TestTemplatesParse(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatalf("模板解析失败: %v", err)
	}
	if r.tpl == nil {
		t.Fatal("模板集合为空")
	}

	// 逐个确认关键页面模板存在。
	for _, name := range []string{
		"index.html", "login.html", "register.html", "profile.html", "password.html",
		"orders.html", "order_detail.html", "guestbook.html", "audit.html",
		"admin_orders.html", "admin_audit.html", "error.html",
		"lab_index.html", "lab_sqli.html", "lab_xss.html", "lab_csrf.html",
		"lab_idor.html", "lab_bruteforce.html", "lab_attacker.html",
	} {
		if r.tpl.Lookup(name) == nil {
			t.Errorf("模板 %s 不存在", name)
		}
	}
}
