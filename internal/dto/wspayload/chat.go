package wspayload

import "encoding/json"

// SendData 是客户端 send 帧的业务 payload。
type SendData struct {
	ClientMsgID string          `json:"client_msg_id" binding:"required,uuid"`
	ConvID      string          `json:"conv_id"       binding:"required"`
	Type        string          `json:"type"          binding:"required,oneof=text image file audio"`
	Content     json.RawMessage `json:"content"       binding:"required"`
	ReplyTo     *string         `json:"reply_to,omitempty"`
	Mentions    []uint64        `json:"mentions,omitempty"`
}

// MsgData 是服务端下发 msg 帧的业务 payload。
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
