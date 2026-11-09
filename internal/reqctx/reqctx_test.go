package reqctx

import (
	"context"
	"testing"

	"github.com/Eiselle01/govulnlab/internal/session"
)

func TestRequestIDAndClientIP(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-42")
	if got := RequestID(ctx); got != "req-42" {
		t.Errorf("RequestID = %q，期望 req-42", got)
	}

	ctx = WithClientIP(ctx, "203.0.113.7")
	if got := ClientIP(ctx); got != "203.0.113.7" {
		t.Errorf("ClientIP = %q", got)
	}
	if got := UserAgent(ctx); got != "" {
		t.Errorf("未设置 UA 时应为空，实际 %q", got)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	data := &session.Data{UserID: 7, Username: "demo", Role: "user", CSRFToken: "tok"}
	ctx := WithSession(context.Background(), data)

	got, ok := Session(ctx)
	if !ok {
		t.Fatal("应该能从 ctx 取出会话")
	}
	if got.UserID != 7 || got.Username != "demo" || got.Role != "user" {
		t.Errorf("会话内容不一致: %+v", got)
	}

	if _, ok := Session(context.Background()); ok {
		t.Error("空 ctx 不应有会话")
	}
}

func TestNonce(t *testing.T) {
	ctx := WithNonce(context.Background(), "n-1")
	if got := Nonce(ctx); got != "n-1" {
		t.Errorf("Nonce = %q", got)
	}
}
