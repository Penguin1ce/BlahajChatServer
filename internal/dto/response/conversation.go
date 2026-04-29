package response

import "encoding/json"

type Conversation struct {
	ConvID    string `json:"conv_id"`
	Type      string `json:"type"`
	PeerKey   string `json:"peer_key,omitempty"`
	Name      string `json:"name,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	OwnerID   uint64 `json:"owner_id,omitempty"`
	LastMsgID string `json:"last_msg_id,omitempty"`
	LastMsgAt int64  `json:"last_msg_at"`
}

type MessageResp struct {
	ID        uint64          `json:"id"`
	MsgID     string          `json:"msg_id"`
	ConvID    string          `json:"conv_id"`
	FromUID   uint64          `json:"from_uid"`
	Type      string          `json:"type"`
	Content   json.RawMessage `json:"content"`
	ReplyTo   *string         `json:"reply_to,omitempty"`
	Status    uint8           `json:"status"`
	CreatedAt int64           `json:"created_at"`
}

type MessageListResp struct {
	Items        []MessageResp `json:"items"`
	NextBeforeID uint64        `json:"next_before_id,omitempty"`
	HasMore      bool          `json:"has_more"`
}
