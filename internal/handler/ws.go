package handler

import (
	"BlahajChatServer/pkg/consts"

	"github.com/gin-gonic/gin"
)

// WebsocketLogin 登录后对 WebSocket 升级
func WebsocketLogin(c *gin.Context) {
	userID := c.GetUint(consts.CtxUserID)

	_ = userID
}
