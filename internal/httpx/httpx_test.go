package httpx

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Eiselle01/govulnlab/internal/apperr"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

func TestOKEnvelope(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/orders", nil)
	r = r.WithContext(reqctx.WithRequestID(r.Context(), "req-1"))
	rec := httptest.NewRecorder()

	OK(rec, r, map[string]any{"hello": "world"})

	if rec.Code != 200 {
		t.Fatalf("状态码应为 200，实际 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 错误: %q", ct)
	}

	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if env.Code != "ok" || env.RequestID != "req-1" {
		t.Errorf("信封字段错误: %+v", env)
	}
}

// 关键安全断言：内部错误绝不允许把底层 err 文本漏给客户端。
func TestErrorHidesInternalDetail(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/orders", nil)
	rec := httptest.NewRecorder()

	Error(rec, r, errors.New("dial tcp 10.0.0.5:3306: connect: connection refused"))

	if rec.Code != 500 {
		t.Fatalf("非业务错误应返回 500，实际 %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "3306") || strings.Contains(rec.Body.String(), "dial tcp") {
		t.Fatalf("响应泄露了内部错误细节: %s", rec.Body.String())
	}

	// 业务错误则要按 apperr 的状态码与文案返回
	rec = httptest.NewRecorder()
	Error(rec, r, apperr.NotFound("订单不存在"))
	if rec.Code != 404 {
		t.Errorf("业务错误状态码应为 404，实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "订单不存在") {
		t.Errorf("业务错误应返回对外文案")
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var p payload
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"a","injected":"x"}`))
	if err := DecodeJSON(req, &p); err == nil {
		t.Fatal("含未知字段的 JSON 应被拒绝（防止参数走私）")
	}

	req = httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"a"}`))
	if err := DecodeJSON(req, &p); err != nil {
		t.Fatalf("合法 JSON 解析失败: %v", err)
	}
	if p.Name != "a" {
		t.Errorf("字段未正确绑定: %+v", p)
	}
}

func TestParsePage(t *testing.T) {
	cases := []struct {
		query      string
		wantPage   int
		wantSize   int
		wantOffset int
	}{
		{"", 1, 10, 0},
		{"page=3&size=10", 3, 10, 20},
		{"page=0&size=0", 1, 10, 0},     // 非法值回落默认
		{"page=-5&size=-1", 1, 10, 0},   // 负数回落默认
		{"page=1&size=9999", 1, 50, 0},  // size 被夹到上限
		{"page=abc&size=xyz", 1, 10, 0}, // 非数字回落默认
		{"page=10001", 1, 10, 0},        // 超过页数上限
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/api/orders?"+c.query, nil)
		p := ParsePage(r)
		if p.Page != c.wantPage || p.Size != c.wantSize || p.Offset != c.wantOffset {
			t.Errorf("ParsePage(%q) = (page=%d,size=%d,offset=%d)，期望 (%d,%d,%d)",
				c.query, p.Page, p.Size, p.Offset, c.wantPage, c.wantSize, c.wantOffset)
		}
		if p.MaxSize != 50 {
			t.Errorf("MaxSize 应固定为 50，实际 %d", p.MaxSize)
		}
	}
}
