package model

import "time"

// 会话类型枚举。持久化存字符串，方便肉眼排查。
const (
	ConvTypeC2C   = "c2c"   // 单聊
	ConvTypeGroup = "group" // 群聊
)

// Conversation 代表一个"会话"本身，存储所有成员看到都一样的客观事实。
//
// 它只记录会话自身的属性（名字、头像、最后一条消息等），
// 不包含任何"某个用户对这个会话的态度"（未读数、置顶、免打扰等）——
// 那些属于 UserConv。
//
// 典型使用场景：
//   - Hub 发群消息前，service 先查 UserConv 拿到成员 uid 列表
//   - 客户端拉会话列表，JOIN UserConv 取出本会话卡片信息
type Conversation struct {
	// ConvId 会话主键。UUID 字符串（36 位），由 service 层建会话时生成。
	ConvId string `gorm:"primaryKey;size:36" json:"conv_id"`

	// Type 会话类型，取值见 ConvType* 常量。建索引便于按类型筛选（如"只看群聊"）。
	Type string `gorm:"size:16;not null;index" json:"type"`

	// PeerKey 单聊去重键。格式 "minUid_maxUid"（两端 uid 排序后拼接），配合唯一索引
	// 保证同一对用户只会有一个单聊会话。群聊时必须为 NULL（而非 ""），
	// 因为 MySQL 唯一索引允许多行 NULL 但不允许多行空串。
	PeerKey *string `gorm:"size:64;uniqueIndex:uk_peer_key" json:"peer_key,omitempty"`

	// Name 群名称。单聊留空，UI 侧直接显示对方昵称。
	Name string `gorm:"size:64;not null;default:''" json:"name,omitempty"`

	// Avatar 群头像 URL。单聊留空。
	Avatar string `gorm:"size:255;not null;default:''" json:"avatar,omitempty"`

	// OwnerID 群主用户 ID。单聊固定为 0。加索引支持"我创建的群"查询。
	OwnerID uint64 `gorm:"index;not null;default:0" json:"owner_id,omitempty"`

	// LastMsgID 会话最后一条消息的 MsgID（Message.MsgID），用于会话列表预览。
	// 每次有新消息时由 service 层更新。
	LastMsgID string `gorm:"size:36;not null;default:''" json:"last_msg_id,omitempty"`

	// LastMsgAt 会话最后活跃时间，用于会话列表按活跃度排序。
	// 建索引使 ORDER BY last_msg_at DESC 能直接走索引。
	LastMsgAt time.Time `gorm:"index" json:"last_msg_at"`

	// CreatedAt 会话创建时间，由 GORM 自动填充。
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt 会话信息更新时间（改名、换头像等），由 GORM 自动维护。
	UpdatedAt time.Time `json:"updated_at"`
}

func (Conversation) TableName() string { return "conversations" }
