package service

import "testing"

func TestValidateUsername(t *testing.T) {
	ok := []string{"abc", "user_01", "A1234567890123456789012345678901"}
	bad := []string{"", "ab", "a b", "user-name", "user@example.com", "张三"}

	for _, u := range ok {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("用户名 %q 应通过校验，实际报错: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("用户名 %q 应被拒绝", u)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		pwd string
		min int
		ok  bool
	}{
		{"abc1234567", 10, true},
		{"ABC1234567", 10, true},
		{"a1", 10, false},         // 太短
		{"abcdefghij", 10, false}, // 无数字
		{"1234567890", 10, false}, // 无字母
		{"aaaaaaaaaa", 10, false}, // 单字符重复
		{"abc1234567890123456789012345678901234567890123456789012345678901234567890", 10, false}, // 超过 72 字节
	}
	for _, c := range cases {
		err := ValidatePassword(c.pwd, c.min)
		if c.ok && err != nil {
			t.Errorf("密码 %q 应通过，实际报错 %v", c.pwd, err)
		}
		if !c.ok && err == nil {
			t.Errorf("密码 %q 应被拒绝", c.pwd)
		}
	}
}
