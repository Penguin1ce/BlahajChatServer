# 本项目的 Kafka 用法

> 配套阅读：`docs/kafka基础知识.md`（基础概念）、`internal/bus/kafka.go`（实现）。
> 这份文档说明：本项目用 Kafka 解决什么问题、用了哪些特性、为什么这样选、哪些没做。

## 一、为什么要上 Kafka

**问题背景**：WS 消息扇出原本是 `service.HandleSend → hub.Broadcast`，`Hub` 是进程内的 map，**只能给本实例的 WS 连接发消息**。一旦多实例部署，目标用户在另一台机器就收不到。

**Kafka 在本项目里解决的事**：
1. **跨实例消息扇出**：所有实例都能拿到所有事件，自己决定要不要 fan-out 给本地连接
2. **未来给异步消费者留口子**：推送、搜索、审计、风控等下游订阅同一个 topic，互不影响

## 二、整体链路

```
WS 客户端
   │ 发送一条消息
   ▼
internal/ws/client.go: dispatch OpSend
   │
   ▼
service.HandleSend
   ├─ Redis SETNX 幂等（按 client_msg_id）
   ├─ MySQL 事务：messages + conversation.last_msg + user_conv.unread
   └─ 返回 MsgData
   │
   ▼
internal/ws/client.go: bus.Global.Publish(ChatEvent)
   │
   ▼
internal/bus/kafka.go: KafkaBus.Publish
   │
   ▼  Key=ConvID, Value=JSON(ChatEvent), acks=all
Kafka topic: chat.events
   │
   ▼  每实例独立 group：blahaj-ws-${HOSTNAME}
KafkaBus consumer goroutine（每实例一份，全量消费）
   │
   ▼
hub.Broadcast(Targets, Frame)
   │
   ▼
本地在线 WS 连接（不在线的目标自动 drop）
```

## 三、用到的 Kafka 特性

### 1. Partition Key = ConvID

**位置**：`internal/bus/kafka.go` 的 `Publish`

```go
return k.writer.WriteMessages(ctx, kafka.Message{
    Key:   []byte(e.ConvID),
    Value: payload,
})
```

**目的**：同一会话内的消息按 hash 落到同一个 partition，partition 内严格有序 → consumer 按 offset 消费 → 用户感知到的消息时序和发送时序一致。

**为什么不用 UserID 做 Key**：UserID 做 Key 会让"同一用户在多个会话里的消息"挤进同一 partition，浪费且没意义；会话内顺序才是用户能感知的"消息时序"。

### 2. `RequiredAcks = RequireAll`

**位置**：`Writer` 构造

```go
RequiredAcks: kafka.RequireAll,
```

**目的**：Producer 等所有 ISR 副本都落盘才返回成功。单 broker 故障也不丢消息。

### 3. `Balancer = &kafka.Hash{}`

**位置**：`Writer` 构造

**目的**：明确按 Key 做 hash 选 partition；不指定的话 kafka-go 默认用 `round-robin`（轮询选 partition），同 Key 也会乱跑——顺序保证就破了。

### 4. 每实例独立 Consumer Group

**位置**：`KafkaConfig.GroupID` 默认 `blahaj-ws-${HOSTNAME}`

```go
if cfg.GroupID == "" {
    host, _ := os.Hostname()
    cfg.GroupID = "blahaj-ws-" + host
}
```

**目的**：多实例部署时，**每个实例都消费全量消息**，由实例自己判断本地 Hub 有没有目标连接。

**对比方案（共享 Group）的问题**：
- 共享 Group 时 Kafka 会在实例间分配 partition
- 假设用户 A 的 WS 连接在 instance-1，但承载 A 那条会话 partition 的 consumer 是 instance-2 → instance-2 收到消息时本地 Hub 没有 A → 消息丢失
- WS 是有状态的（连接粘附在某实例），所以**不能用 Kafka 自带的负载均衡**

### 5. 手动 commit + fan-out 后才 commit

**位置**：`KafkaBus.Run`

```go
if err := k.onEvent(ctx, e); err != nil {
    zlog.Warnf(...)
}
if err := k.reader.CommitMessages(ctx, m); err != nil {
    zlog.Warnf(...)
}
```

