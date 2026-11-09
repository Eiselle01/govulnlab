package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Eiselle01/govulnlab/internal/httpx"
	"github.com/Eiselle01/govulnlab/internal/reqctx"
)

// tokenBucketLua 实现分布式令牌桶。
//
// 选 Lua 而不是「先 GET 再 SET」：多实例部署时，两段式读改写会产生竞态，
// 攻击者并发打请求就能把限额放大数倍。Redis 单线程执行 Lua 保证原子性。
//
// KEYS[1] = 桶 key
// ARGV[1] = 每秒补充令牌数(rate)  ARGV[2] = 桶容量(burst)  ARGV[3] = 当前毫秒时间戳
// 返回 1 放行 / 0 拒绝
const tokenBucketLua = `
local key  = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now  = tonumber(ARGV[3])

local data  = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts     = tonumber(data[2])
if tokens == nil then tokens = burst end
if ts == nil then ts = now end

local delta = math.max(0, now - ts)
tokens = math.min(burst, tokens + delta * rate / 1000)

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call('HSET', key, 'tokens', tokens, 'ts', now)
-- 桶空后最多 burst/rate 秒就能回满，过期时间按这个量给，避免冷 key 长期占用内存
redis.call('PEXPIRE', key, math.ceil(burst / rate * 1000) + 1000)
return allowed
`

var tokenBucketScript = redis.NewScript(tokenBucketLua)

// RateLimiter 令牌桶限流器配置。
type RateLimiter struct {
	rdb   *redis.Client
	log   *slog.Logger
	name  string
	rate  float64
	burst int
}

func NewRateLimiter(rdb *redis.Client, log *slog.Logger, name string, ratePerSec float64, burst int) *RateLimiter {
	return &RateLimiter{rdb: rdb, log: log, name: name, rate: ratePerSec, burst: burst}
}

// Middleware 以「限流器名 + 客户端 IP」为桶维度，对同一来源限速。
func (l *RateLimiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 健康探针不限流，否则 K8s 探活会被误杀。
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}

			key := "gvl:rl:" + l.name + ":" + reqctx.ClientIP(r.Context())
			allowed, err := tokenBucketScript.Run(r.Context(), l.rdb, []string{key},
				l.rate, l.burst, time.Now().UnixMilli()).Int()
			if err != nil {
				// Redis 故障时选择「放行」而不是「全站 503」：可用性优先，并留下告警日志。
				l.log.Error("限流器执行失败，本次放行", "err", err, "bucket", l.name,
					"request_id", reqctx.RequestID(r.Context()))
				next.ServeHTTP(w, r)
				return
			}
			if allowed == 0 {
				w.Header().Set("Retry-After", "1")
				httpx.JSON(w, http.StatusTooManyRequests, httpx.Envelope{
					Code:      "rate_limited",
					Msg:       "请求过于频繁，请稍后再试",
					RequestID: reqctx.RequestID(r.Context()),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
