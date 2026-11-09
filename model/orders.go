package model

import (
	"fmt"
	"time"

	"github.com/Eiselle01/go-security-lab/pkg" //替换成你go.mod里面module名字
	"github.com/google/uuid"
)

// Order 订单数据结构体，对应数据库orders表一行记录
type Order struct {
	ID         int
	OrderNo    string
	UserID     int
	Amount     float64
	Status     int
	GoodsName  string
	CreateTime time.Time
	UpdateTime time.Time
}

// GetOrderByID 查询订单同时联表拿到用户名
type OrderWithUser struct {
	Order
	Username string
}

// CreateOrder 创建订单
// 参数：归属用户名、订单标题、金额；返回创建好的订单对象，或者error
func CreateOrder(userID int, goodsName string, amount float64) (*Order, error) {

	// 生成唯一订单号，简单方案：时间+随机串
	orderNo := fmt.Sprintf("%d_%s", time.Now().Unix(), uuid.NewString())
	tx, err := pkg.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Exec执行insert写入数据库
	res, err := pkg.DB.Exec(
		`INSERT INTO orders(userid,goodsname,amount,orderNo,status) VALUES(?,?,?,?,?)`,
		userID, goodsName, amount, orderNo, 1)
	if err != nil {
		return nil, err
	}

	// LastInsertId 获取数据库自增生成的id
	lastID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	actionDetail := fmt.Sprintf("创建订单，订单号:%s,商品:%s,金额:%d", orderNo, goodsName, amount)
	_, err = tx.Exec(`INSERT INTO audit(user_id, action) VALUES(?,?)`, userID, actionDetail)
	if err != nil {
		return nil, err
	}
	// 提交事务
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	order := &Order{
		ID:        int(lastID),
		OrderNo:   orderNo,
		UserID:    userID,
		Amount:    amount,
		Status:    1,
		GoodsName: goodsName,
		// CreateTime数据库DEFAULT自动填充，Go这里不用赋值
	}
	return order, nil
}

// GetOrderByID 通过订单id查询订单
// 【注意：这个函数只根据id查，不会校验归属用户，后续handler层做权限校验】
func GetOrderByID(orderID int) (*OrderWithUser, error) {
	var owu OrderWithUser
	row := pkg.DB.QueryRow(`
		SELECT o.id,o.user_id,o.goodsname,o.amount,o.create_time,u.username
		FROM orders o
		JOIN users u ON o.user_id = u.id
		WHERE o.id = ?
	`, orderID)

	// Scan把数据库行的字段，赋值到结构体变量
	err := row.Scan(&owu.ID, &owu.UserID, &owu.GoodsName, &owu.Amount, &owu.CreateTime, &owu.Username)
	if err != nil {
		return nil, err
	}
	return &owu, nil
}

// ListUserOrder 查询某一个用户名下所有订单
func ListUserOrder(userid int) ([]*Order, error) {
	rows, err := pkg.DB.Query(`
		SELECT id,orderno,userid, amount,status, goodsname, create_time, updatetime
		FROM orders WHERE user_id = ?
	`, userid)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //❗查询结束必须关闭行，释放数据库连接

	var orderList []*Order
	for rows.Next() { //循环读取每一行
		var o Order
		err := rows.Scan(&o.ID, &o.UserID, &o.GoodsName, &o.Amount, &o.CreateTime)
		if err != nil {
			return nil, err
		}
		orderList = append(orderList, &o)
	}
	return orderList, nil
}

// CancelOrder 取消订单，事务：更新订单状态 + 写入审计日志
func CancelOrder(userID int, orderID int) error {
	tx, err := pkg.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 第一步：先查询这条订单是否属于当前用户（越权校验！）
	var ownerID int
	err = tx.QueryRow(`SELECT userid FROM orders WHERE id = ?`, orderID).Scan(&ownerID)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return fmt.Errorf("无权操作此订单")
	}

	// 更新订单状态为 2=已取消
	_, err = tx.Exec(`UPDATE orders SET status=? WHERE id=?`, 2, orderID)
	if err != nil {
		return err
	}

	// 写入审计日志
	actionDetail := fmt.Sprintf("取消订单，订单ID:%d", orderID)
	_, err = tx.Exec(`INSERT INTO audit(user_id, action) VALUES(?,?)`, userID, actionDetail)
	if err != nil {
		return err
	}

	// 提交事务
	err = tx.Commit()
	return err
}
