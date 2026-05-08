# 本项目的 Kafka 用法

> 配套阅读：`docs/kafka基础知识.md`（基础概念）、`internal/bus/kafka.go`（实现）。
> 这份文档说明：本项目用 Kafka 解决什么问题、用了哪些特性、为什么这样选、哪些没做。

## 一、为什么要上 Kafka

**问题背景**：如果 WS 消息扇出只在进程内调用 `Hub.Broadcast`，`Hub` 这个内存 map **只能给本实例的 WS 连接发消息**。一旦多实例部署，目标用户在另一台机器就收不到。

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
   ├─ MySQL 事务：messages + conversation.last_msg + conversation_state.unread
   └─ 返回 MsgData, created
   │
   ├─ 当前发送连接先收到 ackok（只表示服务端已处理/已落库）
   │
   ├─ created=false：重复 send，只补 ackok，流程结束
   │
   └─ created=true：继续发布新消息事件
   ▼
dao.ListMembers(conv_id)
   │
   ▼
marshalFrame(OpMsg, MsgData)
   │
   ▼
internal/ws/client.go: c.hub.Publish(ChatEvent)
   │  Hub.Publish 委托给它持有的 *KafkaBus
   ▼
internal/bus/kafka.go: KafkaBus.Publish
   │  Key=ConvID, Value=JSON(ChatEvent), RequiredAcks=all
   ▼
Kafka topic: 配置值（默认 chat.events）
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

`ChatEvent` 里带的是发送时刻预计算好的 `Targets` 和已经序列化好的 `OpMsg` Frame。Kafka 下游不再查会话成员，也不负责补离线消息；离线和补偿靠 MySQL 历史消息接口。

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

**目的**：Producer 等 ISR 副本确认后才返回成功，适合以后多 broker 部署时提高事件写入可靠性。当前 `docker-compose.yml` 是单 broker、replication factor = 1，所以 `RequireAll` 实际上只能等当前 broker 确认；broker 整体故障时服务会不可用，不能理解成单节点也能抗 broker 丢失。

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

**目的**：多实例部署时，**每个实例都消费全量消息**，由实例自己判断本地 Hub 有没有目标连接。`group_id` 留空会按机器名生成；如果手动配置，多个 WS 实例不要配置成同一个 group，否则就会变成 Kafka 的共享消费。

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

## 四、ack、fanout 与投递语义

### 1. `ackok` 的含义

`ackok` 是 `Client.dispatch` 在 `service.HandleSend` 成功返回后立刻发给当前发送连接的回执。它表示：

- 参数校验通过
- 发送者是会话成员
- 消息已经写入 MySQL `messages`
- `conversations.last_msg_id/last_msg_at` 和其他会话状态 `conversation_state.unread` 已经在同一个事务里更新

它不表示 Kafka publish 已经成功，也不表示其它在线端已经收到 `msg`。

### 2. Kafka fanout 的语义

`(*ws.Hub).Publish` 发生在落库和 `ackok` 之后，内部调用 Hub 持有的 `KafkaBus.Publish` 把事件写入 Kafka，再由每个实例的 `KafkaBus.Run` consumer 拉回并通过 `EventHandler` 回调进入 `Hub.HandleEvent → Hub.Broadcast`。

如果 Kafka publish 成功，consumer 侧是偏 at-least-once 的：fan-out 后才 commit offset，实例在 fan-out 后、commit 前崩溃，重启后可能再次处理同一条 `ChatEvent`。Kafka 事件重复时，服务端 consumer 当前不去重，客户端需要按 `msg_id` 去重。

如果 Kafka publish 失败，消息可能已经落库且发送端已经收到 `ackok`，但在线推送可能缺失。当前没有引入 Outbox，设计上接受这个 best-effort 在线推窗口；客户端后续通过历史消息接口补齐。

### 3. 幂等边界

- **重复 send 幂等**：`service.HandleSend` 用 `client_msg_id + Redis SETNX` 在落库前拦截重复发送；命中幂等时只补 `ackok`，不会再次发布 `ChatEvent`。
- **重复 fanout 幂等**：Kafka/consumer 层可能重复投递同一个 `msg_id` 的 `msg`，客户端按 `msg_id` 或 `client_msg_id` 合并，避免重复渲染。

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
