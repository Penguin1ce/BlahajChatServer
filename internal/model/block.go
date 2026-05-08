package model

import "time"

// Block 表示单向拉黑关系。
// UID 拉黑 BlockedUID；A 拉黑 B 不代表 B 也拉黑 A。
type Block struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UID        uint64    `gorm:"not null;uniqueIndex:uk_block_pair,priority:1;index" json:"uid"`
	BlockedUID uint64    `gorm:"not null;uniqueIndex:uk_block_pair,priority:2;index" json:"blocked_uid"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Block) TableName() string { return "blocks" }
