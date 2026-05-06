package bus

import "context"

// ChatEvent 是一条"会话新增消息"扇出事件。
// Targets 在 publish 前由调用方算好，避免下游再查成员列表。
// Frame 是已经序列化好的 OpMsg 帧，Hub 拿到后可以直接 fan-out。
type ChatEvent struct {
	MsgID   string
	ConvID  string
	Targets []uint64
	Frame   []byte
}

// EventHandler 让 KafkaBus 消费到事件后通过函数闭包反向接到 ws.Hub，
// 避免 bus 包 import internal/ws 造成循环引用。
type EventHandler func(ctx context.Context, e ChatEvent) error
