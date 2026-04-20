package chathelper

import (
	"net/http"

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
