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

	// 创建client
	client := NewClient(GlobalHub, conn, userID)
	// 向Hub注册Client
	GlobalHub.Register(client)
	// 客户端client开始服务，启动读取和写入线程
	client.Serve()
	zlog.Infof("用户 %d 建立 WebSocket 成功 conn=%s", userID, client.connID)
}
