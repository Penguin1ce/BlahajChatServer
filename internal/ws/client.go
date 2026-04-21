package ws

import (
	"sync/atomic"
	"time"

	"BlahajChatServer/internal/zlog"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	sendBufSize    = 64
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1 << 20 // 1MB，单帧上限
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID uint64
	connID string
	send   chan []byte
	closed atomic.Bool
}

func NewClient(hub *Hub, conn *websocket.Conn, userID uint64) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		connID: uuid.NewString(),
		send:   make(chan []byte, sendBufSize),
	}
}

// Serve 在 goroutine 里启动读写泵。Upgrade 成功后调用即可。
func (c *Client) Serve() {
	go c.writePump()
	go c.readPump()
}

// close 关闭底层 conn 与 send 通道；多次调用安全。
func (c *Client) close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	close(c.send)
	_ = c.conn.Close()
}

// readPump 收帧 → 交给业务分发。退出即触发 Unregister。
func (c *Client) readPump() {
	defer c.hub.Unregister(c)

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zlog.Warnf("WS 非正常关闭 uid=%d conn=%s err=%s", c.userID, c.connID, err.Error())
			}
			return
		}
		// TODO 把 payload 反序列化成 Frame 后按 Op 分发到 service 层
		_ = payload
	}
}

// writePump 从 send 取数据写 conn，并按 pingPeriod 发 ping 保活。
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.hub.Unregister(c)
	}()

	for {
		select {
		case data, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// send 被 Hub 关闭，优雅挥手
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
