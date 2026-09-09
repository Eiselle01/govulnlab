package model

import (
	"database/sql"
	"time"
)

// AuditLog 审计日志表
type AuditLog struct {
	ID         int
	Username   string    //操作人用户名
	Action     string    //操作描述："用户登录","修改密码","创建订单","提交留言","退出登录"
	CreateTime time.Time //操作发生时间
}

// CreateAuditLogTable 创建审计日志表，main启动调用
func CreateAuditLogTable(db *sql.DB) error {
	sqlStr := `
CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	action TEXT NOT NULL,
	create_time DATETIME NOT NULL
);`
	_, err := db.Exec(sqlStr)
	return err
}

// InsertAuditLog 插入一条审计记录
func InsertAuditLog(username string, action string) error {
	_, err := db.Exec(`
INSERT INTO audit_log(username, action, create_time)
VALUES(?, ?, ?)`, username, action, time.Now())
	return err
}

// QueryAuditLogByUser 查询某个用户自己的审计日志（用户只能看自己的）
func QueryAuditLogByUser(username string) ([]*AuditLog, error) {
	rows, err := db.Query(`
SELECT id,username,action,create_time
FROM audit_log
WHERE username = ?
ORDER BY create_time DESC`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*AuditLog
	for rows.Next() {
		var item AuditLog
		err := rows.Scan(&item.ID, &item.Username, &item.Action, &item.CreateTime)
		if err != nil {
			return nil, err
		}
		list = append(list, &item)
	}
	return list, nil
}