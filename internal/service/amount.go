package service

import (
	"fmt"
	"strings"
)

// PageResult 是分页查询的统一返回结构（用泛型避免为每种实体写一遍）。
type PageResult[T any] struct {
	Items []T
	Total int64
	Page  int
	Size  int
}

// HasPrev / HasNext 供模板直接判断是否渲染上一页/下一页。
func (p PageResult[T]) HasPrev() bool { return p.Page > 1 }
func (p PageResult[T]) HasNext() bool { return int64(p.Page*p.Size) < p.Total }
func (p PageResult[T]) TotalPages() int {
	if p.Size <= 0 {
		return 1
	}
	pages := int((p.Total + int64(p.Size) - 1) / int64(p.Size))
	if pages < 1 {
		pages = 1
	}
	return pages
}

// AmountToYuan 把「分」格式化成两位小数的元，模板展示统一用它，避免浮点误差。
func AmountToYuan(cent int64) string {
	sign := ""
	if cent < 0 {
		sign = "-"
		cent = -cent
	}
	return fmt.Sprintf("%s%d.%02d", sign, cent/100, cent%100)
}

// ParseAmount 把用户输入的金额字符串转成「分」。
//
// 为什么不用 strconv.ParseFloat：0.1+0.2=0.30000000000000004，
// 一旦用 float 参与金额计算和入库，迟早出现对不上账的问题。
// 这里用正则把整数位、小数位分开，纯整数运算得到分。
func ParseAmount(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("金额不能为空")
	}
	// 最多 8 位整数 + 2 位小数，全角字符/科学计数法一律拒绝。
	if !isDecimal(s) {
		return 0, fmt.Errorf("金额格式不正确，最多两位小数")
	}

	parts := strings.SplitN(s, ".", 2)
	yuan, err := parseUint(parts[0])
	if err != nil {
		return 0, fmt.Errorf("金额格式不正确")
	}
	var frac int64
	if len(parts) == 2 && parts[1] != "" {
		digits := parts[1]
		if len(digits) == 1 {
			digits += "0"
		}
		frac, err = parseUint(digits)
		if err != nil {
			return 0, fmt.Errorf("金额格式不正确")
		}
	}

	cent := yuan*100 + frac
	if cent <= 0 {
		return 0, fmt.Errorf("金额必须大于 0")
	}
	if cent > maxAmountCent {
		return 0, fmt.Errorf("金额超出单笔上限（%s 元）", AmountToYuan(maxAmountCent))
	}
	return cent, nil
}

const maxAmountCent = int64(100_000_000) // 100 万元

// isDecimal 手写扫描而不是 regexp：逻辑更直观，且不依赖正则回溯。
func isDecimal(s string) bool {
	if len(s) > 12 { // 8 位整数 + 1 位小数点 + 2 位小数
		return false
	}
	dots, digits := 0, 0
	fracDigits := 0
	seenDot := false
	for _, c := range s {
		switch {
		case c == '.':
			dots++
			if dots > 1 || digits == 0 {
				return false
			}
			seenDot = true
		case c >= '0' && c <= '9':
			digits++
			if seenDot {
				fracDigits++
				if fracDigits > 2 {
					return false
				}
			} else if digits > 8 {
				return false
			}
		default:
			return false
		}
	}
	return digits > 0
}

func parseUint(s string) (int64, error) {
	var n int64
	if s == "" {
		return 0, fmt.Errorf("空数字")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("非法数字")
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}
