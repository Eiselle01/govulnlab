// Package reqctx 统一请求级上下文键，避免各层自己发明 context key 导致取不到值。
package reqctx

import (
	"context"

	"github.com/Eiselle01/govulnlab/internal/session"
)

type ctxKey int

const (
	keySession ctxKey = iota
	keyRequestID
	keyClientIP
	keyNonce
	keyUserAgent
)

func WithSession(ctx context.Context, s *session.Data) context.Context {
	return context.WithValue(ctx, keySession, s)
}

func Session(ctx context.Context) (*session.Data, bool) {
	s, ok := ctx.Value(keySession).(*session.Data)
	return s, ok && s != nil
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(keyRequestID).(string); ok {
		return v
	}
	return ""
}

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, keyClientIP, ip)
}

func ClientIP(ctx context.Context) string {
	if v, ok := ctx.Value(keyClientIP).(string); ok {
		return v
	}
	return ""
}

func WithNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, keyNonce, nonce)
}

// Nonce 供模板给内联脚本签名，配合 CSP 的 nonce-xxx 使用。
func Nonce(ctx context.Context) string {
	if v, ok := ctx.Value(keyNonce).(string); ok {
		return v
	}
	return ""
}

func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, keyUserAgent, ua)
}

func UserAgent(ctx context.Context) string {
	if v, ok := ctx.Value(keyUserAgent).(string); ok {
		return v
	}
	return ""
}
