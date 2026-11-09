// Package session 实现基于 Redis 的服务端会话。
//
// 为什么不用 JWT：JWT 天然无法“立刻失效”（改密、踢人、封号都要等过期），
// 而这个项目把「会话可撤销 + 审计」当作工程要点，所以选择 Redis 集中式会话。
// Cookie 里只放一个高熵随机 ID，真正的身份数据留在服务端，客户端无法伪造。
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Eiselle01/govulnlab/internal/config"
)

// ErrNotFound 表示会话不存在或已过期。
var ErrNotFound = errors.New("session not found")

// Data 是会话里保存的身份信息，JSON 序列化后写进 Redis。
type Data struct {
	UserID    int64  `json:"uid"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CSRFToken string `json:"csrf"`
	IssuedAt  int64  `json:"iat"`
}

// IsAdmin 判断是否管理员。
func (d *Data) IsAdmin() bool { return d != nil && d.Role == "admin" }

type Manager struct {
	rdb        *redis.Client
	prefix     string
	cookieName string
	ttl        time.Duration
	secure     bool
}

func NewManager(rdb *redis.Client, cfg *config.Config) *Manager {
	return &Manager{
		rdb:        rdb,
		prefix:     "gvl:session:",
		cookieName: cfg.Session.CookieName,
		ttl:        cfg.SessionTTL(),
		secure:     cfg.Session.Secure,
	}
}

// GenerateID 生成 32 字节（256 bit）随机 ID，base64url 编码后 43 个字符。
// 256 bit 熵足够让 ID 不可枚举，避免会话预测。
func GenerateID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机 ID 失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (m *Manager) key(sid string) string { return m.prefix + sid }

// Create 新建会话；CSRFToken 为空时自动生成。
// 登录时必须是「先销毁旧会话、再建新会话」，否则攻击者预先塞给你的 session id
// 会在你登录后继续有效，这就是会话固定漏洞（Session Fixation）。
func (m *Manager) Create(ctx context.Context, d Data) (string, *Data, error) {
	sid, err := GenerateID()
	if err != nil {
		return "", nil, err
	}
	if d.CSRFToken == "" {
		d.CSRFToken, err = GenerateID()
		if err != nil {
			return "", nil, err
		}
	}
	if d.IssuedAt == 0 {
		d.IssuedAt = time.Now().Unix()
	}

	payload, err := json.Marshal(d)
	if err != nil {
		return "", nil, fmt.Errorf("序列化会话失败: %w", err)
	}
	if err := m.rdb.Set(ctx, m.key(sid), payload, m.ttl).Err(); err != nil {
		return "", nil, fmt.Errorf("写入会话失败: %w", err)
	}
	return sid, &d, nil
}

// Get 读取会话，并顺带续期（滑动过期）。
// 只续期一次过期时间的成本很低，换来的是活跃用户的会话不会中途失效。
func (m *Manager) Get(ctx context.Context, sid string) (*Data, error) {
	if sid == "" {
		return nil, ErrNotFound
	}
	raw, err := m.rdb.Get(ctx, m.key(sid)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取会话失败: %w", err)
	}

	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("反序列化会话失败: %w", err)
	}
	m.rdb.Expire(ctx, m.key(sid), m.ttl)
	return &d, nil
}

// Update 覆写已有会话内容（例如改密后更换 CSRF Token）。
func (m *Manager) Update(ctx context.Context, sid string, d Data) error {
	payload, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("序列化会话失败: %w", err)
	}
	return m.rdb.Set(ctx, m.key(sid), payload, m.ttl).Err()
}

// Renew 销毁旧会话并签发新会话，用于登录成功、修改密码等“权限变更”时刻。
func (m *Manager) Renew(ctx context.Context, oldSID string, d Data) (string, *Data, error) {
	if oldSID != "" {
		_ = m.Destroy(ctx, oldSID)
	}
	return m.Create(ctx, d)
}

func (m *Manager) Destroy(ctx context.Context, sid string) error {
	if sid == "" {
		return nil
	}
	if err := m.rdb.Del(ctx, m.key(sid)).Err(); err != nil {
		return fmt.Errorf("销毁会话失败: %w", err)
	}
	return nil
}

// SID 从 Cookie 中取出会话 ID。
func (m *Manager) SID(r *http.Request) (string, bool) {
	c, err := r.Cookie(m.cookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// SetCookie 写会话 Cookie。
// HttpOnly 阻断 XSS 直接读取会话；SameSite=Lax 缓解跨站 POST；
// Secure 在 HTTPS 环境必须打开（生产用 session.secure=true）。
func (m *Manager) SetCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(m.ttl.Seconds()),
	})
}

// ClearCookie 清除浏览器侧 Cookie（服务端会话另行 Destroy）。
func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (m *Manager) CookieName() string { return m.cookieName }
func (m *Manager) TTL() time.Duration { return m.ttl }
