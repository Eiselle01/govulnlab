// Package service 是业务层：负责业务规则、事务编排与错误语义。
//
// 它不直接碰 http.Request / ResponseWriter，因此可以被单元测试、CLI、定时任务复用；
// 这也是这套分层最值钱的地方——业务逻辑可测、可复用、不依赖框架。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Eiselle01/govulnlab/internal/apperr"
	"github.com/Eiselle01/govulnlab/internal/config"
	"github.com/Eiselle01/govulnlab/internal/store"
)

// bcryptCost 12 比默认 10 更抗暴力破解，代价是单次哈希约 20~40ms。
// 登录接口本身有限流，这个成本可以接受；注册/改密是低频操作，更无所谓。
const bcryptCost = 12

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

// AuthService 处理注册、登录、改密三件事。
type AuthService struct {
	st        *store.Store
	log       *slog.Logger
	sec       config.Security
	blockFor  time.Duration
	dummyHash []byte // 用户不存在时也做一次 bcrypt 比较，抹平响应时间差异
}

func NewAuthService(st *store.Store, log *slog.Logger, cfg *config.Config) *AuthService {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	if err != nil {
		// 极端情况下生成失败也不该阻断启动：置空后走普通错误分支。
		log.Warn("生成占位哈希失败，用户不存在分支的耗时防护将关闭", "err", err)
		dummy = nil
	}
	return &AuthService{
		st:        st,
		log:       log,
		sec:       cfg.Security,
		blockFor:  cfg.LoginBlockDuration(),
		dummyHash: dummy,
	}
}

// ValidateUsername 用户名只允许字母、数字、下划线，长度 3~32。
func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return apperr.BadRequest("用户名只能包含字母、数字、下划线，长度 3~32")
	}
	return nil
}

// ValidatePassword 密码策略：长度达标、同时包含字母和数字、不能是同一个字符重复。
// 不追求复杂到反人类的规则，但要挡住 "123456"、"aaaaaaaa" 这类弱口令。
func ValidatePassword(pwd string, minLen int) error {
	if len(pwd) < minLen {
		return apperr.BadRequest(fmt.Sprintf("密码长度至少 %d 位", minLen))
	}
	if len(pwd) > 72 {
		// bcrypt 只接受前 72 字节，超出部分会被静默忽略，必须显式拒绝。
		return apperr.BadRequest("密码长度不能超过 72 字节")
	}
	hasLetter, hasDigit := false, false
	for _, c := range pwd {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return apperr.BadRequest("密码必须同时包含字母和数字")
	}
	if strings.Count(pwd, pwd[:1]) == len(pwd) {
		return apperr.BadRequest("密码不能是同一个字符重复")
	}
	return nil
}

// Register 注册新用户。
func (s *AuthService) Register(ctx context.Context, username, password string) (int64, error) {
	username = strings.TrimSpace(username)
	if err := ValidateUsername(username); err != nil {
		return 0, err
	}
	if err := ValidatePassword(password, s.sec.PasswordMinLen); err != nil {
		return 0, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return 0, apperr.Internal("密码加密失败", err)
	}
	id, err := s.st.CreateUser(ctx, username, string(hash))
	if err != nil {
		if isErr(err, store.ErrDuplicate) {
			return 0, apperr.Conflict("用户名已存在")
		}
		return 0, apperr.Internal("注册失败", err)
	}
	return id, nil
}

// Login 校验凭证。失败次数计入 Redis，达到阈值后锁定一段时间。
// 返回的用户对象交给 handler 生成会话。
func (s *AuthService) Login(ctx context.Context, ip, username, password string) (*store.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, apperr.BadRequest("用户名和密码不能为空")
	}

	key := s.failKey(ip, username)
	if n, err := s.failCount(ctx, key); err == nil && n >= int64(s.sec.LoginMaxFail) {
		return nil, apperr.TooManyRequests(fmt.Sprintf("登录失败次数过多，请 %d 分钟后再试", s.sec.LoginBlockMinutes))
	}

	user, err := s.st.GetUserByUsername(ctx, username)
	if err != nil {
		if isErr(err, store.ErrNotFound) {
			// 用户不存在与密码错误返回同一句话，避免账号枚举；
			// 同时做一次等价耗时的 bcrypt 比较，抹平时间差。
			if s.dummyHash != nil {
				_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
			}
			s.recordFailure(ctx, key)
			return nil, apperr.Unauthorized("用户名或密码错误")
		}
		return nil, apperr.Internal("登录失败", err)
	}

	if user.Status != 1 {
		s.recordFailure(ctx, key)
		return nil, apperr.Unauthorized("账号已被禁用")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.recordFailure(ctx, key)
		return nil, apperr.Unauthorized("用户名或密码错误")
	}

	// 登录成功清空失败计数，避免“历史失败”把正常用户越锁越久。
	_ = s.st.RDB.Del(ctx, key).Err()
	return user, nil
}

// ChangePassword 校验旧密码后更新，改密成功后调用方必须让旧会话失效。
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, oldPwd, newPwd string) error {
	if oldPwd == "" || newPwd == "" {
		return apperr.BadRequest("新旧密码都不能为空")
	}
	if err := ValidatePassword(newPwd, s.sec.PasswordMinLen); err != nil {
		return err
	}
	if oldPwd == newPwd {
		return apperr.BadRequest("新密码不能与旧密码相同")
	}

	user, err := s.st.GetUserByID(ctx, userID)
	if err != nil {
		if isErr(err, store.ErrNotFound) {
			return apperr.Unauthorized("会话已失效，请重新登录")
		}
		return apperr.Internal("读取用户失败", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPwd)); err != nil {
		return apperr.BadRequest("旧密码不正确")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcryptCost)
	if err != nil {
		return apperr.Internal("密码加密失败", err)
	}
	if err := s.st.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return apperr.Internal("更新密码失败", err)
	}
	return nil
}

// failKey 不把用户名明文写进 Redis key（key 可能被导出到监控面板），用哈希代替。
func (s *AuthService) failKey(ip, username string) string {
	h := sha256.Sum256([]byte(strings.ToLower(username)))
	return "gvl:login:fail:" + ip + ":" + hex.EncodeToString(h[:8])
}

func (s *AuthService) failCount(ctx context.Context, key string) (int64, error) {
	n, err := s.st.RDB.Get(ctx, key).Int64()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *AuthService) recordFailure(ctx context.Context, key string) {
	n, err := s.st.RDB.Incr(ctx, key).Result()
	if err != nil {
		s.log.Warn("记录登录失败次数出错", "err", err)
		return
	}
	if n == 1 {
		_ = s.st.RDB.Expire(ctx, key, s.blockFor).Err()
	}
}

func isErr(err, target error) bool {
	return errors.Is(err, target)
}
