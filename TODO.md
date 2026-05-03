# TODO

## 当前收口：Kafka 扇出工程化

当前代码已经接入了 `FanoutBus` 和 `KafkaBus`，下一步目标不是继续堆功能，而是把这条链路收成“可跑、可关、可解释”的状态。

### 已完成

- [x] 新增 `internal/bus` 包
- [x] 定义 `ChatEvent`
- [x] 定义 `FanoutBus`
- [x] 实现 `KafkaBus`
- [x] `client.go` 从直接调用 Hub 改为 `bus.Global.Publish`
- [x] `main.go` 固定装配 `KafkaBus`
- [x] `bus` 包不 import `internal/ws`，避免 import cycle

### 待验证

- [ ] 本地消息能写入 Kafka 并由 consumer 扇出
- [x] `go build ./...`
- [x] `go test ./...`
- [ ] `docker compose up -d --build`

### 已知边界

- 当前 Kafka 只负责在线消息扇出事件，不是消息事实源
- MySQL `messages` 表仍是消息事实源
- Kafka publish 失败时，消息已经落库，客户端可以通过历史消息接口补偿
- 暂不做 Outbox，等确实需要严格事件不丢时再加

## P0：IM 主功能闭环

### 1. 消息已读闭环

- [ ] 实现 WS `OpRead`
- [ ] service 层新增 `HandleRead(uid, data)`
- [ ] 复用 `dao.UpdateLastRead(uid, convID, msgID)`
- [ ] 读到消息后清空当前用户该会话 `unread`
- [ ] 可选：广播 `OpNotify` 给会话其他在线端
- [ ] ws_tester 增加已读上报按钮

### 2. 会话列表接口

- [ ] 新增 `GET /api/conversations`
- [ ] 返回当前用户加入的会话列表
- [ ] 包含会话基础信息：`conv_id`、`type`、`name`、`avatar`、`last_msg_id`、`last_msg_at`
- [ ] 包含个人状态：`unread`、`pinned`、`muted`、`last_read_msg_id`
- [ ] 按 `pinned DESC, last_msg_at DESC` 排序
- [ ] ws_tester 增加拉会话列表按钮

### 3. 端到端验证清单

- [ ] A/B 两个账号登录
- [ ] A 创建或获取和 B 的 C2C 会话
- [ ] A/B 同时连接 WebSocket
- [ ] A 发消息后收到 `ackok`
- [ ] B 在线收到 `msg`
- [ ] A 的另一个页面也能收到 `msg`
- [ ] 重发同一个 `client_msg_id` 不重复落库
- [ ] `GET /api/conversations/:id/messages` 能拉到历史消息
- [ ] 已读上报后 `user_conv.unread` 清零

## P1：简历加分项

- [ ] `messages` 增加 `(conv_id, id)` 复合索引
- [ ] 实现 WS `OpRecall`
- [ ] WebSocket 单连接发送频率限制
- [ ] handler/service 单元测试补关键路径
- [ ] README 增加接口列表、WebSocket 协议说明和简化架构图

## P2：远期演进

### Outbox 可靠事件投递

当前不实现。只有 Kafka 需要承担可靠事件投递时再考虑。

- [ ] 新增 `chat_outbox` 表
- [ ] `HandleSend` 事务里同时写 outbox
- [ ] 后台 worker 扫描 pending 事件并投递 Kafka
- [ ] 成功标记 `sent`，失败增加 `retry_count` 和 `last_error`
