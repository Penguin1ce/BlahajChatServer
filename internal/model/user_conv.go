package model

import "time"

// UserConv 主观视角：某个用户和某个会话的关系 + 个人状态。
// 同时承担两个职责：
//  1. 成员关系：存在这一行 ⇒ 该用户在该会话内
//  2. 个人状态：未读数、置顶、免打扰、读到哪条
//
// 联合唯一 (uid, conv_id) 防止重复入会话；
// conv_id 单独索引给 "查会话有哪些成员" 用。
//
// 与 Conversation 的分工：
//   - Conversation：客观事实，所有成员看到都一样
//   - UserConv：主观视角，每人一行，互不影响
type UserConv struct {
	// ID 自增主键。单纯给 GORM/DB 用，业务层不关心。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// UID 用户 ID。联合唯一索引 uk_user_conv 的第一列，
	// 支持 "查我加入了哪些会话" 走索引前缀。
	UID uint64 `gorm:"not null;uniqueIndex:uk_user_conv,priority:1" json:"uid"`

	// ConvID 会话 ID（Conversation.ConvId）。
	// 同时挂两个索引：
	//   - uk_user_conv 联合唯一（第二列）：防止同一用户重复加入同一会话
	//   - 单列普通索引 idx_user_conv_conv_id：给 "查该会话所有成员" 用（服务端推消息 fan-out）
	ConvID string `gorm:"size:36;not null;uniqueIndex:uk_user_conv,priority:2;index" json:"conv_id"`

	// LastReadMsgID 本人已读到的最后一条消息的 MsgID。
	// 客户端上报"已读到哪"时更新，用于计算未读 / 显示已读位置。
	LastReadMsgID string `gorm:"size:36;not null;default:''" json:"last_read_msg_id"`

	// Unread 未读数。写放大维护：会话内每条新消息到来时给所有非发送者 +1，
	// 读到 LastReadMsgID 时清 0。好处是客户端拉会话列表零计算。
	Unread uint32 `gorm:"not null;default:0" json:"unread"`

	// Pinned 是否置顶。客户端会话列表排序：pinned DESC, last_msg_at DESC。
	Pinned bool `gorm:"not null;default:false" json:"pinned"`

	// Muted 是否免打扰。true 时仍然收消息但不弹通知、不加红点角标。
	Muted bool `gorm:"not null;default:false" json:"muted"`

	// CreatedAt 加入会话的时间，由 GORM 自动填充。可用于 "入群时间" 展示。
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt 个人状态最后变更时间，由 GORM 自动维护。
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserConv) TableName() string { return "user_conv" }
