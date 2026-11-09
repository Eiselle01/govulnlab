package model

import (
	// "database/sql"
)

// GuestBook 留言板结构体，对应guestbook表
type GuestBook struct {
	ID       int
	Username string //留言人
	Content  string //留言内容
}

// CreateGuestBookTable 创建留言板表，程序启动调用
// func CreateGuestBookTable(db *sql.DB) error {
// 	sqlStr := `
// CREATE TABLE IF NOT EXISTS guestbook (
// 	id INTEGER PRIMARY KEY AUTO_INCREMENT,
// 	username TEXT NOT NULL,
// 	content TEXT NOT NULL
// );`
// 	_, err := db.Exec(sqlStr)
// 	return err
// }

// AddMessage 添加一条留言
func AddMessage(username string, content string) error {
	_, err := db.Exec(`INSERT INTO guestbook(username,content) VALUES(?,?)`, username, content)
	return err
}

// ListAllMessage 查询全部留言，按id倒序，最新留言在最上面
func ListAllMessage() ([]*GuestBook, error) {
	rows, err := db.Query(`SELECT id,username,content FROM guestbook ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*GuestBook
	for rows.Next() {
		var g GuestBook
		err := rows.Scan(&g.ID, &g.Username, &g.Content)
		if err != nil {
			return nil, err
		}
		list = append(list, &g)
	}
	return list, nil
}