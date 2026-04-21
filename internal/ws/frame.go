package ws

import "encoding/json"

type Op string

const (
	OpSend   Op = "send"
	OpAck    Op = "ack"
	OpRead   Op = "read"
	OpRecall Op = "recall"
	OpTyping Op = "typing"
	OpPing   Op = "ping"

	OpAckOK  Op = "ackok"
	OpError  Op = "error"
	OpMsg    Op = "msg"
	OpNotify Op = "notify"
	OpKick   Op = "kick"
	OpPong   Op = "pong"
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
