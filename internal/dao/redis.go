package dao

import (
	"context"
	"fmt"
	"log"
	"time"

	"BlahajChatServer/config"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func InitRedis() {
	c := config.CFG.Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Password: c.Password,
		DB:       c.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Fatal("连接 Redis 失败 ", err)
	}
}
