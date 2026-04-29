package ws

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/service"
	"BlahajChatServer/internal/zlog"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	sendBufSize    = 64
	writeWait      = 10 * time.Second
	sendHandleWait = 5 * time.Second
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

// dispatch 解 Frame 并按 Op 分发。
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
		// 收到 send 帧的ping
		zlog.Debug("WS 收到 ping 帧",
			"uid", c.userID,
			"conn", c.connID,
			"ping", "pong",
		)
		c.sendFrame(OpPong, frame.Seq, nil)

	case OpSend:
		// send 是发消息主链路：WS 层负责解协议帧、回 ack、扇出 msg；
		// 真正的幂等、成员校验、落库、未读更新都放在 service.HandleSend。
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

		ctx, cancel := context.WithTimeout(context.Background(), sendHandleWait)
		defer cancel()

		// created=false 表示命中了 client_msg_id 幂等：旧消息已经落过库。
		// 这种情况只需要给当前发送连接补 ack，不要再次广播 msg。
		msg, created, err := service.HandleSend(ctx, c.userID, d)
		if err != nil {
			zlog.Warnf("WS send 处理失败 uid=%d conn=%s conv=%s err=%s", c.userID, c.connID, d.ConvID, err.Error())
			c.sendFrame(OpError, frame.Seq, ErrorData{Code: "send_failed", Message: err.Error()})
			return
		}

		// ackok 是“发送请求已被服务端成功处理”的回执，只发给当前连接。
		// 客户端用 client_msg_id 把本地发送中的临时消息换成服务端正式 msg_id。
		c.sendFrame(OpAckOK, frame.Seq, AckOKData{
			MsgID:       msg.MsgID,
			ClientMsgID: d.ClientMsgID,
			ConvID:      d.ConvID,
			Ts:          msg.CreatedAt,
		})

		if !created {
			return
		}

		// OpMsg 是“会话新增消息”的业务事件，需要发给会话所有成员的所有在线端。
		// 这里包含发送者自己的其它端；当前连接也会收到一份，客户端用 msg_id/client_msg_id 去重。
		members, err := dao.ListMembers(ctx, d.ConvID)
		if err != nil {
			zlog.Warnf("WS send 成员列表查询失败 uid=%d conn=%s conv=%s err=%s", c.userID, c.connID, d.ConvID, err.Error())
			return
		}
		data, err := marshalFrame(OpMsg, 0, msg)
		if err != nil {
			zlog.Errorf("WS msg 帧序列化失败 uid=%d conn=%s msg=%s err=%s", c.userID, c.connID, msg.MsgID, err.Error())
			return
		}
		if !c.hub.Broadcast(&Envelope{Targets: members, Data: data}) {
			zlog.Warnf("WS msg 广播失败 uid=%d conn=%s msg=%s targets=%d", c.userID, c.connID, msg.MsgID, len(members))
		}

	default:
		// 未识别的 op：可能是客户端版本超前 / 拼写错。回 error 不断连，让客户端自己处理
		c.sendFrame(OpError, frame.Seq, ErrorData{Code: "unknown_op", Message: string(frame.Op)})
	}
}

// sendFrame 序列化 Frame 并塞进 send 通道，由 writePump 串行写出。
// closed 已置位 / 缓冲已满都不阻塞，避免回压拖死 dispatch。
func (c *Client) sendFrame(op Op, seq uint64, payload any) {
	data, err := marshalFrame(op, seq, payload)
	if err != nil {
		zlog.Errorf("帧序列化失败 op=%s err=%s", op, err.Error())
		return
	}
	c.sendRaw(data)
}

func marshalFrame(op Op, seq uint64, payload any) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return json.Marshal(Frame{Op: op, Seq: seq, Data: raw})
}

func (c *Client) sendRaw(data []byte) {
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
		// 缓冲满 = 这条连接已经不是"慢"而是"坏"了。继续给它喂数据只会污染整个 Hub 的派发链路，所以快速失败、断连、让客户端重连重新建立干净的会话，是更稳的策略。
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
