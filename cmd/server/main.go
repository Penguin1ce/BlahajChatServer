package main

import (
	"strconv"

	"BlahajChatServer/config"
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
	router.Init()
	if err := router.GE.Run(":" + strconv.Itoa(config.CFG.Server.Port)); err != nil {
		zlog.Fatal("HTTP 服务启动失败", "err", err)
	}
}
