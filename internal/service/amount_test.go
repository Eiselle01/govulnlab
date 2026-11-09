package service

import "testing"

// TestParseAmount 覆盖金额解析的边界，这类“钱”的逻辑必须表驱动测试。
func TestParseAmount(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"12", 1200, false},
		{"12.3", 1230, false},
		{"12.34", 1234, false},
		{"0.01", 1, false},
		{"1000000", 100000000, false},
		{"7.00", 700, false},
		{"  9.9  ", 990, false},

		{"0", 0, true},
		{"0.00", 0, true},
		{"", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"1.234", 0, true},
		{"1e3", 0, true},
		{"１２", 0, true},        // 全角数字必须拒绝
		{"100000000", 0, true}, // 超出单笔上限
		{"123456789.1", 0, true},
	}
	for _, c := range cases {
		got, err := ParseAmount(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseAmount(%q) 期望报错，实际得到 %d", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAmount(%q) 意外报错: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseAmount(%q) = %d, 期望 %d", c.in, got, c.want)
		}
	}
}

func TestAmountToYuan(t *testing.T) {
	cases := map[int64]string{0: "0.00", 5: "0.05", 1200: "12.00", 1234: "12.34", 100000000: "1000000.00"}
	for in, want := range cases {
		if got := AmountToYuan(in); got != want {
			t.Errorf("AmountToYuan(%d) = %q, 期望 %q", in, got, want)
		}
	}
}
