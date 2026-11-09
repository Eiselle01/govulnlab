package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// 本文件只服务于「故意留洞」的靶场模块，与业务表隔离。
// 唯一一处允许把 SQL 字符串直接交给数据库执行：LabQueryUserRaw。
// 它存在的意义是让学习者亲眼看到注入发生了什么，因此：
//   - 只由 lab 路由调用，lab 路由默认只允许本机访问；
//   - 生产部署必须 lab.enabled=false；
//   - 业务代码禁止引用本文件的任何函数（代码评审重点检查项）。

// LabQueryUserRaw 执行一段「由上层拼接」的 SQL，用于 SQL 注入演示。
func (s *Store) LabQueryUserRaw(ctx context.Context, rawSQL string) (*LabUser, error) {
	var u LabUser
	err := s.DB.QueryRowContext(ctx, rawSQL).Scan(&u.ID, &u.Username, &u.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("靶场查询失败: %w", err)
	}
	return &u, nil
}

// LabLogin 用于暴力破解演示：参数化查询（不做注入），但调用方故意不加锁定。
func (s *Store) LabLogin(ctx context.Context, username, password string) (*LabUser, error) {
	var u LabUser
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, username, role FROM lab_users WHERE username = ? AND password = ?`, username, password).
		Scan(&u.ID, &u.Username, &u.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("靶场登录查询失败: %w", err)
	}
	u.Password = password
	return &u, nil
}

// LabUpsertUser 供 seed 使用（幂等）。
func (s *Store) LabUpsertUser(ctx context.Context, username, password, role string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO lab_users(username, password, role) VALUES(?, ?, ?)
		 ON DUPLICATE KEY UPDATE password = VALUES(password), role = VALUES(role)`,
		username, password, role)
	if err != nil {
		return fmt.Errorf("初始化靶场用户失败: %w", err)
	}
	return nil
}

func (s *Store) LabAddNote(ctx context.Context, author, content string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO lab_notes(author, content) VALUES(?, ?)`, author, content)
	if err != nil {
		return 0, fmt.Errorf("写入靶场留言失败: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) LabListNotes(ctx context.Context, limit int) ([]*LabNote, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, author, content, created_at FROM lab_notes ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询靶场留言失败: %w", err)
	}
	defer rows.Close()

	list := make([]*LabNote, 0, 16)
	for rows.Next() {
		var n LabNote
		if err := rows.Scan(&n.ID, &n.Author, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取靶场留言失败: %w", err)
		}
		list = append(list, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历靶场留言失败: %w", err)
	}
	return list, nil
}

// LabGetOrder 故意「不带归属条件」，越权演示由 handler 直接展示结果。
func (s *Store) LabGetOrder(ctx context.Context, id int64) (*LabOrder, error) {
	var o LabOrder
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, user_id, order_no, goods_name, amount_cent, status, created_at
		 FROM lab_orders WHERE id = ?`, id).
		Scan(&o.ID, &o.UserID, &o.OrderNo, &o.GoodsName, &o.AmountCent, &o.Status, &o.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("查询靶场订单失败: %w", err)
	}
	return &o, nil
}

func (s *Store) LabUpsertOrder(ctx context.Context, o LabOrder) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO lab_orders(user_id, order_no, goods_name, amount_cent, status) VALUES(?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE goods_name = VALUES(goods_name)`,
		o.UserID, o.OrderNo, o.GoodsName, o.AmountCent, o.Status)
	if err != nil {
		return fmt.Errorf("初始化靶场订单失败: %w", err)
	}
	return nil
}

// LabGetNickname 读取靶场昵称；没有记录时返回空串。
func (s *Store) LabGetNickname(ctx context.Context, userID int64) (string, error) {
	var nickname string
	err := s.DB.QueryRowContext(ctx, `SELECT nickname FROM lab_profiles WHERE user_id = ?`, userID).Scan(&nickname)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取靶场昵称失败: %w", err)
	}
	return nickname, nil
}

func (s *Store) LabSetNickname(ctx context.Context, userID int64, nickname string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO lab_profiles(user_id, nickname) VALUES(?, ?)
		 ON DUPLICATE KEY UPDATE nickname = VALUES(nickname)`, userID, nickname)
	if err != nil {
		return fmt.Errorf("更新靶场昵称失败: %w", err)
	}
	return nil
}
