package store

import "time"

// User 对应 users 表。PasswordHash 是 bcrypt 哈希，任何时候都不允许出现明文。
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	Status       int
	CreatedAt    time.Time
}

// Order 对应 orders 表。金额单位是「分」。
type Order struct {
	ID         int64
	OrderNo    string
	UserID     int64
	Username   string
	GoodsName  string
	AmountCent int64
	Status     int
	Remark     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// 订单状态常量，避免代码里到处写魔法数字。
const (
	OrderStatusPending  = 1
	OrderStatusPaid     = 2
	OrderStatusCanceled = 3
)

// StatusText 把状态码翻译成中文，模板层直接调用。
func (o *Order) StatusText() string {
	switch o.Status {
	case OrderStatusPending:
		return "待支付"
	case OrderStatusPaid:
		return "已支付"
	case OrderStatusCanceled:
		return "已取消"
	default:
		return "未知"
	}
}

// Note 对应 guestbook 表。
type Note struct {
	ID        int64
	UserID    int64
	Username  string
	Content   string
	CreatedAt time.Time
}

// AuditLog 对应 audit_log 表。RequestID 把审计记录和访问日志串起来。
type AuditLog struct {
	ID        int64
	UserID    int64
	Action    string
	Detail    string
	IP        string
	UA        string
	RequestID string
	CreatedAt time.Time
}

// LabUser 靶场专用用户，口令是明文（仅用于演示注入，真实系统必须 bcrypt）。
type LabUser struct {
	ID       int64
	Username string
	Password string
	Role     string
}

// LabOrder 靶场订单，用于越权演示。
type LabOrder struct {
	ID         int64
	UserID     int64
	OrderNo    string
	GoodsName  string
	AmountCent int64
	Status     int
	CreatedAt  time.Time
}

// LabNote 靶场留言，用于存储型 XSS 演示。
type LabNote struct {
	ID        int64
	Author    string
	Content   string
	CreatedAt time.Time
}
