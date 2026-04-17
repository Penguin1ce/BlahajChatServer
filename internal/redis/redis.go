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

// SetNXValueByKeyExpire 原子 SET ... NX EX ttl：仅当 key 不存在时写入并设置过期时间。
// 返回 ok=true 表示写入成功；ok=false 表示 key 已存在（常用于限流 / 幂等场景）。
func SetNXValueByKeyExpire(key string, value string, expire time.Duration) (bool, error) {
	_, err := RDB.SetArgs(ctx, key, value, redis.SetArgs{
		Mode: "NX",
		TTL:  expire,
	}).Result()
	if err != nil {
		// key 已存在时 redis 返回 nil，对应 go-redis 的 redis.Nil
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		zlog.Error("Redis SetArgs NX 失败", "key", key, "ttl", expire, "err", err)
		return false, err
	}
	return true, nil
}

func DelValueByKey(key string) {
	if err := RDB.Del(ctx, key).Err(); err != nil {
		zlog.Error("Redis 删除失败", "key", key, "err", err)
	}
}

func ExistsKey(key string) bool {
	n, err := RDB.Exists(ctx, key).Result()
	// 先不管系统错误
	if err != nil {
		zlog.Error("Redis Exists 失败", "key", key, "err", err)
		return false
	}
	return n > 0
}

func ExpireKey(key string, expire time.Duration) error {
	if err := RDB.Expire(ctx, key, expire).Err(); err != nil {
		zlog.Error("Redis Expire 失败", "key", key, "ttl", expire, "err", err)
		return err
	}
	return nil
}
