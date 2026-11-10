package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Eiselle01/govulnlab/internal/apperr"
	"github.com/Eiselle01/govulnlab/internal/store"
)

// OrderService 订单业务。所有“是不是我的订单”的判断都在这一层收口，
// handler 不允许自己去查库判断归属，避免同一规则散落多处后被漏改。
type OrderService struct {
	st  *store.Store
	log *slog.Logger
}

func NewOrderService(st *store.Store, log *slog.Logger) *OrderService {
	return &OrderService{st: st, log: log}
}

// Create 创建订单。
func (s *OrderService) Create(ctx context.Context, userID int64, goodsName, amountRaw string) (*store.Order, error) {
	goodsName = strings.TrimSpace(goodsName)
	if utf8.RuneCountInString(goodsName) == 0 {
		return nil, apperr.BadRequest("商品名称不能为空")
	}
	if utf8.RuneCountInString(goodsName) > 128 {
		return nil, apperr.BadRequest("商品名称不能超过 128 个字符")
	}

	cent, err := ParseAmount(amountRaw)
	if err != nil {
		return nil, apperr.BadRequest(err.Error())
	}

	o := &store.Order{
		OrderNo:    newOrderNo(),
		UserID:     userID,
		GoodsName:  goodsName,
		AmountCent: cent,
	}
	audit := auditFromCtx(ctx, userID, "create_order",
		fmt.Sprintf("创建订单 %s，商品 %s，金额 %s 元", o.OrderNo, goodsName, AmountToYuan(cent)))

	if err := s.st.CreateOrder(ctx, o, audit); err != nil {
		return nil, apperr.Internal("创建订单失败", err)
	}
	return o, nil
}

// Detail 查询订单详情。归属校验在 SQL 里完成，查不到时返回 404 而不是 403：
// 403 等于告诉攻击者“这个订单存在，只是你没权限”，方便他枚举 id 做规模探测。
func (s *OrderService) Detail(ctx context.Context, userID, orderID int64) (*store.Order, error) {
	o, err := s.st.GetOrder(ctx, orderID, userID)
	if err != nil {
		if isErr(err, store.ErrNotFound) {
			// 这里已无法区分“订单不存在”和“不是你的订单”，正是想要的效果：
			// 两种情况对外表现一致，攻击者拿不到任何“这个 id 存在”的信号。
			s.log.Warn("订单不存在或越权访问被拦截", "user_id", userID, "order_id", orderID)
			return nil, apperr.NotFound("订单不存在")
		}
		return nil, apperr.Internal("查询订单失败", err)
	}
	return o, nil
}

// Cancel 取消订单。
func (s *OrderService) Cancel(ctx context.Context, userID, orderID int64) error {
	err := s.st.CancelOrder(ctx, userID, orderID)
	switch {
	case err == nil:
		return nil
	case isErr(err, store.ErrNotFound), isErr(err, store.ErrNotOwner):
		return apperr.NotFound("订单不存在")
	case isErr(err, store.ErrInvalidState):
		return apperr.Conflict("只有待支付的订单可以取消")
	default:
		return apperr.Internal("取消订单失败", err)
	}
}

// ListMine 查询当前用户的订单（分页）。
func (s *OrderService) ListMine(ctx context.Context, userID int64, offset, size int) (PageResult[*store.Order], error) {
	items, err := s.st.ListOrdersByUser(ctx, userID, offset, size)
	if err != nil {
		return PageResult[*store.Order]{}, apperr.Internal("查询订单失败", err)
	}
	total, err := s.st.CountOrdersByUser(ctx, userID)
	if err != nil {
		return PageResult[*store.Order]{}, apperr.Internal("统计订单失败", err)
	}
	return PageResult[*store.Order]{Items: items, Total: total, Page: offset/size + 1, Size: size}, nil
}

// ListAll 管理员全量订单（分页）。
func (s *OrderService) ListAll(ctx context.Context, offset, size int) (PageResult[*store.Order], error) {
	items, err := s.st.ListAllOrders(ctx, offset, size)
	if err != nil {
		return PageResult[*store.Order]{}, apperr.Internal("查询订单失败", err)
	}
	total, err := s.st.CountAllOrders(ctx)
	if err != nil {
		return PageResult[*store.Order]{}, apperr.Internal("统计订单失败", err)
	}
	return PageResult[*store.Order]{Items: items, Total: total, Page: offset/size + 1, Size: size}, nil
}

// newOrderNo 生成业务订单号：时间前缀（便于人工排序）+ UUID 片段（保证唯一）。
func newOrderNo() string {
	return "ORD" + time.Now().Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
}
