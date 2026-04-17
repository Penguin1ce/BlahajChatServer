package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"BlahajChatServer/config"
	"BlahajChatServer/internal/zlog"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client
var ctx = context.Background()

func InitRedis() {
	c := config.GetConfig().Redis
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: c.Password,
		DB:       c.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := RDB.Ping(pingCtx).Err(); err != nil {
		zlog.Fatal("连接 Redis 失败", "addr", addr, "err", err)
	}
	zlog.Info("Redis 连接成功", "addr", addr, "db", c.DB)
}

func GetValueByKey(key string) (string, error) {
	value, err := RDB.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Warn("Redis Key 不存在", "key", key)
			return "", nil
		}
		zlog.Error("Redis 读取失败", "key", key, "err", err)
		return "", err
	}
	return value, nil
}

func SetValueByKey(key string, value string) error {
	if err := RDB.Set(ctx, key, value, 0).Err(); err != nil {
		zlog.Error("Redis 写入失败", "key", key, "err", err)
		return err
	}
	return nil
}

func SetValueByKeyExpire(key string, value string, expire time.Duration) error {
	if err := RDB.Set(ctx, key, value, expire).Err(); err != nil {
		zlog.Error("Redis 写入失败", "key", key, "ttl", expire, "err", err)
		return err
	}
	return nil
}

func DelValueByKey(key string) error {
	if err := RDB.Del(ctx, key).Err(); err != nil {
		zlog.Error("Redis 删除失败", "key", key, "err", err)
		return err
	}
	return nil
}

func ExistsKey(key string) (bool, error) {
	n, err := RDB.Exists(ctx, key).Result()
	if err != nil {
		zlog.Error("Redis Exists 失败", "key", key, "err", err)
		return false, err
	}
	return n > 0, nil
}

func ExpireKey(key string, expire time.Duration) error {
	if err := RDB.Expire(ctx, key, expire).Err(); err != nil {
		zlog.Error("Redis Expire 失败", "key", key, "ttl", expire, "err", err)
		return err
	}
	return nil
}
