package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const orderColumns = `o.id, o.order_no, o.user_id, o.goods_name, o.amount_cent, o.status, o.remark, o.created_at, o.updated_at`

func scanOrder(row interface {
	Scan(dest ...any) error
}) (*Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.OrderNo, &o.UserID, &o.GoodsName, &o.AmountCent,
		&o.Status, &o.Remark, &o.CreatedAt, &o.UpdatedAt, &o.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取订单失败: %w", err)
	}
	return &o, nil
}

// CreateOrder 在同一个事务里写下单和审计：两者要么都成功，要么都回滚。
// 审计写失败不能导致“订单已创建但没记录”，这是安全审计的基本要求。
func (s *Store) CreateOrder(ctx context.Context, o *Order, audit AuditLog) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO orders(order_no, user_id, goods_name, amount_cent, status, remark) VALUES(?, ?, ?, ?, ?, ?)`,
		o.OrderNo, o.UserID, o.GoodsName, o.AmountCent, OrderStatusPending, o.Remark)
	if err != nil {
		if isDuplicate(err) {
			return fmt.Errorf("%w: 订单号 %s", ErrDuplicate, o.OrderNo)
		}
		return fmt.Errorf("插入订单失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取订单 ID 失败: %w", err)
	}
	o.ID = id
	o.Status = OrderStatusPending

	if err := insertAuditTx(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交订单事务失败: %w", err)
	}
	return nil
}

// GetOrder 按订单 id + 归属用户查询，联表带出用户名。
// 归属条件写进 SQL（WHERE o.id = ? AND o.user_id = ?）：数据库替我们判断“这条订单是不是你的”，
// 而不是先查出来再在 Go 里 if 比较——少一次两步逻辑，也不把“订单存在但不属于你”的信息带回应用层。
func (s *Store) GetOrder(ctx context.Context, orderID, userID int64) (*Order, error) {
	q := `SELECT ` + orderColumns + `, u.username
	      FROM orders o JOIN users u ON o.user_id = u.id
	      WHERE o.id = ? AND o.user_id = ?`
	return scanOrder(s.DB.QueryRowContext(ctx, q, orderID, userID))
}

// ListOrdersByUser 分页查询某用户自己的订单，按创建时间倒序。
func (s *Store) ListOrdersByUser(ctx context.Context, userID int64, offset, size int) ([]*Order, error) {
	q := `SELECT ` + orderColumns + `, u.username
	      FROM orders o JOIN users u ON o.user_id = u.id
	      WHERE o.user_id = ?
	      ORDER BY o.created_at DESC, o.id DESC
	      LIMIT ? OFFSET ?`
	return s.queryOrders(ctx, q, userID, size, offset)
}

func (s *Store) CountOrdersByUser(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE user_id = ?`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计用户订单失败: %w", err)
	}
	return n, nil
}

// ListAllOrders 管理员视图：不分用户，全量分页。
func (s *Store) ListAllOrders(ctx context.Context, offset, size int) ([]*Order, error) {
	q := `SELECT ` + orderColumns + `, u.username
	      FROM orders o JOIN users u ON o.user_id = u.id
	      ORDER BY o.created_at DESC, o.id DESC
	      LIMIT ? OFFSET ?`
	return s.queryOrders(ctx, q, size, offset)
}

func (s *Store) CountAllOrders(ctx context.Context) (int64, error) {
	var n int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计全部订单失败: %w", err)
	}
	return n, nil
}

func (s *Store) queryOrders(ctx context.Context, query string, args ...any) ([]*Order, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询订单失败: %w", err)
	}
	defer rows.Close()

	list := make([]*Order, 0, 16)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历订单结果集失败: %w", err)
	}
	return list, nil
}

// CancelOrder 取消订单。归属校验放在 SQL 条件里，避免“先查再改”的竞态（TOCTOU）：
// 用 WHERE user_id = ? 让数据库替我们判断归属，改到 0 行就说明不是本人或状态已变。
func (s *Store) CancelOrder(ctx context.Context, userID, orderID int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var owner int64
	var status int
	err = tx.QueryRowContext(ctx, `SELECT user_id, status FROM orders WHERE id = ?`, orderID).Scan(&owner, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("查询订单归属失败: %w", err)
	}
	if owner != userID {
		return ErrNotOwner
	}
	if status != OrderStatusPending {
		return ErrInvalidState
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE orders SET status = ? WHERE id = ? AND status = ?`, OrderStatusCanceled, orderID, OrderStatusPending)
	if err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidState
	}

	if err := insertAuditTx(ctx, tx, AuditLog{
		UserID: userID,
		Action: "cancel_order",
		Detail: fmt.Sprintf("取消订单 id=%d", orderID),
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交取消订单事务失败: %w", err)
	}
	return nil
}
