package model

import "time"

// 消息类型枚举。与 ws.SendData.Type 对齐，存字符串方便扩展。
const (
	MsgTypeText  = "text"  // 纯文本
	MsgTypeImage = "image" // 图片
	MsgTypeFile  = "file"  // 文件
	MsgTypeAudio = "audio" // 语音
)

// 消息状态。
const (
	MsgStatusNormal   uint8 = 0 // 正常
	MsgStatusRecalled uint8 = 1 // 已撤回（保留行，Content 可能清空）
	MsgStatusDeleted  uint8 = 2 // 软删（某些运营场景）
)

// Message 会话内的一条消息。
//
// 双主键设计说明：
//   - ID 自增 BIGINT 作为真正主键，用于会话内排序、分页 cursor、
//     "查 msg_id > last_read_msg_id 的数量"这类查询（UUID 无法比较大小）。
//   - MsgID UUID 作为对外稳定 ID：客户端 ACK、撤回、引用消息（ReplyTo）全部用它。
//     Conversation.LastMsgID / UserConv.LastReadMsgID 存的也是它。
//
// Content 存的是已序列化的 JSON 字符串，不同 Type 的结构不同（文本 / 图片 url+尺寸 / 文件 meta 等），
// 由 ws 层按 Type 解析，DAO/DB 不关心内部结构。
type Message struct {
	// ID 自增主键，服务端内部用。会话内严格单调递增，充当稳定的排序键 / 分页游标。
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// MsgID 对外消息 ID，UUID。由 service 层在落库前生成，与 ClientMsgID 不同——
	// ClientMsgID 是客户端生成用于幂等去重的，MsgID 是服务端权威 ID。
	MsgID string `gorm:"size:36;not null;uniqueIndex:uk_msg_id" json:"msg_id"`

	// ConvID 消息所属会话。与 CreatedAt 组成联合索引 idx_conv_created，
	// 支持 "查某会话最近 N 条消息" 的分页查询走索引。
	ConvID string `gorm:"size:36;not null;index:idx_conv_created,priority:1" json:"conv_id"`

	// CreatedAt 消息创建时间（服务端时间，不信客户端）。
	// 位于 idx_conv_created 第二列，配合 ConvID 支持 ORDER BY created_at DESC 走索引。
	// 注意这里**没有**依赖 GORM 自动填充——它位于联合索引里需要显式 not null。
	CreatedAt time.Time `gorm:"not null;index:idx_conv_created,priority:2" json:"created_at"`

	// FromUID 发送者用户 ID。单独加索引支持 "查某人发的所有消息"（审计 / 删除）。
	FromUID uint64 `gorm:"not null;index" json:"from_uid"`

	// Type 消息类型，取值见 MsgType* 常量。
	Type string `gorm:"size:16;not null" json:"type"`

	// Content 消息正文的 JSON 字符串。text 类型就是 `{"text":"xxx"}`，
	// image / file / audio 带 url、size、duration 等字段。解析在 ws 层完成。
	Content string `gorm:"type:text;not null" json:"content"`

	// ReplyTo 被回复消息的 MsgID；普通消息为 NULL。
	// 用指针而非空串是为了语义清晰："没回复"与"回复了空" 要能区分。
	ReplyTo *string `gorm:"size:36" json:"reply_to,omitempty"`

	// Status 消息状态，取值见 MsgStatus* 常量。
	// 撤回不物理删除是为了保留顺序和引用链完整。
	Status uint8 `gorm:"not null;default:0" json:"status"`
}

func (Message) TableName() string { return "messages" }