**目的**：crash 重启后从上次 commit 的 offset 续读，不丢消息。如果用 auto-commit，offset 可能在 fan-out 之前就提交了，崩溃就漏发。

### 6. 坏消息跳过（poison pill 防御）

**位置**：JSON 反序列化失败时仍然 `CommitMessages`

```go
if err := json.Unmarshal(m.Value, &e); err != nil {
    zlog.Errorf(...)
    _ = k.reader.CommitMessages(ctx, m)
    continue
}
```

**目的**：避免一条格式错的消息无限卡死整个 partition 的消费。

### 7. `StartOffset = LastOffset`

**位置**：`Reader` 构造

**目的**：新 group 第一次启动时只消费"加入之后"的新消息，不重放历史。已有 group 还是从 committed offset 继续，互不冲突。

### 8. KRaft 单 broker（运维侧）

**位置**：`docker-compose.yml` 的 kafka 服务

**目的**：本地开发用 `apache/kafka:3.8.0` 单节点 KRaft 模式，不依赖 ZooKeeper，启动快、配置简单。

## 四、投递语义：at-least-once

**保证什么**：
- `service.HandleSend` 落库成功 → bus.Publish → KafkaBus 写入成功 → 一定会送达 KafkaBus consumer → 一定调用本地 fan-out

**可能重复的场景**：
- 实例重启：从 committed offset 重读 → 部分事件被本实例处理两次
- KafkaBus.Publish 网络抖动重试 → 同一事件可能被写入 Kafka 两次

**为什么用户感知不到重复**：
1. **Service 层幂等**：`service.HandleSend` 用 `client_msg_id + Redis SETNX` 在落库前就拦截了重复发送
2. **客户端幂等**：前端按 `msg_id` 去重，一个 msg_id 只渲染一次

链路上层层做幂等，所以 Kafka 这里允许 at-least-once，不必上 exactly-once。

## 五、有意没做的事

| 特性                       | 不做的原因                                                                                                                                                                                                             |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Outbox 模式**            | 落库 + Kafka 之间确实有不一致窗口（事务外 publish）。但本项目 best-effort 在线推 + 离线靠 HTTP 拉历史，能容忍"在线推丢失"。Outbox 引入一张表 + worker + `SELECT FOR UPDATE SKIP LOCKED` + 重试，对实习项目复杂度不划算 |
| **事务 Producer**          | 只有要 exactly-once 才用得上                                                                                                                                                                                           |
| **Schema Registry / Avro** | 当前事件 schema 简单（4 个字段），JSON 够用。规模上去再考虑 protobuf                                                                                                                                                   |
| **多副本 / 多 Broker**     | docker-compose 单 broker，replication factor = 1。生产部署再加                                                                                                                                                         |
| **死信队列（DLQ）**        | 当前坏消息直接 skip + 日志，量级不需要专门 DLQ                                                                                                                                                                         |
| **消费端去重**             | 客户端按 msg_id 幂等，consumer 端不再重复造轮子                                                                                                                                                                        |

## 六、面试可以聊的设计点

1. **为什么 Key 用 ConvID 不用 UserID** —— 见第 3 节第 1 条
2. **为什么每实例独立 Group 不共享 Group** —— 见第 3 节第 4 条
3. **at-least-once 怎么不让用户感知重复** —— 见第 4 节
4. **为什么没做 Outbox** —— 见第 5 节
5. **顺序保证的完整链条** —— 同 Key（ConvID）+ Hash balancer + partition 内有序 + 单 consumer goroutine 串行处理 + 客户端按 msg_id 渲染

## 七、相关代码索引

| 关注点               | 文件                                      |
| -------------------- | ----------------------------------------- |
| 事件结构 / 接口定义  | `internal/bus/bus.go`                     |
| Kafka 实现           | `internal/bus/kafka.go`                   |
| 配置定义             | `config/config.go` 的 `Kafka`             |
| 配置示例             | `config/config.example.toml` 的 `[kafka]` |
| 启动装配             | `cmd/server/main.go`                      |
| 调用方               | `internal/ws/client.go` 的 OpSend 分支    |
| docker-compose 服务  | `docker-compose.yml` 的 `kafka`           |
