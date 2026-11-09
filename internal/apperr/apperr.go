// Package apperr 定义带“业务码 + HTTP 状态码”的错误类型。
//
// 分层原则：service 层只返回 *apperr.Error，handler 层统一翻译成响应，
// 这样业务码不用在每一层重复判断，也避免把内部错误细节直接漏给前端。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error 业务错误。Code 是给前端/日志看的稳定标识，Msg 是可直接展示的提示。
type Error struct {
	Code   string
	Status int
	Msg    string
	Err    error // 包装的底层错误，只进日志，不进响应
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *Error) Unwrap() error { return e.Err }

// New 构造错误；底层错误可以为 nil。
func New(code string, status int, msg string, err error) *Error {
	return &Error{Code: code, Status: status, Msg: msg, Err: err}
}

func BadRequest(msg string) *Error {
	return New("bad_request", http.StatusBadRequest, msg, nil)
}

func Unauthorized(msg string) *Error {
	return New("unauthorized", http.StatusUnauthorized, msg, nil)
}

func Forbidden(msg string) *Error {
	return New("forbidden", http.StatusForbidden, msg, nil)
}

func NotFound(msg string) *Error {
	return New("not_found", http.StatusNotFound, msg, nil)
}

func Conflict(msg string) *Error {
	return New("conflict", http.StatusConflict, msg, nil)
}

func TooManyRequests(msg string) *Error {
	return New("too_many_requests", http.StatusTooManyRequests, msg, nil)
}

func Internal(msg string, err error) *Error {
	return New("internal_error", http.StatusInternalServerError, msg, err)
}

// As 是 errors.As 的薄封装，少写点样板代码。
func As(err error) (*Error, bool) {
	var ae *Error
	ok := errors.As(err, &ae)
	return ae, ok
}
