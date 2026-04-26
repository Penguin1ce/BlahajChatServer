package ws

import (
	"encoding/json"
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
		c.dispatch(payload)
	}
}

// dispatch 解 Frame 并按 Op 分发。第一阶段只跑通协议骨架，
// send 暂时回写死的 ackok，真正落库 / fan-out 等第二阶段接 service 层。
func (c *Client) dispatch(payload []byte) {
	var frame Frame
	if err := json.Unmarshal(payload, &frame); err != nil {
		zlog.Warnf("WS 帧解析失败 uid=%d conn=%s err=%s", c.userID, c.connID, err.Error())
		c.sendFrame(OpError, 0, ErrorData{Code: "bad_frame", Message: err.Error()})
		return
	}
	//
	switch frame.Op {
	case OpPing:
		// 收到业务心跳：原样回 pong，seq 透传，客户端用它算 RTT
		c.sendFrame(OpPong, frame.Seq, nil)

	case OpSend:
		// 收到客户端发的消息帧。
		// 第一阶段：只校验 payload 格式 + 回写死的 ackok，验证协议链路（read→dispatch→write）通畅。
		// 第二阶段：接 service.HandleSend —— Redis 幂等(client_msg_id) → 成员资格校验
		//          → 生成 MsgID 落 messages 表 → 更新 user_conv.LastMsgAt/unread → fan-out 给其它在线端。
		var d SendData
		if err := json.Unmarshal(frame.Data, &d); err != nil {
			c.sendFrame(OpError, frame.Seq, ErrorData{Code: "bad_data", Message: err.Error()})
			return
		}

		// 收到 send 帧的观察日志：只打关键字段，避免把消息正文刷进日志
		zlog.Debug("WS 收到 send 帧",
			"uid", c.userID,
			"conn", c.connID,
			"client_msg_id", d.ClientMsgID,
			"conv_id", d.ConvID,
			"type", d.Type,
		)
		c.sendFrame(OpAckOK, frame.Seq, AckOKData{
			MsgID:       uuid.NewString(), // TODO 第二阶段改为 service 层生成的全局 MsgID
			ClientMsgID: d.ClientMsgID,    // 原样回带，客户端用它匹配本地"发送中"草稿
			ConvID:      d.ConvID,
			Ts:          time.Now().UnixMilli(), // 服务端权威时间戳，不信任客户端时钟
		})

	default:
		// 未识别的 op：可能是客户端版本超前 / 拼写错。回 error 不断连，让客户端自己处理
		c.sendFrame(OpError, frame.Seq, ErrorData{Code: "unknown_op", Message: string(frame.Op)})
	}
}

// sendFrame 序列化 Frame 并塞进 send 通道，由 writePump 串行写出。
// closed 已置位 / 缓冲已满都不阻塞，避免回压拖死 dispatch。
func (c *Client) sendFrame(op Op, seq uint64, payload any) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			zlog.Errorf("帧 payload 序列化失败 op=%s err=%s", op, err.Error())
			return
		}
		raw = b
	}
	data, err := json.Marshal(Frame{Op: op, Seq: seq, Data: raw})
	if err != nil {
		zlog.Errorf("帧序列化失败 op=%s err=%s", op, err.Error())
		return
	}
	if c.closed.Load() {
		return
	}
	defer func() {
		// close() 与本写入之间存在窄窗口竞态，兜底防 panic
		_ = recover()
	}()
	select {
	case c.send <- data:
	default:
		zlog.Warnf("WS send 缓冲满 uid=%d conn=%s", c.userID, c.connID)
		go c.hub.Unregister(c)
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
