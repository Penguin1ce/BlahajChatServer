package model

import (
	"encoding/json"
	"time"
)

// GroupInfo 存群聊自身的信息，以及群成员快照。
//
// 成员关系的事实源放在 Members 里；conversation_state 只保留每个用户
// 对这个会话的个人状态（未读、已读、置顶、免打扰）。
type GroupInfo struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// ConvID 指向 conversations.conv_id。一个群对应一个 group conversation。
	ConvID string `gorm:"size:36;not null;uniqueIndex" json:"conv_id"`

	Name    string `gorm:"size:64;not null;default:''" json:"name"`
	Avatar  string `gorm:"size:255;not null;default:''" json:"avatar"`
	OwnerID uint64 `gorm:"not null;index" json:"owner_id"`

	// Members 存群成员 uid 数组，例如 [1,2,3]。
	// 成员变更时必须和 conversation_state 在同一个事务里同步更新。
	Members     json.RawMessage `gorm:"type:json;not null" json:"members"`
	MemberCount uint32          `gorm:"not null;default:0" json:"member_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (GroupInfo) TableName() string { return "group_info" }
