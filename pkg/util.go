package pkg

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// GlobalCtx 全局上下文，redis操作统一使用
var GlobalCtx = context.Background()

type contextKey string

const SessionCtxKey contextKey = "session_data"

// GenSessionID 生成sid、csrf_token随机字符串
func GenSessionID() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// SessionData Redis存储的会话结构体，代替map，强类型
type SessionData struct {
	Username  string `json:"username"`
	CsrfToken string `json:"csrf_token"`
}

// GetSessionFromRedis 读取http请求，拿到sid cookie，从redis读取会话
// 返回：(会话数据, 是否登录, error)
func GetSessionFromRedis(r *http.Request) (*SessionData, bool, error) {

	// 1. 打印所有请求头
	fmt.Printf("📋 请求头: %+v\n", r.Header)

	// 2. 尝试读取 cookie
	cookie, err := r.Cookie("mysession")
	if err != nil {
		fmt.Printf("❌ 读取 cookie 失败: %v\n", err)
		return nil, false, err
	}
	fmt.Printf("✅ 读取到 cookie: %s\n", cookie.Value)

	// 3. 从 Redis 获取会话
	val, err := RedisCli.Get(GlobalCtx, cookie.Value).Result()
	if err != nil {
		fmt.Printf("❌ Redis 查询失败: %v\n", err)
		return nil, false, err
	}
	fmt.Printf("✅ Redis 数据: %s\n", val)

	// 4. 反序列化
	var data SessionData
	err = json.Unmarshal([]byte(val), &data)
	if err != nil {
		fmt.Printf("❌ JSON 反序列化失败: %v\n", err)
		return nil, false, err
	}

	return &data, true, nil
}
