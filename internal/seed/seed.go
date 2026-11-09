// Package seed 初始化演示数据：管理员、普通用户、靶场账号与靶场订单。
// 设计成幂等：重复执行不会产生重复数据，也不会覆盖已有用户的密码。
package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/Eiselle01/govulnlab/internal/store"
)

type Options struct {
	AdminUsername string
	AdminPassword string
	DemoUsername  string
	DemoPassword  string
}

// Result 返回实际创建/跳过的信息，方便 CLI 打印。
type Result struct {
	Created []string
	Skipped []string
}

func Run(ctx context.Context, st *store.Store, log *slog.Logger, opt Options) (Result, error) {
	var res Result

	adminID, created, err := ensureUser(ctx, st, opt.AdminUsername, opt.AdminPassword, "admin")
	if err != nil {
		return res, err
	}
	record(&res, opt.AdminUsername, created)

	demoID, created, err := ensureUser(ctx, st, opt.DemoUsername, opt.DemoPassword, "user")
	if err != nil {
		return res, err
	}
	record(&res, opt.DemoUsername, created)

	// 靶场账号：明文口令，仅用于演示注入/暴破，绝不与业务账号混用。
	labAccounts := []struct {
		username string
		password string
		role     string
	}{
		{"labadmin", "Lab@123456", "admin"},
		{"labuser", "Lab@123456", "user"},
	}
	for _, a := range labAccounts {
		if err := st.LabUpsertUser(ctx, a.username, a.password, a.role); err != nil {
			return res, err
		}
		res.Created = append(res.Created, "lab_user:"+a.username)
	}

	// 靶场订单：故意分属不同用户，用于水平越权演示。
	labOrders := []store.LabOrder{
		{UserID: adminID, OrderNo: "LAB-ADMIN-0001", GoodsName: "【admin 的订单】服务器续费", AmountCent: 129900, Status: 1},
		{UserID: adminID, OrderNo: "LAB-ADMIN-0002", GoodsName: "【admin 的订单】监控告警套餐", AmountCent: 59900, Status: 2},
		{UserID: demoID, OrderNo: "LAB-DEMO-0001", GoodsName: "【demo 的订单】测试商品", AmountCent: 1990, Status: 1},
	}
	for _, o := range labOrders {
		if err := st.LabUpsertOrder(ctx, o); err != nil {
			return res, err
		}
		res.Created = append(res.Created, "lab_order:"+o.OrderNo)
	}

	log.Info("演示数据初始化完成",
		"created", len(res.Created),
		"skipped", len(res.Skipped),
	)
	return res, nil
}

func record(res *Result, name string, created bool) {
	if created {
		res.Created = append(res.Created, "user:"+name)
	} else {
		res.Skipped = append(res.Skipped, "user:"+name)
	}
}

// ensureUser 存在则跳过，不存在则创建。返回用户 id 与是否新建。
func ensureUser(ctx context.Context, st *store.Store, username, password, role string) (int64, bool, error) {
	existing, err := st.GetUserByUsername(ctx, username)
	if err == nil {
		return existing.ID, false, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return 0, false, fmt.Errorf("查询用户 %s 失败: %w", username, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, false, fmt.Errorf("生成密码哈希失败: %w", err)
	}

	var id int64
	if role == "admin" {
		id, err = st.CreateAdmin(ctx, username, string(hash))
	} else {
		id, err = st.CreateUser(ctx, username, string(hash))
	}
	if err != nil {
		return 0, false, fmt.Errorf("创建用户 %s 失败: %w", username, err)
	}
	return id, true, nil
}
