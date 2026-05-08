package model

import "time"

// 好友关系状态。
const (
	FriendshipStatusNormal  = "normal"  // 正常好友关系
	FriendshipStatusDeleted = "deleted" // 已删除，保留历史行
)

// 好友申请状态。
const (
	FriendApplyStatusPending  = "pending"  // 待处理
	FriendApplyStatusAccepted = "accepted" // 已同意
	FriendApplyStatusRejected = "rejected" // 已拒绝
	FriendApplyStatusCanceled = "canceled" // 已撤回
)

// Friendship 表示两个用户之间已经建立的好友关系。
//
// 这是无方向关系：业务层写入前必须保证 UIDA < UIDB，
// 这样同一对用户只会有一行，方便去重和查询。
type Friendship struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// UIDA / UIDB 是排序后的好友双方 uid，联合唯一防止重复好友关系。
	UIDA uint64 `gorm:"not null;uniqueIndex:uk_friendship_pair,priority:1;index" json:"uid_a"`
	UIDB uint64 `gorm:"not null;uniqueIndex:uk_friendship_pair,priority:2;index" json:"uid_b"`

	// Status 只区分正常 / 已删除
	Status string `gorm:"size:16;not null;default:normal;index" json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Friendship) TableName() string { return "friendships" }
