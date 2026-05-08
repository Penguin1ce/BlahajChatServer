# WebSocket 发消息后端链路

本文记录用户从 WebSocket 发送消息后，当前后端的处理逻辑和函数调用顺序。

当前版本已经把“消息扇出”从 `client.go` 里直接调用 `Hub.Broadcast`，改成了通过 `(*ws.Hub).Publish` 发布 `ChatEvent`。运行时固定使用 `KafkaBus`：先写入 Kafka，再由每个服务实例自己的 consumer 消费事件，最后只扇出到各自本机 `Hub` 上的在线连接。`Hub` 同时充当生产者（Publish）和消费者回调（HandleEvent），KafkaBus 由 Hub 持有，`bus` 包不再有全局变量。

## 总览

### 公共主链路

```text
Client.readPump
→ Client.dispatch
→ service.HandleSend
→ dao.IsMember（C2C 看 peer_key，群聊看 group_info.members）
→ dao.CreateMessageTx
→ dao.UpdateLastMsgTx
→ dao.IncrUnreadExceptTx
→ Client.sendFrame(OpAckOK)
→ dao.ListMembers（C2C 看 peer_key，群聊看 group_info.members）
→ marshalFrame(OpMsg)
→ (*ws.Hub).Publish(ChatEvent)
```

### KafkaBus 分支

```text
(*ws.Hub).Publish
→ KafkaBus.Publish
→ Kafka Writer 写入配置的 topic（默认 chat.events）
→ KafkaBus.Run consumer FetchMessage
→ 反序列化 ChatEvent
→ (*ws.Hub).HandleEvent  （EventHandler 回调）
→ Hub.Broadcast
→ Hub.Run
→ Hub.SendToUser
→ Client.writePump
→ Kafka consumer 手动 commit offset
```

## 启动时如何装配 Bus

`cmd/server/main.go` 启动时只调用 `ws.InitHub`，由它负责初始化 `Hub`、装配 `KafkaBus` 并把 `Hub.HandleEvent` 作为消费回调注入：

```go
if err := ws.InitHub(context.Background(), bus.KafkaConfig{
    Brokers: config.CFG.Kafka.Brokers,
    Topic:   config.CFG.Kafka.Topic,
    GroupID: config.CFG.Kafka.GroupID,
}); err != nil {
    zlog.Fatal("WS Hub 初始化失败", "err", err)
}
defer ws.CloseHub()
```

`ws` 包内部会调 `bus.InitKafka(ctx, cfg, h.HandleEvent)` 启动 Kafka consumer，并把返回的 `*KafkaBus` 存到 `Hub.bus` 字段。`bus` 包没有全局变量，"谁拥有 KafkaBus"和"谁负责本地扇出"都收敛在 ws 包内。

所以 `client.go` 不需要关心 Kafka 细节，只需要通过自己持有的 `Hub` 调用：

```go
c.hub.Publish(ctx, event)
```

## 详细流程

1. 用户通过 WebSocket 发来一帧消息：

```json
{
  "op": "send",
  "seq": 1,
  "data": {
    "client_msg_id": "xxx",
    "conv_id": "conv_1",
    "type": "text",
    "content": {"text": "你好"}
  }
}
```

2. `Client.readPump` 读取原始 WebSocket 消息：

```go
_, payload, err := c.conn.ReadMessage()
c.dispatch(payload)
```

3. `Client.dispatch` 解析 `Frame`，根据 `op` 进入 `OpSend` 分支。

4. `dispatch` 把 `frame.Data` 解析成 `SendData`：

```go
var d SendData
json.Unmarshal(frame.Data, &d)
```

5. `dispatch` 创建一个短超时 context，然后调用 service：

```go
ctx, cancel := context.WithTimeout(context.Background(), sendHandleWait)
defer cancel()

msg, created, err := service.HandleSend(ctx, c.userID, d)
```

这里的 `c.userID` 是当前 WebSocket 连接对应的登录用户，也就是发送者。

6. `service.HandleSend` 先校验消息参数：

```go
validateSendData(d)
```

主要检查 `client_msg_id`、`conv_id`、`type`、`content` 是否存在且合法。

7. `HandleSend` 生成服务端消息 ID：

```go
msgID := uuid.NewString()
```

8. `HandleSend` 使用 Redis `SETNX` 做 `client_msg_id` 幂等：

```go
redis.SetNXValueByKeyExpire(key, msgID, consts.ClientMsgIDIdemTTL)
```

如果 `client_msg_id` 已经处理过，会走旧消息分支：

```go
waitExistingMsgData(ctx, oldMsgID)
```

此时返回 `created=false`。后续只给当前连接补 `ackok`，不会再次发布 `ChatEvent`，也就不会重复广播 `OpMsg`。

9. 如果是新消息，`HandleSend` 检查发送者是不是会话成员：

```go
dao.IsMember(ctx, uid, d.ConvID)
```

