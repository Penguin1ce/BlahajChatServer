package model

import "time"

// FriendApply 表示一条好友申请。
// 申请是有方向的：FromUID 发给 ToUID。接收方处理后更新 Status。
type FriendApply struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	FromUID uint64 `gorm:"not null;index:idx_friend_apply_from" json:"from_uid"`
	ToUID   uint64 `gorm:"not null;index:idx_friend_apply_to_status,priority:1" json:"to_uid"`

	Status string `gorm:"size:16;not null;default:pending;index:idx_friend_apply_to_status,priority:2" json:"status"`
	Reason string `gorm:"size:255;not null;default:''" json:"reason"`

	HandledAt *time.Time `json:"handled_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (FriendApply) TableName() string { return "friend_apply" }
