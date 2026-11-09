package pkg

import (
	"database/sql"
	"fmt"
	// "os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() (*sql.DB, error) {
	// 打开数据库文件，不存在则自动创建test.db
	var err error
	// 从配置拿参数，不再硬编码
	db, err := sql.Open(GlobalConfig.DB.Driver, GlobalConfig.DB.Dsn)
	if err != nil {
		panic("数据库打开失败: " + err.Error())
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("数据库ping失败: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	// 根据驱动选择迁移脚本
	// var migrateFile string
	// if GlobalConfig.DB.Driver == "mysql" {
	// 	migrateFile = "./migrations/001_init_mysql.sql"
	// } else {
	// 	migrateFile = "./migrations/001_init_schema.sql"
	// }

	// sqlBytes, err := os.ReadFile(migrateFile)
	// if err != nil {
	// 	return nil, fmt.Errorf("读取迁移脚本失败: %w", err)
	// }

	// _, err = db.Exec(string(sqlBytes))
	// if err != nil {
	// 	return nil, fmt.Errorf("执行建表脚本失败: %w", err)
	// }

	DB = db
	fmt.Println("✅数据库初始化完成，driver:", GlobalConfig.DB.Driver)
	return DB, nil
}

//var err error
// db, err := sql.Open("sqlite3", "./test.db")
// if err != nil {
// 	panic("数据库打开失败: " + err.Error())
// }

// err = db.Ping()
// if err != nil {
// 	return nil, err
// }

// import _ "embed"
//   embed是 Go1.16 + 自带标准库，不需要 go get 安装。下划线_表示只引入包，不直接调用包内函数，只是启用//go:embed这个编译器指令。
//   //go:embed migrations/001_init_schema.sql
//   👉 这不是普通注释！是编译器指令
//   编译阶段（go build /go run），Go 编译器自动读取这个 sql 文件，把文件全部内容，直接塞到变量 schemaSQL 里面。
//   var schemaSQL string
//   编译完成后，schemaSQL这个字符串变量，就完整存着你001_init_schema.sql里面所有 SQL 文本。
//   最大好处（解决你现在的痛点）
//   // ✅ 运行程序的时候，不再去硬盘找这个 sql 文件！
//   文件内容打包进 exe 二进制程序里面。
//   // 不管你在哪里运行 exe，不管当前工作目录在哪，永远读得到 SQL，彻底解决os.ReadFile相对路径找不到文件的坑。

// sqlBytes, err := os.ReadFile("migrations/001_init_shema.sql")
// if err != nil {
// 	panic("读取迁移文件失败:" + err.Error())
// }