不是成员则返回 `ErrNotMember`。

10. 检查通过后，`HandleSend` 构造 `model.Message`。

11. `HandleSend` 开启 MySQL 事务：

```go
dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    dao.CreateMessageTx(ctx, tx, msg)
    dao.UpdateLastMsgTx(ctx, tx, d.ConvID, msgID, now)
    dao.IncrUnreadExceptTx(ctx, tx, d.ConvID, uid)
})
```

事务里依次做：

- 插入 `messages`
- 更新 `conversations.last_msg_id` 和 `conversations.last_msg_at`
- 给除发送者外的其他会话状态 `conversation_state.unread + 1`

如果事务失败，会删除 Redis 幂等 key，避免 Redis 占坑但消息没有落库。

12. 事务成功后，`HandleSend` 把 `model.Message` 转成 `MsgData` 返回：

```go
messageToMsgData(msg, d.Mentions)
```

13. 回到 `Client.dispatch`，先给发送者当前连接回 `ackok`：

```go
c.sendFrame(OpAckOK, frame.Seq, AckOKData{
    MsgID:       msg.MsgID,
    ClientMsgID: d.ClientMsgID,
    ConvID:      d.ConvID,
    Ts:          msg.CreatedAt,
})
```

`ackok` 只代表“服务端已经处理并落库了这次发送请求”，只发给当前发送连接。它不代表 Kafka fanout 已经成功，也不代表其它在线端已经收到 `msg`。

14. 如果 `created=false`，说明这是重复发送，只回 `ackok`，流程结束。

15. 如果是新消息，`dispatch` 查询会话所有成员：

```go
members, err := dao.ListMembers(ctx, d.ConvID)
```

这里的 `members` 会被放进 `ChatEvent.Targets`。目标成员在 publish 前计算好，避免下游重新查成员列表造成不一致。

16. `dispatch` 把 `MsgData` 包成 `OpMsg` 帧：

```go
data, err := marshalFrame(OpMsg, 0, msg)
```

17. `dispatch` 发布扇出事件：

```go
err := c.hub.Publish(ctx, bus.ChatEvent{
    MsgID:   msg.MsgID,
    ConvID:  d.ConvID,
    Targets: members,
    Frame:   data,
})
```

`Hub.Publish` 内部委托给它持有的 `KafkaBus.Publish`。从这里开始，事件进入 KafkaBus 扇出流程。这里的 `Targets` 是发送时刻查出的会话成员快照，下游不再重新查成员列表。

## KafkaBus 扇出流程

`KafkaBus.Publish` 会把 `ChatEvent` 序列化成 JSON，然后写入 Kafka：

```go
payload, err := json.Marshal(e)
k.writer.WriteMessages(ctx, kafka.Message{
    Key:   []byte(e.ConvID),
    Value: payload,
})
```

Kafka message 的 key 使用 `ConvID`，writer 配置了 `kafka.Hash` balancer。这样同一个会话的事件会进入同一个分区，便于保持会话内顺序。

`KafkaBus.Run` 启动的 consumer goroutine 会持续拉取事件：

```go
m, err := k.reader.FetchMessage(ctx)
```

拉到事件后反序列化：

```go
var e ChatEvent
json.Unmarshal(m.Value, &e)
```

然后调用启动时注入的 `EventHandler` 回调，也就是 `(*ws.Hub).HandleEvent`，进入本机 Hub：

```go
k.onEvent(ctx, e) // 实际指向 Hub.HandleEvent → Hub.Broadcast
```

本地扇出完成后手动提交 offset：

```go
k.reader.CommitMessages(ctx, m)
```

每个服务实例都应该使用独立的 consumer group（默认 `blahaj-ws-${HOSTNAME}`）。这样所有实例都能消费同一条 `ChatEvent`，但每个实例只投递自己本机 `Hub` 上存在的在线连接。

所以 Kafka 当前承担的是“在线消息扇出事件通道”，不是消息事实源。消息事实源仍然是 MySQL 的 `messages` 表；离线用户和 Kafka publish 失败后的补偿都依赖历史消息接口。

## 最终效果

- 发送者当前连接收到 `ackok`
- 新消息会发布一条 `ChatEvent`
- 事件先进入 Kafka，再由各实例 consumer 拉回本机 Hub
- 会话内所有在线成员收到 `msg`
- 发送者其它端也收到 `msg`
- 当前发送连接也会收到 `msg`，客户端需要用 `msg_id` 或 `client_msg_id` 做本地去重

## 当前边界

- Kafka 只负责在线扇出，不负责消息落库
- `ackok` 表示落库成功，不等于 fanout 成功
- Kafka publish 失败时，消息可能已经落库；客户端可以通过历史消息接口补偿
- 当前暂未引入 Outbox，因此还不保证“消息落库成功后 Kafka 事件一定最终发出”
- `ChatEvent.Targets` 是发送时的成员快照，下游不再重新查询成员列表
