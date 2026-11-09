// Package middleware 提供可组合的 HTTP 中间件。
//
// 组合顺序即执行顺序：Chain(h, A, B) 表示 A 先于 B 执行，A 负责最外层（日志、恢复、限流），
// B 负责更靠近业务（鉴权、CSRF）。
package middleware

import (
	"net/http"
)

// Middleware 是标准 net/http 中间件签名。
type Middleware func(http.Handler) http.Handler

// Chain 把多个中间件包成一个 Handler。
// 从后往前包，保证调用顺序与书写顺序一致（先写的在最外层）。
func Chain(h http.Handler, ms ...Middleware) http.Handler {
	for i := len(ms) - 1; i >= 0; i-- {
		h = ms[i](h)
	}
	return h
}

// Mode 决定鉴权失败时的响应形态：
//   - ModePage：浏览器页面请求，302 跳登录页；
//   - ModeAPI：JSON 接口，401 + 统一错误体。
type Mode int

const (
	ModePage Mode = iota
	ModeAPI
)
