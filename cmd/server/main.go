package main

import (
	"context"
	"strconv"

	"BlahajChatServer/config"
	"BlahajChatServer/internal/bus"
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/redis"
	"BlahajChatServer/internal/router"
	"BlahajChatServer/internal/ws"
	"BlahajChatServer/internal/zlog"
)

func main() {
	config.InitConfig()
	zlog.Init()
	defer zlog.Sync()

	zlog.Info("服务启动中", "env", config.CFG.Server.Env, "port", config.CFG.Server.Port)

	dao.InitMySQL()
	redis.InitRedis()

	if err := ws.InitHub(context.Background(), bus.KafkaConfig{
		Brokers: config.CFG.Kafka.Brokers,
		Topic:   config.CFG.Kafka.Topic,
		GroupID: config.CFG.Kafka.GroupID,
	}); err != nil {
		zlog.Fatal("WS Hub 初始化失败", "err", err)
	}
	defer func() {
		if err := ws.CloseHub(); err != nil {
			zlog.Warn("WS Hub 关闭失败", "err", err)
		}
	}()

	router.Init()
	if err := router.GE.Run(":" + strconv.Itoa(config.CFG.Server.Port)); err != nil {
		zlog.Fatal("HTTP 服务启动失败", "err", err)
	}
}
