package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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

	router.Init()
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(config.CFG.Server.Port),
		Handler: router.GE,
	}

	// 监听中断信号，收到后退出 Notify 注册的 ctx。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// HTTP 服务在独立 goroutine 里跑，主 goroutine 阻塞等信号。
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zlog.Fatal("HTTP 服务启动失败", "err", err)
		}
	}()
	zlog.Info("HTTP 服务已启动", "addr", srv.Addr)

	<-ctx.Done()
	stop() // 恢复默认信号处理：退出过程中再按一次 Ctrl-C 可强杀
	zlog.Info("收到退出信号，开始优雅关闭")

	// 退出顺序：先停 HTTP（不再接新请求、等存量请求收尾），再关 Hub/Kafka 冲刷缓冲。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		zlog.Warn("HTTP 服务优雅关闭超时", "err", err)
	}
	if err := ws.CloseHub(); err != nil {
		zlog.Warn("WS Hub 关闭失败", "err", err)
	}
	zlog.Info("已优雅退出")
}
