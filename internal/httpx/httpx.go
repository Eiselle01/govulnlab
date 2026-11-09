// Package httpx 提供统一的 HTTP 响应格式与参数解析工具。
//
// 统一响应体：{"code":"ok","msg":"...","request_id":"...","data":...}
// code 是稳定业务码（前端可按码分支），request_id 用于把用户反馈和日志对上。
package httpx

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Eiselle01/govulnlab/internal/apperr"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// Envelope 是所有 JSON 接口的固定外层结构。
type Envelope struct {
	Code      string `json:"code"`
	Msg       string `json:"msg"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
}

func JSON(w http.ResponseWriter, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

// OK 返回成功响应（HTTP 200）。
func OK(w http.ResponseWriter, r *http.Request, data any) {
	JSON(w, http.StatusOK, Envelope{
		Code:      "ok",
		Msg:       "success",
		RequestID: reqctx.RequestID(r.Context()),
		Data:      data,
	})
}

// Created 返回 201（新建资源语义）。
func Created(w http.ResponseWriter, r *http.Request, data any) {
	JSON(w, http.StatusCreated, Envelope{
		Code:      "ok",
		Msg:       "created",
		RequestID: reqctx.RequestID(r.Context()),
		Data:      data,
	})
}

// Error 把任意 error 翻译成响应。
// 非业务错误一律 500 + 通用文案，绝不把 err.Error() 漏给客户端（防止泄露表名/路径）。
func Error(w http.ResponseWriter, r *http.Request, err error) {
	ae, ok := apperr.As(err)
	if !ok {
		JSON(w, http.StatusInternalServerError, Envelope{
			Code:      "internal_error",
			Msg:       "服务器内部错误，请稍后重试",
			RequestID: reqctx.RequestID(r.Context()),
		})
		return
	}
	JSON(w, ae.Status, Envelope{
		Code:      ae.Code,
		Msg:       ae.Msg,
		RequestID: reqctx.RequestID(r.Context()),
	})
}

// DecodeJSON 解析 JSON 请求体：限制为单对象、拒绝未知字段（防参数污染）。
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.BadRequest("请求体不是合法的 JSON")
	}
	return nil
}

// Page 分页参数，做了上下限收敛，避免 ?size=100000 打爆数据库。
type Page struct {
	Page    int
	Size    int
	MaxSize int
	Offset  int
}

func ParsePage(r *http.Request) Page {
	p := Page{Page: 1, Size: 10, MaxSize: 50}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 10000 {
			p.Page = n
		}
	}
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > p.MaxSize {
				n = p.MaxSize
			}
			p.Size = n
		}
	}
	p.Offset = (p.Page - 1) * p.Size
	return p
}
