package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CreateUser 插入普通用户。用户名唯一冲突会被翻译成 ErrDuplicate。
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (int64, error) {
	return s.createUser(ctx, username, passwordHash, "user")
}

// CreateAdmin 供 seed 使用，创建一个管理员账号。
func (s *Store) CreateAdmin(ctx context.Context, username, passwordHash string) (int64, error) {
	return s.createUser(ctx, username, passwordHash, "admin")
}

func (s *Store) createUser(ctx context.Context, username, passwordHash, role string) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, role) VALUES(?, ?, ?)`,
		username, passwordHash, role)
	if err != nil {
		if isDuplicate(err) {
			return 0, fmt.Errorf("%w: 用户名 %s", ErrDuplicate, username)
		}
		return 0, fmt.Errorf("插入用户失败: %w", err)
	}
	return res.LastInsertId()
}

// GetUserByUsername 按用户名查询。用户不存在返回 ErrNotFound。
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, status, created_at FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, status, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取用户失败: %w", err)
	}
	return &u, nil
}

// UpdatePassword 更新密码哈希。
func (s *Store) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}
	return nil
}

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计用户失败: %w", err)
	}
	return n, nil
}
