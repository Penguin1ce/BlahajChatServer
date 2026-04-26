package ws

import "encoding/json"

type Op string

const (
	// ---- 上行（客户端 → 服务端）----
	OpSend   Op = "send"   // 发消息，data: SendData
	OpAck    Op = "ack"    // 客户端确认收到某条 msg，data: AckData（应用层 ack，区别于 TCP / WS 控制帧）
	OpRead   Op = "read"   // 已读上报：我在该会话读到哪条了，data: ReadData
	OpRecall Op = "recall" // 撤回某条消息，data: {msg_id}
	OpTyping Op = "typing" // 输入中（不落库，直接 fan-out 给会话其它成员），data: {conv_id}
	OpPing   Op = "ping"   // 业务层心跳；gorilla 的控制帧 ping 已在 writePump 处理，这条仅用于上层探活

	// ---- 下行（服务端 → 客户端）----
	OpAckOK  Op = "ackok"  // 对 OpSend 的成功回执（已落库），data: AckOKData
	OpError  Op = "error"  // 上一帧处理失败，data: ErrorData
	OpMsg    Op = "msg"    // 推送一条新消息（发送者的其它端也会收到，用于多端同步），data: MsgData
	OpNotify Op = "notify" // 通用业务通知，目前主要承载已读回执：{conv_id, reader_uid, msg_id}
	OpKick   Op = "kick"   // 服务端主动踢下线（重复登录、被封禁等）
	OpPong   Op = "pong"   // OpPing 的回应
)

// Frame 是线上唯一格式
type Frame struct {
	Op   Op              `json:"op"`
	Seq  uint64          `json:"seq,omitempty"`
	Data json.RawMessage `json:"data,omitempty"` // ⭐ 延迟解析，按 op 分发
}

// ---- 上行 payload ----
type SendData struct {
	ClientMsgID string          `json:"client_msg_id" binding:"required,uuid"`
	ConvID      string          `json:"conv_id"       binding:"required"`
	Type        string          `json:"type"          binding:"required,oneof=text image file audio"`
	Content     json.RawMessage `json:"content"       binding:"required"`
	ReplyTo     *string         `json:"reply_to,omitempty"`
	Mentions    []uint64        `json:"mentions,omitempty"`
}

type AckData struct {
	MsgID string `json:"msg_id"`
}
type ReadData struct {
	ConvID string `json:"conv_id"`
	MsgID  string `json:"msg_id"`
}

// ---- 下行 payload ----
type AckOKData struct {
	MsgID       string `json:"msg_id"`
	ClientMsgID string `json:"client_msg_id"`
	ConvID      string `json:"conv_id"`
	Ts          int64  `json:"ts"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type MsgData struct {
	MsgID     string          `json:"msg_id"`
	ConvID    string          `json:"conv_id"`
	FromUID   uint64          `json:"from_uid"`
	Type      string          `json:"type"`
	Content   json.RawMessage `json:"content"`
	ReplyTo   *string         `json:"reply_to,omitempty"`
	Mentions  []uint64        `json:"mentions,omitempty"`
	CreatedAt int64           `json:"created_at"`
}
