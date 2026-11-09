package model

import (
	// "database/sql"
	"time"
)

// AuditLog 审计日志表

type AuditLog struct {
	ID         int       `gorm:"column:id"`
	UserID     int       `gorm:"column:user_id"`
	Action     string    `gorm:"column:action"`
	Detail     string    `gorm:"column:detail"`
	CreateTime time.Time `gorm:"column:create_time"`
	IpAddr     string    `gorm:"column:ip_addr"` // IP
}

// CreateAuditLogTable 创建审计日志表，main启动调用
// func CreateAuditLogTable(db *sql.DB) error {
// 	sqlStr := `
// CREATE TABLE IF NOT EXISTS audit_log (
// 	id INTEGER PRIMARY KEY AUTO_INCREMENT,
// 	username TEXT NOT NULL,
// 	action TEXT NOT NULL,
// 	create_time DATETIME NOT NULL
// );`
// 	_, err := db.Exec(sqlStr)
// 	return err
// }

// InsertAuditLog 插入一条审计记录
func InsertAuditLog(userid int, action string, detail string, ipaddr string) error {
	_, err := db.Exec(`
INSERT INTO audit(user_id, action, detail, ip_addr)
VALUES(?, ?, ?, ?)`, userid, action, detail, ipaddr)
	return err
}

// QueryAuditLogByUser 查询某个用户自己的审计日志（用户只能看自己的）
func QueryAuditLogByUser(userid int) ([]*AuditLog, error) {
	rows, err := db.Query(`
SELECT id,user_id,action,detail,create_time,ip_addr
FROM audit_log
WHERE user_id = ?
ORDER BY create_time DESC`, userid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*AuditLog
	for rows.Next() {
		var item AuditLog
		err := rows.Scan(&item.ID, &item.UserID, &item.Action, &item.Detail, &item.CreateTime, &item.IpAddr)
		if err != nil {
			return nil, err
		}
		list = append(list, &item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
