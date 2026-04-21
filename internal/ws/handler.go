package ws

import (
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/zlog"
	"BlahajChatServer/pkg/consts"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 开发阶段允许所有来源
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WSLoginHandler(c *gin.Context) {
	userID := c.GetUint64(consts.CtxUserID)
	if userID == 0 {
		response.Fail(c, http.StatusUnauthorized, consts.UserMessageError)
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 已自行写回 4xx，这里只记日志
		zlog.Error("WS upgrade 失败: " + err.Error())
		return
	}
	client := NewClient(GlobalHub, conn, userID)
	GlobalHub.Register(client)
	client.Serve()
	zlog.Infof("用户 %d 建立 WebSocket 成功 conn=%s", userID, client.connID)
}

func PingWSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	defer conn.Close()
	zlog.Info("ping ws success")
	for {
		// 读取客户端发送的消息
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			zlog.Info("连接断开: " + err.Error())
			break
		}

		zlog.Infof("收到来自 %s 的消息: %s", string(payload))

		// 4. 异步处理逻辑 (微服务核心) [cite: 4, 10]
		// 不要在这里直接写数据库！而是将消息封装后丢进 Kafka (Upstream Topic)
		// 简单测试可以先原样返回 (Echo)
		if err := conn.WriteMessage(messageType, payload); err != nil {
			zlog.Error("发送失败: " + err.Error())
			break
		}
	}
}
