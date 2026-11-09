package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestConstructorsMapStatusAndCode(t *testing.T) {
	cases := []struct {
		err        *Error
		wantStatus int
		wantCode   string
	}{
		{BadRequest("参数错误"), http.StatusBadRequest, "bad_request"},
		{Unauthorized("请先登录"), http.StatusUnauthorized, "unauthorized"},
		{Forbidden("无权访问"), http.StatusForbidden, "forbidden"},
		{NotFound("不存在"), http.StatusNotFound, "not_found"},
		{Conflict("已存在"), http.StatusConflict, "conflict"},
		{TooManyRequests("太频繁"), http.StatusTooManyRequests, "too_many_requests"},
		{Internal("内部错误", errors.New("boom")), http.StatusInternalServerError, "internal_error"},
	}
	for _, c := range cases {
		if c.err.Status != c.wantStatus {
			t.Errorf("%s 状态码应为 %d，实际 %d", c.wantCode, c.wantStatus, c.err.Status)
		}
		if c.err.Code != c.wantCode {
			t.Errorf("错误码应为 %s，实际 %s", c.wantCode, c.err.Code)
		}
		if c.err.Msg == "" {
			t.Errorf("%s 的对外文案不应为空", c.wantCode)
		}
	}
}

func TestAsUnwrapsWrappedError(t *testing.T) {
	base := NotFound("订单不存在")
	// 模拟 store 层用 %w 包装后向上返回的形态
	wrapped := fmt.Errorf("查询订单失败: %w", base)

	got, ok := As(wrapped)
	if !ok {
		t.Fatal("As 应该能从包装链里取出 *Error")
	}
	if got.Status != http.StatusNotFound || got.Msg != "订单不存在" {
		t.Errorf("取出的错误不正确: %+v", got)
	}

	if _, ok := As(errors.New("普通错误")); ok {
		t.Error("普通 error 不应被识别为业务错误")
	}
}

func TestUnwrapKeepsCauseChain(t *testing.T) {
	cause := errors.New("deadlock")
	e := Internal("写库失败", cause)

	if !errors.Is(e, cause) {
		t.Error("Unwrap 应能还原底层 cause（便于日志与 errors.Is 判断）")
	}
	if s := e.Error(); s == "" || s == "internal_error: 写库失败" {
		t.Errorf("Error() 应包含底层原因，实际 %q", s)
	}
}
