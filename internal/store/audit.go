package store

import (
	"context"
	"database/sql"
	"fmt"
)

// InsertAudit 写一条审计日志（独立事务，用于登录、改密这类非订单操作）。
func (s *Store) InsertAudit(ctx context.Context, a AuditLog) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO audit_log(user_id, action, detail, ip, ua, request_id) VALUES(?, ?, ?, ?, ?, ?)`,
		a.UserID, a.Action, a.Detail, a.IP, a.UA, a.RequestID)
	if err != nil {
		return fmt.Errorf("写入审计日志失败: %w", err)
	}
	return nil
}

// insertAuditTx 供事务内复用（订单创建/取消必须与审计同生共死）。
func insertAuditTx(ctx context.Context, tx *sql.Tx, a AuditLog) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO audit_log(user_id, action, detail, ip, ua, request_id) VALUES(?, ?, ?, ?, ?, ?)`,
		a.UserID, a.Action, a.Detail, a.IP, a.UA, a.RequestID)
	if err != nil {
		return fmt.Errorf("事务内写入审计日志失败: %w", err)
	}
	return nil
}

func (s *Store) ListAuditByUser(ctx context.Context, userID int64, offset, size int) ([]*AuditLog, error) {
	return s.queryAudit(ctx,
		`SELECT id, user_id, action, detail, ip, ua, request_id, created_at
		 FROM audit_log WHERE user_id = ?
		 ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, userID, size, offset)
}

func (s *Store) CountAuditByUser(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log WHERE user_id = ?`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计个人审计日志失败: %w", err)
	}
	return n, nil
}

func (s *Store) ListAllAudit(ctx context.Context, offset, size int) ([]*AuditLog, error) {
	return s.queryAudit(ctx,
		`SELECT id, user_id, action, detail, ip, ua, request_id, created_at
		 FROM audit_log
		 ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, size, offset)
}

func (s *Store) CountAllAudit(ctx context.Context) (int64, error) {
	var n int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计审计日志失败: %w", err)
	}
	return n, nil
}

func (s *Store) queryAudit(ctx context.Context, query string, args ...any) ([]*AuditLog, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询审计日志失败: %w", err)
	}
	defer rows.Close()

	list := make([]*AuditLog, 0, 16)
	for rows.Next() {
		var a AuditLog
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.Detail, &a.IP, &a.UA, &a.RequestID, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取审计日志失败: %w", err)
		}
		list = append(list, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历审计结果集失败: %w", err)
	}
	return list, nil
}
