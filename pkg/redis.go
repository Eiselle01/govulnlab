package pkg

import (
	// "context"
	// "crypto/rand"
	// "encoding/hex"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	RedisCli *redis.Client
	// GlobalCtx = context.Background()
)

// 初始化redis
// func initRedis() {
// 	redisCli = redis.NewClient(&redis.Options{
// 		Addr:     "127.0.0.1:6379",
// 		Password: "",
// 		DB:       0,
// 	})
// }

func InitRedis() error {
	RedisCli = redis.NewClient(&redis.Options{
		Addr:     GlobalConfig.Redis.Addr,
		Password: GlobalConfig.Redis.Password,
		DB:       GlobalConfig.Redis.DB,
	})

	_, err := RedisCli.Ping(GlobalCtx).Result()
	if err != nil {
		panic("redis连接失败: " + err.Error())
	}
	fmt.Println("✅redis初始化成功")
	return nil
}
