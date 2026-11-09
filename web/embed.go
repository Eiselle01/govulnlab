// Package web 负责把页面模板与静态资源编译进二进制（//go:embed）。
// 好处：部署时只需要一个可执行文件 + 配置文件，不会出现“漏拷 static 目录导致样式全丢”。
package web

import "embed"

//go:embed templates/*.html
var Templates embed.FS

//go:embed static/*
var Static embed.FS
