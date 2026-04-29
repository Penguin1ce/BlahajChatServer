# WebSocket 发消息后端链路

本文记录用户从 WebSocket 发送消息后，当前后端的处理逻辑和函数调用顺序。

## 总览

```text
Client.readPump
→ Client.dispatch
→ service.HandleSend
→ dao.IsMember
→ dao.CreateMessageTx
→ dao.UpdateLastMsgTx
→ dao.IncrUnreadExceptTx
→ Client.sendFrame(OpAckOK)
→ dao.ListMembers
→ Hub.Broadcast
→ Hub.Run
→ Hub.SendToUser
→ Client.writePump
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

5. `dispatch` 调用 service：

```go
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

此时返回 `created=false`，后续不会重复广播 `OpMsg`。

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
- 给除发送者外的其他成员 `user_conv.unread + 1`

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

14. 如果 `created=false`，说明这是重复发送，只回 `ackok`，流程结束。

15. 如果是新消息，`dispatch` 查询会话所有成员：

```go
members, err := dao.ListMembers(ctx, d.ConvID)
```

16. `dispatch` 把 `MsgData` 包成 `OpMsg` 帧：

```go
data, err := marshalFrame(OpMsg, 0, msg)
```

17. `dispatch` 交给 Hub 广播：

```go
c.hub.Broadcast(&Envelope{
    Targets: members,
    Data:    data,
})
```

18. `Hub.Broadcast` 把广播任务塞进 `broadcast` channel。

19. `Hub.Run` 收到 `Envelope` 后，对每个目标用户调用：

```go
h.SendToUser(uid, env.Data)
```

20. `Hub.SendToUser` 找到这个用户所有在线连接：

```go
set := h.clients[uid]
```

然后把消息塞进每个连接的 `send` channel：

```go
c.send <- data
```

21. 每个目标连接自己的 `Client.writePump` 从 `send` channel 取出数据，真正写回 WebSocket：

```go
c.conn.WriteMessage(websocket.TextMessage, data)
```

## 最终效果

- 发送者当前连接收到 `ackok`
- 会话内所有在线成员收到 `msg`
- 发送者其他端也收到 `msg`
- 当前发送连接也会收到 `msg`，客户端需要用 `msg_id` 或 `client_msg_id` 做本地去重

