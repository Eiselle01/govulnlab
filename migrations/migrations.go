// Package migrations 把 SQL 迁移脚本编译进二进制。
//
// 用 //go:embed 而不是运行时读文件，解决两个真实痛点：
//  1. 二进制可以放到任意目录运行（容器里工作目录常常不是代码根目录）；
//  2. 部署时不会出现「忘了拷 migrations 目录」导致的启动失败。
package migrations

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed *.sql
var fs embed.FS

// Files 返回按文件名升序排列的迁移脚本（0001、0002 ... 顺序即执行顺序）。
func Files() ([]string, error) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("读取内嵌迁移目录失败: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func Read(name string) (string, error) {
	b, err := fs.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("读取迁移脚本 %s 失败: %w", name, err)
	}
	return string(b), nil
}
