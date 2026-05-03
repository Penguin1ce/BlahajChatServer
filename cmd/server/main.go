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
	ws.InitHub()

	fanout := func(_ context.Context, e bus.ChatEvent) error {
		ws.GlobalHub.Broadcast(&ws.Envelope{Targets: e.Targets, Data: e.Frame})
		return nil
	}
	if err := bus.InitKafka(context.Background(), bus.KafkaConfig{
		Brokers: config.CFG.Kafka.Brokers,
		Topic:   config.CFG.Kafka.Topic,
		GroupID: config.CFG.Kafka.GroupID,
	}, fanout); err != nil {
		zlog.Fatal("KafkaBus 初始化失败", "err", err)
	}
	defer func() {
		if err := bus.CloseGlobal(); err != nil {
			zlog.Warn("消息扇出通道关闭失败", "err", err)
		}
	}()

	router.Init()
	if err := router.GE.Run(":" + strconv.Itoa(config.CFG.Server.Port)); err != nil {
		zlog.Fatal("HTTP 服务启动失败", "err", err)
	}
}
