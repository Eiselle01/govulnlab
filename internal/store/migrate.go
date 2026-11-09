package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Eiselle01/govulnlab/migrations"
)

// Migrate 按文件顺序执行迁移，并把已执行版本写入 schema_migrations。
//
// 为什么不用现成的迁移库：这个项目只需要「顺序执行 + 幂等 + 有记录」，
// 几十行代码就能做到，还能避免多引一个依赖。重复执行是安全的：
// 已应用版本会被跳过，新版本在一个事务里执行 SQL 并登记版本。
func (s *Store) Migrate(ctx context.Context, log *slog.Logger) error {
	const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(128) NOT NULL PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4`
	if _, err := s.DB.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	files, err := migrations.Files()
	if err != nil {
		return err
	}
	for _, name := range files {
		var applied string
		err := s.DB.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = ?`, name).Scan(&applied)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("查询迁移记录 %s 失败: %w", name, err)
		}

		content, err := migrations.Read(name)
		if err != nil {
			return err
		}

		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("开启迁移事务失败: %w", err)
		}
		// 逐条执行：驱动层不开 multiStatements（那会放大 SQL 注入的危害），
		// 由迁移器自己把脚本拆成独立语句。
		for _, stmt := range splitStatements(content) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES(?)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("登记迁移 %s 失败: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %s 失败: %w", name, err)
		}
		logMigrate(log, name)
	}
	return nil
}

// splitStatements 把 SQL 脚本按分号拆成单条语句。
// 只处理本项目用到的简单语法：跳过整行 -- 注释、忽略字符串字面量里的分号。
func splitStatements(src string) []string {
	var out []string
	var buf strings.Builder
	inQuote := false

	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inQuote && strings.HasPrefix(trimmed, "--") {
			continue
		}
		for _, ch := range line {
			switch {
			case ch == '\'':
				inQuote = !inQuote
				buf.WriteRune(ch)
			case ch == ';' && !inQuote:
				if stmt := strings.TrimSpace(buf.String()); stmt != "" {
					out = append(out, stmt)
				}
				buf.Reset()
			default:
				buf.WriteRune(ch)
			}
		}
		buf.WriteString("\n")
	}
	if stmt := strings.TrimSpace(buf.String()); stmt != "" {
		out = append(out, stmt)
	}
	return out
}
