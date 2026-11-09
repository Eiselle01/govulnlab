package middleware

import "testing"

func TestTokensEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "ab", false},
		{"", "", false},
		{"abc", "", false},
	}
	for _, c := range cases {
		if got := tokensEqual(c.a, c.b); got != c.want {
			t.Errorf("tokensEqual(%q,%q)=%v 期望 %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSameOrigin(t *testing.T) {
	cases := []struct {
		origin, host string
		want         bool
	}{
		{"http://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"https://example.com", "example.com", true},
		{"http://evil.com", "127.0.0.1:8080", false},
		{"not-a-url", "127.0.0.1:8080", false},
		{"", "127.0.0.1:8080", false},
	}
	for _, c := range cases {
		if got := sameOrigin(c.origin, c.host); got != c.want {
			t.Errorf("sameOrigin(%q,%q)=%v 期望 %v", c.origin, c.host, got, c.want)
		}
	}
}

func TestIsFormContentType(t *testing.T) {
	if !isFormContentType("application/x-www-form-urlencoded; charset=utf-8") {
		t.Error("表单类型应被识别")
	}
	if !isFormContentType("multipart/form-data; boundary=x") {
		t.Error("multipart 应被识别")
	}
	if isFormContentType("application/json") {
		t.Error("JSON 不应被当成表单")
	}
}
