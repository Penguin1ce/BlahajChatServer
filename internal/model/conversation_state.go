package model

import "time"

// ConversationState 是某个用户对某个会话的个人状态。
//
// 它不再承担成员关系：
//   - C2C 成员来自 conversations.peer_key
//   - 群成员来自 group_info.members
//
// 这里仅保存会话列表需要的个人视角数据，比如未读、已读位置、置顶和免打扰。
type ConversationState struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	UID    uint64 `gorm:"not null;uniqueIndex:uk_conversation_state,priority:1;index" json:"uid"`
	ConvID string `gorm:"size:36;not null;uniqueIndex:uk_conversation_state,priority:2;index" json:"conv_id"`

	LastReadMsgID string `gorm:"size:36;not null;default:''" json:"last_read_msg_id"`
	Unread        uint32 `gorm:"not null;default:0" json:"unread"`
	Pinned        bool   `gorm:"not null;default:false" json:"pinned"`
	Muted         bool   `gorm:"not null;default:false" json:"muted"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ConversationState) TableName() string { return "conversation_state" }
