// Package logger 构建结构化日志器（log/slog）。
package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Eiselle01/govulnlab/internal/config"
)

// New 按配置创建 slog.Logger。生产环境建议 format=json，便于日志采集侧解析。
func New(cfg config.Log) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(cfg.Level))); err != nil {
		return nil, fmt.Errorf("非法日志级别 %q: %w", cfg.Level, err)
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text", "":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		return nil, fmt.Errorf("非法日志格式 %q（可选 text/json）", cfg.Format)
	}
	return slog.New(handler), nil
}
