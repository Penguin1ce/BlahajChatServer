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

// FanoutBus 抽象消息扇出通道。
// 当前实现固定走 KafkaBus，调用方只需要发布 ChatEvent。
type FanoutBus interface {
	Publish(ctx context.Context, e ChatEvent) error
	Close() error
}

// PublishFunc 让 KafkaBus 消费到事件后通过函数闭包反向接到 ws.Hub，
// 避免 bus 包 import internal/ws 造成循环引用。
type PublishFunc func(ctx context.Context, e ChatEvent) error

// Global 由 main 在启动时赋值，handler / service 层共享一个实例。
var Global FanoutBus

func CloseGlobal() error {
	if Global == nil {
		return nil
	}
	return Global.Close()
}
