package service

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/Eiselle01/govulnlab/internal/apperr"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
	"github.com/Eiselle01/govulnlab/internal/store"
)

// GuestbookService 留言板业务。
type GuestbookService struct {
	st  *store.Store
	log *slog.Logger
}

func NewGuestbookService(st *store.Store, log *slog.Logger) *GuestbookService {
	return &GuestbookService{st: st, log: log}
}

// Add 提交留言。内容原样入库，输出侧由 html/template 负责转义。
func (s *GuestbookService) Add(ctx context.Context, userID int64, content string) error {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) == 0 {
		return apperr.BadRequest("留言内容不能为空")
	}
	if utf8.RuneCountInString(content) > 1000 {
		return apperr.BadRequest("留言不能超过 1000 个字符")
	}
	if _, err := s.st.AddNote(ctx, userID, content); err != nil {
		return apperr.Internal("提交留言失败", err)
	}
	return nil
}

func (s *GuestbookService) List(ctx context.Context, offset, size int) (PageResult[*store.Note], error) {
	items, err := s.st.ListNotes(ctx, offset, size)
	if err != nil {
		return PageResult[*store.Note]{}, apperr.Internal("读取留言失败", err)
	}
	total, err := s.st.CountNotes(ctx)
	if err != nil {
		return PageResult[*store.Note]{}, apperr.Internal("统计留言失败", err)
	}
	return PageResult[*store.Note]{Items: items, Total: total, Page: offset/size + 1, Size: size}, nil
}

// AuditService 审计写入与查询。
//
// 审计字段（IP / UA / request_id）在这里统一补齐，业务层不用关心怎么取。
type AuditService struct {
	st  *store.Store
	log *slog.Logger
}

func NewAuditService(st *store.Store, log *slog.Logger) *AuditService {
	return &AuditService{st: st, log: log}
}

// Record 写一条审计日志。注意：审计失败只记 error 日志，不阻断主业务流程，
// 否则“审计挂了”会连带业务不可用；但订单场景例外（与订单同事务，见 store.CreateOrder）。
func (s *AuditService) Record(ctx context.Context, userID int64, action, detail string) {
	a := auditFromCtx(ctx, userID, action, detail)
	if err := s.st.InsertAudit(ctx, a); err != nil {
		s.log.Error("写入审计日志失败", "err", err, "action", action)
	}
}

func (s *AuditService) ListMine(ctx context.Context, userID int64, offset, size int) (PageResult[*store.AuditLog], error) {
	items, err := s.st.ListAuditByUser(ctx, userID, offset, size)
	if err != nil {
		return PageResult[*store.AuditLog]{}, apperr.Internal("查询审计日志失败", err)
	}
	total, err := s.st.CountAuditByUser(ctx, userID)
	if err != nil {
		return PageResult[*store.AuditLog]{}, apperr.Internal("统计审计日志失败", err)
	}
	return PageResult[*store.AuditLog]{Items: items, Total: total, Page: offset/size + 1, Size: size}, nil
}

func (s *AuditService) ListAll(ctx context.Context, offset, size int) (PageResult[*store.AuditLog], error) {
	items, err := s.st.ListAllAudit(ctx, offset, size)
	if err != nil {
		return PageResult[*store.AuditLog]{}, apperr.Internal("查询审计日志失败", err)
	}
	total, err := s.st.CountAllAudit(ctx)
	if err != nil {
		return PageResult[*store.AuditLog]{}, apperr.Internal("统计审计日志失败", err)
	}
	return PageResult[*store.AuditLog]{Items: items, Total: total, Page: offset/size + 1, Size: size}, nil
}

// auditFromCtx 从请求上下文补齐审计元数据，并做字段截断（表结构有长度限制）。
func auditFromCtx(ctx context.Context, userID int64, action, detail string) store.AuditLog {
	return store.AuditLog{
		UserID:    userID,
		Action:    clip(action, 64),
		Detail:    clip(detail, 512),
		IP:        clip(reqctx.ClientIP(ctx), 64),
		UA:        clip(reqctx.UserAgent(ctx), 255),
		RequestID: clip(reqctx.RequestID(ctx), 64),
	}
}

func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}
