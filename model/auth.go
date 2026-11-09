package model

import "github.com/Eiselle01/go-security-lab/pkg"

// QueryAllOrder 管理员查询全部订单，联表拿到用户名
func QueryAllOrder() ([]*Order, error) {
	rows, err := pkg.DB.Query(`
	SELECT o.id, o.oredr_no,o.user_id, o.goods_name, o.amount,  o.create_time,o.status
	FROM orders o,
	LEFT JOIN users u ON o.user_id = u.id
	ORDER BY o.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Order
	for rows.Next() {
		var item Order
		err := rows.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.GoodsName, &item.Amount, &item.CreateTime, &item.Status)
		if err != nil {
			return nil, err
		}
		list = append(list, &item)
	}
	return list, nil
}


func QueryAllAuditLog() ([]*AuditLog, error) {
	rows, err := pkg.DB.Query(`
	SELECT a.id, a.user_id,  a.action, a.create_time
	FROM audit a
	LEFT JOIN users u ON a.user_id = u.id
	ORDER BY a.create_time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*AuditLog
	for rows.Next() {
		var item AuditLog
		err := rows.Scan(&item.ID, &item.UserID,  &item.Action, &item.CreateTime)
		if err != nil {
			return nil, err
		}
		list = append(list, &item)
	}
	return list, nil
}