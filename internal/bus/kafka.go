package bus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"BlahajChatServer/internal/zlog"
)

// KafkaConfig 由 main 装配，避免 bus 包反向依赖 config 包。
type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string // 留空时自动用 "blahaj-ws-${HOSTNAME}"
}

// KafkaBus 把 ChatEvent 走 Kafka 跨实例分发。
//
// Producer：Key=ConvID（同会话同分区，保证顺序）、acks=all。
// Consumer：每实例独立 GroupID，全量消费，本地 fan-out 后手动 commit。
//   - "本实例 publish 的事件本实例也会收到"——这是有意为之，
//     单条消息只会经过一个 fan-out 路径，行为统一。
//   - "consumer 端不去重"——客户端按 msg_id 幂等。
type KafkaBus struct {
	writer  *kafka.Writer
	reader  *kafka.Reader
	onEvent PublishFunc

	mu        sync.Mutex
	runOnce   sync.Once
	closeOnce sync.Once
	cancel    context.CancelFunc
	done      chan struct{}
	started   bool
	closed    bool
	closeErr  error
}

func InitKafka(parent context.Context, cfg KafkaConfig, onEvent PublishFunc) error {
	kbus, err := NewKafka(cfg, onEvent)
	if err != nil {
		return err
	}
	kbus.Run(parent)
	Global = kbus
	zlog.Info("使用 KafkaBus 作为消息扇出通道",
		"brokers", cfg.Brokers,
		"topic", cfg.Topic,
	)
	return nil
}

func NewKafka(cfg KafkaConfig, onEvent PublishFunc) (*KafkaBus, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka: brokers 为空")
	}
	if cfg.Topic == "" {
		return nil, errors.New("kafka: topic 为空")
	}
	if cfg.GroupID == "" {
		host, err := os.Hostname()
		if err != nil || host == "" {
			host = "unknown"
		}
		cfg.GroupID = "blahaj-ws-" + host
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{}, // 按 Key (ConvID) hash 分区
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		StartOffset: kafka.LastOffset, // 新 group 从最新开始，避免重启重放历史
		MinBytes:    1,
		MaxBytes:    10 << 20, // 10MB
	})
	return &KafkaBus{
		writer:  w,
		reader:  r,
		onEvent: onEvent,
	}, nil
}

// Run 启动 consumer goroutine。重复调用安全；调用方一般在 main 里调一次。
func (k *KafkaBus) Run(parent context.Context) {
	k.runOnce.Do(func() {
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithCancel(parent)

		k.mu.Lock()
		if k.closed {
			k.mu.Unlock()
			cancel()
			return
		}
		k.cancel = cancel
		k.done = make(chan struct{})
		k.started = true
		done := k.done
		k.mu.Unlock()

		go func() {
			defer close(done)
			for {
				m, err := k.reader.FetchMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
					zlog.Errorf("KafkaBus FetchMessage 失败 err=%s", err.Error())
					select {
					case <-ctx.Done():
						return
					case <-time.After(500 * time.Millisecond):
					}
					continue
				}
				var e ChatEvent
				if err := json.Unmarshal(m.Value, &e); err != nil {
					zlog.Errorf("KafkaBus 反序列化失败 offset=%d err=%s", m.Offset, err.Error())
					// 坏消息直接 commit 跳过，否则会无限卡住
					_ = k.reader.CommitMessages(ctx, m)
					continue
				}
				if err := k.onEvent(ctx, e); err != nil {
					zlog.Warnf("KafkaBus 本地扇出失败 msg=%s err=%s", e.MsgID, err.Error())
				}
				if err := k.reader.CommitMessages(ctx, m); err != nil {
					zlog.Warnf("KafkaBus commit 失败 offset=%d err=%s", m.Offset, err.Error())
				}
			}
		}()
	})
}

func (k *KafkaBus) Publish(ctx context.Context, e ChatEvent) error {
	k.mu.Lock()
	closed := k.closed
	k.mu.Unlock()
	if closed {
		return errors.New("kafka: bus 已关闭")
	}

	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return k.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(e.ConvID),
		Value: payload,
	})
}

func (k *KafkaBus) Close() error {
	k.closeOnce.Do(func() {
		k.mu.Lock()
		k.closed = true
		cancel := k.cancel
		done := k.done
		started := k.started
		k.mu.Unlock()

		if cancel != nil {
			cancel()
		}

		var firstErr error
		if err := k.writer.Close(); err != nil {
			firstErr = err
		}
		if err := k.reader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		if started && done != nil {
			<-done
		}
		k.closeErr = firstErr
	})
	return k.closeErr
}
