package pkg

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() (*sql.DB, error) {
	// 打开数据库文件，不存在则自动创建test.db
	var err error
	db, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		panic("数据库打开失败: " + err.Error())
	}

	// // SQL语句：没有users表就新建一张users表
	// createTableSql := `
	// CREATE TABLE IF NOT EXISTS users (
	// 	id INTEGER PRIMARY KEY AUTOINCREMENT,
	// 	username TEXT NOT NULL UNIQUE,
	// 	password TEXT NOT NULL
	// );`

	// 执行建表SQL
	// _, err = db.Exec(createTableSql)
	// if err != nil {
	// 	panic("创建users表失败: " + err.Error())
	// }
	// createOrderTableSql := `
	// CREATE TABLE IF NOT EXISTS orders (
	// 	id INTEGER PRIMARY KEY AUTOINCREMENT,
	// 	username TEXT NOT NULL,
	// 	title TEXT NOT NULL,
	// 	amount INTEGER NOT NULL,
	// 	create_time DATETIME DEFAULT CURRENT_TIMESTAMP
	// );`
	// _, err = db.Exec(createOrderTableSql)
	// if err != nil {
	// 	panic("创建orders表失败:" + err.Error())
	// }
	sqlBytes, err := os.ReadFile("./migrations/001_init_schema.sql")
	if err != nil {
		panic("读取迁移文件失败:" + err.Error())
	}

	// 执行全部sql
	_, err = db.Exec(string(sqlBytes))
	if err != nil {
		panic("执行建表SQL失败:" + err.Error())
	}
	DB = db
	fmt.Println("数据库初始化成功,users,orders表就绪")

	return DB, nil
	// ✅新增sessions会话表
	// _, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
	// 	sid TEXT PRIMARY KEY,
	// 	username TEXT NOT NULL,
	// 	csrf_token TEXT NOT NULL,
	// 	create_time INTEGER NOT NULL
	// )`)
	// if err != nil {
	// 	panic(err)
	// }
}
