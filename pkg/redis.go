package pkg

import (
	// "context"
	// "crypto/rand"
	// "encoding/hex"

	"github.com/redis/go-redis/v9"
)

var (
	RedisCli  *redis.Client
	// GlobalCtx = context.Background()
)

// // 初始化redis
// func initRedis() {
// 	redisCli = redis.NewClient(&redis.Options{
// 		Addr:     "127.0.0.1:6379",
// 		Password: "",
// 		DB:       0,
// 	})
// }

func InitRedis() error {
	RedisCli = redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})
	// 一定要做一次ping测试，初始化就检测连通性
	_, err := RedisCli.Ping(GlobalCtx).Result()
	return err
}

//在将session储存到Redis后就不需要这些了·
// session存储：key=sessionId，value=登录的用户名
// var (
// 	sessionStore = make(map[string]SessionData) //sid->username
// 	sessionMutex sync.Mutex                     //读写sessionmap必须加这个锁
// )

// type SessionData struct {
// 	Username  string
// 	CsrfToken string
// }

// func GenSessionID() string {
// 	buf := make([]byte, 16)
// 	_, _ = rand.Read(buf)
// 	return hex.EncodeToString(buf)
// }
