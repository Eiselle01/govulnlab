package model

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// User 用户表结构体，对应users数据库表
type User struct {
	ID       int
	Username string
	Role 	string
	Password string //存bcrypt哈希字符串，不是明文
}

var db *sql.DB

// InitDB 注入db实例，model层拿到sqlite连接
func InitDB(sqlDB *sql.DB) {
	db = sqlDB
}

// CreateUser 注册：新建用户，传入用户名、明文密码，内部生成bcrypt哈希存入数据库
func CreateUser(username string, rawPassword string) (int64, error) {
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0,err
	}
	hashPwd := string(hashBytes)

	res, err := db.Exec(`INSERT INTO users(username,password) VALUES(?,?)`, username, hashPwd)
	if err!=nil{
		fmt.Errorf("用户创建失败，用户名可能被占用")
		return 0, err
	}
	userId, err :=res.LastInsertId()
	
	return userId, err
}

// QueryUserByName 根据用户名查询用户，返回用户结构体
func QueryUserByName(username string) (*User, error) {
	var u User
	row := db.QueryRow(`SELECT id,username,role,password FROM users WHERE username = ?`, username)
	err := row.Scan(&u.ID, &u.Username, &u.Role,&u.Password)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CheckPassword 比对明文密码和数据库存的hash
func CheckPassword(hashPwd string, rawPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashPwd), []byte(rawPwd))
	return err == nil
}

// UpdateUserPwd 更新用户密码，传入用户名、明文新密码，内部自动bcrypt加密
func UpdateUserPwd(username string, newRawPwd string) error {
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newRawPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hash := string(hashBytes)
	_, err = db.Exec(`UPDATE users SET password = ? WHERE username = ?`, hash, username)
	return err
}