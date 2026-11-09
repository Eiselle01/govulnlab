package store

import (
	"context"
	"fmt"
)

// AddNote 新增留言。内容原样入库（不做“入库即转义”），
// 因为 XSS 的正确防线在「输出侧」——用 html/template 自动转义。
func (s *Store) AddNote(ctx context.Context, userID int64, content string) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO guestbook(user_id, content) VALUES(?, ?)`, userID, content)
	if err != nil {
		return 0, fmt.Errorf("插入留言失败: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) ListNotes(ctx context.Context, offset, size int) ([]*Note, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT g.id, g.user_id, u.username, g.content, g.created_at
		 FROM guestbook g JOIN users u ON g.user_id = u.id
		 ORDER BY g.created_at DESC, g.id DESC
		 LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, fmt.Errorf("查询留言失败: %w", err)
	}
	defer rows.Close()

	list := make([]*Note, 0, 16)
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.UserID, &n.Username, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取留言失败: %w", err)
		}
		list = append(list, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历留言结果集失败: %w", err)
	}
	return list, nil
}

func (s *Store) CountNotes(ctx context.Context) (int64, error) {
	var n int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM guestbook`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计留言失败: %w", err)
	}
	return n, nil
}
