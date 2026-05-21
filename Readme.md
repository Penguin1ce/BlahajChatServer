## 当前数据模型

当前后端以 MySQL 作为消息和关系事实源，Redis 负责验证码、Refresh Token、Access Token 黑名单和发送幂等，Kafka 只负责在线消息 fanout。

核心表职责如下：

| 表 | 职责 |
| --- | --- |
| `users` | 用户账号、邮箱、昵称、头像、密码哈希 |
| `friendships` | 已建立好友关系，`uid_a + uid_b` 联合唯一，业务层保证 `uid_a < uid_b` |
| `friend_apply` | 好友申请，状态为 `pending / accepted / rejected / canceled` |
| `blocks` | 单向拉黑关系，`uid` 拉黑 `blocked_uid` |
| `conversations` | 会话公共信息，包含 `conv_id/type/peer_key/last_msg_id/last_msg_at` |
| `group_info` | 群聊资料和群成员快照，`members` 是 JSON uid 数组 |
| `conversation_state` | 当前用户对某个会话的个人状态：`last_read_msg_id/unread/pinned/muted` |
| `messages` | 消息事实源，保存每条消息正文和排序游标 |

成员关系来源按会话类型区分：

- C2C：从 `conversations.peer_key` 解析双方 uid。
- 群聊：从 `group_info.members` 反序列化成员 uid。
- `conversation_state` 不再表示成员关系，只表示用户视角下的会话状态。

## 注册接口

注册分两步：先获取邮箱验证码，再提交注册信息。

### 第一步：获取邮箱验证码

```
POST /auth/getcode
```

**请求体**

| 字段    | 类型   | 必填 | 说明     |
| ------- | ------ | ---- | -------- |
| `email` | string | 是   | 注册邮箱 |

```json
{
    "email": "email@icloud.com"
}
```

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": "发送成功,请前往邮箱查收"
}
```

**错误响应**

| HTTP 状态码 | 说明                         |
| ----------- | ---------------------------- |
| `429`       | 验证码申请过于频繁，稍后再试 |
| `502`       | 邮件发送失败                 |
| `500`       | 系统错误                     |

> 验证码有效期 **5 分钟**，过期后需重新获取。

---

### 第二步：提交注册

```
POST /auth/register
```

**请求体**

| 字段         | 类型   | 必填 | 约束         | 说明             |
| ------------ | ------ | ---- | ------------ | ---------------- |
| `email`      | string | 是   | 合法邮箱格式 | 注册邮箱         |
| `password`   | string | 是   | 6–64 位      | 登录密码         |
| `nickname`   | string | 否   | 最长 32 字   | 用户昵称         |
| `email_code` | string | 是   | 6 位数字     | 邮箱收到的验证码 |

```json
{
    "email": "email@icloud.com",
    "password": "TLS123",
    "nickname": "TLS测试用户",
    "email_code": "584445"
}
```

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "id": 1,
        "email": "email@icloud.com",
        "nickname": "TLS测试用户",
        "avatar_url": "https://images.cdn.org/img/index/sticker.webp",
        "created_at": "2026-04-20T12:00:00Z",
        "updated_at": "2026-04-20T12:00:00Z"
    }
}
```

**错误响应**

| HTTP 状态码 | 说明                                   |
| ----------- | -------------------------------------- |
| `400`       | 参数校验失败、验证码未发送或验证码错误 |
| `409`       | 该邮箱已被注册                         |
| `500`       | 系统错误                               |

### 业务流程

```
客户端
  │
  ├─ POST /auth/getcode ──► 生成 6 位验证码，写入 Redis（TTL 5min），发送邮件
  │
  └─ POST /auth/register
         │
         ├─ 查 Redis：key = "sendEmailCode:{email}"
         │     不存在 → 400（验证码未发送或已过期）
         │
         ├─ 对比验证码
         │     不一致 → 400（验证码错误）
         │
         ├─ 查数据库：邮箱是否已注册
         │     已存在 → 409
         │
         ├─ bcrypt 加密密码，写入 users 表
         │
         └─ 删除 Redis 验证码（防止重复使用）→ 200
```

## 登录接口

```
POST /auth/login
```

**请求体**

| 字段       | 类型   | 必填 | 说明     |
| ---------- | ------ | ---- | -------- |
| `email`    | string | 是   | 注册邮箱 |
| `password` | string | 是   | 登录密码 |

```json
{
    "email": "email@icloud.com",
    "password": "TLS123"
}
```

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "user": {
            "id": 1,
            "email": "email@icloud.com",
            "nickname": "TLS测试用户",
            "avatar_url": "https://images.cdn.org/img/index/sticker.webp",
            "created_at": "2026-04-20T12:00:00Z",
            "updated_at": "2026-04-20T12:00:00Z"
        },
        "token": {
            "access_token": "<jwt>",
            "refresh_token": "<64位hex>",
            "expires_in": 900
        }
    }
}
```

> `expires_in` 单位为秒，默认 900（15 分钟）。

**错误响应**

| HTTP 状态码 | 说明           |
| ----------- | -------------- |
| `400`       | 参数校验失败   |
| `401`       | 邮箱或密码错误 |
| `500`       | 系统错误       |

### 业务流程

```
POST /auth/login
  │
  ├─ 参数绑定失败 → 400
  │
  ├─ 查数据库：email 是否存在
  │     不存在 → 401
  │
  ├─ bcrypt 比对密码
  │     不匹配 → 401
  │
  ├─ 签发 Access Token（JWT HS256，TTL 15min，携带 userID/jti）
  ├─ 生成 Refresh Token（32字节随机 hex，TTL 30天）
  ├─ Redis 写入：refresh:{token} → userID
  ├─ Redis 写入：userSession:{userID} SADD token（多设备索引）
  │
  └─ 返回 user 信息 + token 对 → 200
```

### Token 使用说明

登录后客户端持有两个 token，分工如下：

|          | Access Token         | Refresh Token           |
| -------- | -------------------- | ----------------------- |
| 寿命     | 15 分钟              | 30 天                   |
| 存储位置 | 内存（不持久化）     | 持久存储（Keychain 等） |
| 用途     | 每次请求鉴权         | Access Token 过期后换新 |
| 存 Redis | 黑名单（注销时写入） | 主存储                  |

**后续请求携带方式：**

```
Authorization: Bearer <access_token>
```

或 WebSocket 握手：

```
ws://host/ws/wslogin?token=<access_token>
```

## HTTP 会话接口

这些接口都需要携带：

```
Authorization: Bearer <access_token>
```

### 创建或获取单聊会话

```
POST /api/conversations/c2c
```

**请求体**

| 字段       | 类型   | 必填 | 说明       |
| ---------- | ------ | ---- | ---------- |
| `peer_uid` | number | 是   | 对方用户 ID |

```json
{
    "peer_uid": 2
}
```

成功后返回会话基础信息，`conv_id` 后续用于发消息、拉历史和拉会话列表展示。

### 创建群聊

```
POST /api/conversations/group
```

创建群聊时，服务端会自动把当前登录用户加入成员列表并设置为群主。`member_uids` 里只需要传其它初始成员 uid。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 群名称，最长 64 字 |
| `avatar` | string | 否 | 群头像 URL |
| `member_uids` | number[] | 是 | 初始成员 uid 列表，不需要包含自己 |

```json
{
    "name": "测试群",
    "member_uids": [2, 3]
}
```

服务端会在同一个 MySQL 事务里写入：

1. `conversations`：`type = "group"`，保存群公共信息。
2. `group_info`：保存群主、成员 JSON 和成员数。
3. `conversation_state`：为每个群成员创建个人会话状态。

成功后返回会话基础信息。后续群聊发消息仍走同一套 WebSocket `send`，成员 fanout 会从 `group_info.members` 读取。

### 群成员列表

```
GET /api/conversations/:id/members
```

只有群成员可以查看。成功后返回成员用户信息和是否群主：

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "items": [
            {
                "uid": 1,
                "email": "owner@icloud.com",
                "nickname": "群主",
                "avatar_url": "https://images.cdn.org/img/index/sticker.webp",
                "owner": true
            }
        ]
    }
}
```

### 邀请群成员

```
PUT /api/conversations/:id/members
```

当前最小版本只允许群主邀请成员。服务端会锁定该群的 `group_info` 行，在同一个事务里更新 `members/member_count` 并补齐新成员的 `conversation_state`。

```json
{
    "member_uids": [4, 5]
}
```

成功后返回更新后的群成员列表。重复传已有成员不会报错，只会保持幂等。

### 退出群聊

```
DELETE /api/conversations/:id/members/me
```

普通成员退出时，服务端会在同一个事务里从 `group_info.members` 移除当前用户，并删除自己的 `conversation_state`。当前版本不支持群主直接退出，需要后续先实现转让群主或解散群。

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "ok": true
    }
}
```

### 拉取当前用户会话列表

```
GET /api/conversations
```

返回当前登录用户的会话列表。会话列表由 `conversations` 和 `conversation_state` 组合得到：

- `conversations` 提供会话公共信息：`conv_id`、`type`、`peer_key`、`name`、`avatar`、`owner_id`、`last_msg_id`、`last_msg_at`
- `conversation_state` 提供当前用户自己的状态：`last_read_msg_id`、`unread`、`pinned`、`muted`
- 排序规则：`pinned DESC, last_msg_at DESC`

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": [
        {
            "conv_id": "5c241bb7-224d-4bf7-8fe2-6e612fe6083e",
            "type": "c2c",
            "peer_key": "1_2",
            "last_msg_id": "f419b00a-7739-4cbf-ba81-a2d4b80f704b",
            "last_msg_at": 1777990262926,
            "last_read_msg_id": "",
            "unread": 2,
            "pinned": false,
            "muted": false
        }
    ]
}
```

> 这里的 `unread` 是当前登录用户视角下的未读数；同一个会话里，不同用户看到的 `unread`、`pinned`、`muted` 可以不同。

### 会话接口错误码

| HTTP 状态码 | 说明 |
| --- | --- |
| `400` | 参数错误，例如群名称为空、成员列表为空、C2C uid 不合法 |
| `403` | 不是会话成员、没有群主权限，或群主直接退出群聊 |
| `404` | 会话不存在，或群成员 uid 不存在 |
| `500` | 系统错误 |

### 拉取历史消息

```
GET /api/conversations/:id/messages?before_id=0&limit=20
```

| Query       | 类型   | 必填 | 说明                                          |
| ----------- | ------ | ---- | --------------------------------------------- |
| `before_id` | number | 否   | 游标；传 `0` 表示拉最新一页                    |
| `limit`     | number | 否   | 每页数量，默认 `20`，最大 `100`                |

服务端会先校验当前用户是否属于该会话；不是成员时返回 `403`。

## HTTP 好友接口

这些接口都需要携带：

```
Authorization: Bearer <access_token>
```

### 好友列表

```
GET /api/friends
```

返回当前用户的正常好友关系。好友关系是无方向的，服务端内部用排序后的 `uid_a + uid_b` 去重。

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "items": [
            {
                "uid": 2,
                "email": "friend@icloud.com",
                "nickname": "好友",
                "avatar_url": "https://images.cdn.org/img/index/sticker.webp"
            }
        ]
    }
}
```

### 发起好友申请

```
POST /api/friends/apply
```

`from_uid` 不从请求体读取，永远以后端 JWT 登录态为准。

**请求体**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `to_uid` | number | 是 | 要添加的用户 ID |
| `reason` | string | 否 | 申请说明 |

```json
{
    "to_uid": 2,
    "reason": "你好"
}
```

服务端会校验：

- 不能添加自己。
- `to_uid` 必须存在。
- 任意方向存在拉黑关系时不能申请。
- 已经是好友时不能重复申请。
- 两人之间已有 `pending` 申请时不能重复插入。

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "id": 1,
        "from_uid": 1,
        "to_uid": 2,
        "status": "pending",
        "reason": "你好",
        "created_at": "2026-05-08T12:00:00+08:00"
    }
}
```

### 查看收到的好友申请

```
GET /api/friends/applies
```

只返回当前用户收到的 `pending` 申请。

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "items": [
            {
                "id": 1,
                "from_uid": 1,
                "to_uid": 2,
                "status": "pending",
                "reason": "你好",
                "created_at": 1777990262926
            }
        ]
    }
}
```

### 同意好友申请

```
POST /api/friends/applies/:id/accept
```

只有申请接收方本人可以操作。服务端会在同一个 MySQL 事务里完成：

1. 把 `friend_apply.status` 从 `pending` 更新为 `accepted`。
2. 写入或恢复 `friendships`，如果旧关系是 `deleted` 会恢复为 `normal`。
3. 创建或获取双方 C2C 会话。
4. 确保双方都有对应的 `conversation_state`。

成功后返回 C2C 会话信息。

### 拒绝好友申请

```
POST /api/friends/applies/:id/reject
```

只有申请接收方本人可以操作。服务端只会把 `friend_apply.status` 从 `pending` 更新为 `rejected`。

**成功响应 `200`**

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "ok": true
    }
}
```

### 好友接口错误码

| HTTP 状态码 | 说明 |
| --- | --- |
| `400` | 参数错误，例如添加自己 |
| `403` | 没有权限，或存在拉黑关系 |
| `404` | 目标用户或好友申请不存在 |
| `409` | 已经是好友、已有 pending 申请，或申请已处理 |
| `500` | 系统错误 |

## WebSocket 接口

聊天链路走 WS 长连接，连接成功后通过 JSON Frame 收发业务消息。HTTP 仅承担登录、拉历史、上传等短连接场景。

### 建立连接

```
GET /ws/wslogin?token=<access_token>
```

握手前会走 `JWTAuth` 中间件验证 token，未通过返回 `401`。Upgrade 成功后，服务端会：

1. 创建 `Client`，分配一个 `connID`（uuid，用于日志定位）
2. 注册到 `Hub`（同一用户的多端连接共存）
3. 启动读 / 写两个 goroutine 处理收发

**错误响应（握手阶段）**

| HTTP 状态码 | 说明                                  |
| ----------- | ------------------------------------- |
| `401`       | token 缺失 / 非法 / 已过期 / 已被注销 |
| `400`       | Upgrade 失败（缺少 ws 协议头等）      |

### Frame 结构

所有 WS 业务帧统一格式：

```json
{
  "op": "send",
  "seq": 1,
  "data": { /* 按 op 不同结构不同 */ }
}
```

| 字段   | 类型   | 必填 | 说明                                                 |
| ------ | ------ | ---- | ---------------------------------------------------- |
| `op`   | string | 是   | 帧类型，见下方"Op 列表"                              |
| `seq`  | number | 否   | 客户端自增序号，服务端在响应里原样回带，便于配对回执 |
| `data` | object | 否   | 业务负载，结构由 `op` 决定                           |

### Op 列表

| 方向   | Op       | 状态     | data 结构                                                                      | 作用                           |
| ------ | -------- | -------- | ------------------------------------------------------------------------------ | ------------------------------ |
| ↑ 上行 | `ping`   | ✅ 已实现 | 无                                                                             | 业务心跳（探活 / 测 RTT）      |
| ↑ 上行 | `send`   | ✅ 已实现 | `{client_msg_id, conv_id, type, content, reply_to?, mentions?}`                | 发消息                         |
| ↑ 上行 | `ack`    | 🚧 规划中 | `{msg_id}`                                                                     | 客户端确认收到某条 `msg`       |
| ↑ 上行 | `read`   | ✅ 已实现 | `{conv_id, msg_id}`                                                            | 已读上报                       |
| ↑ 上行 | `recall` | 🚧 规划中 | `{msg_id}`                                                                     | 撤回                           |
| ↑ 上行 | `typing` | 🚧 规划中 | `{conv_id}`                                                                    | 输入中（不落库，直接 fan-out） |
| ↓ 下行 | `pong`   | ✅ 已实现 | 无                                                                             | `ping` 的回应                  |
| ↓ 下行 | `ackok`  | ✅ 已实现 | `{msg_id, client_msg_id, conv_id, ts}`                                         | `send` 成功回执                |
| ↓ 下行 | `error`  | ✅ 已实现 | `{code, message}`                                                              | 上一帧处理失败                 |
| ↓ 下行 | `msg`    | ✅ 已实现 | `{msg_id, conv_id, from_uid, type, content, reply_to?, mentions?, created_at}` | 推送新消息（多端同步）         |
| ↓ 下行 | `notify` | 🚧 规划中 | `{conv_id, reader_uid, msg_id}`                                                | 已读回执通知                   |
| ↓ 下行 | `kick`   | 🚧 规划中 | -                                                                              | 服务端主动踢下线（重复登录等） |

> 当前已经打通 `send -> ackok -> Kafka fanout -> msg` 主链路。`send` 会走 `chat.HandleSend` 做幂等、成员校验、消息落库、会话最后消息更新和未读数更新；`read` 会更新当前用户在该会话的 `last_read_msg_id` 并清空 `unread`。`ack` / `recall` / `typing` 等上行帧还没有接入 dispatch，目前会返回 `error{code:"unknown_op"}`。

### `send` / `msg` 行为说明

- `send` 成功后，服务端先返回 `ackok` 给当前发送连接。`ackok` 只表示这次发送请求已经被服务端处理成功，当前实现里也就是消息已经落到 MySQL。
- `client_msg_id` 由客户端生成（建议 UUID v4），用于本地"发送中"草稿匹配，也用于服务端 Redis `SETNX` 幂等去重。重复发送命中幂等时，只会补回当前连接的 `ackok`，不会再次 fan-out `msg`。
- 新消息落库成功后，WS 层会查询会话成员，生成 `ChatEvent{MsgID, ConvID, Targets, Frame}`，再通过 `(*ws.Hub).Publish` 写入 Kafka。`Hub` 持有 `*bus.KafkaBus`，业务层不再访问 `bus` 包的全局变量。
- Kafka 只承担在线 fanout 事件通道，不是消息事实源；事实源仍是 MySQL 的 `messages` 表。Kafka publish 失败时，消息可能已经落库，客户端可通过历史消息接口补偿。
- 每个服务实例的 Kafka consumer 都会消费事件，并只向本机 `Hub` 上在线的目标用户连接投递 `msg`。不在线的用户不会被 Kafka "补发"，上线后走历史消息接口。
- `Targets` 包含会话所有成员，所以发送者自己的其它端会收到 `msg`；当前发送连接也会收到一份 `msg`。客户端需要按 `msg_id` 或 `client_msg_id` 做本地去重和状态合并。

### `read` 行为说明

- 当前 `read` 的语义是"把当前会话标记为已读到这条消息"，客户端应在已经展示到会话最新可见消息后再上报。
- 服务端会先校验当前用户是该会话成员，并确认 `msg_id` 属于这个 `conv_id`。
- 校验通过后，服务端更新当前用户的 `conversation_state.last_read_msg_id`，同时把该会话 `unread` 清零。这里的清零成立，是因为当前版本把 `read` 当成"会话已读"动作，而不是任意中间游标。
- 如果以后要支持"只读到中间某条，后面仍有未读"，需要把 `unread` 改成按 `last_read_msg_id` 之后的消息重新计算，而不是直接清零。
- 当前版本只更新当前用户自己的已读状态，暂不 fan-out `notify` 给会话其他成员。

### 帧示例

**心跳**

```json
// ↑ 上行
{ "op": "ping", "seq": 1 }

// ↓ 下行
{ "op": "pong", "seq": 1 }
```

**发送文本消息**

```json
// ↑ 上行
{
  "op": "send",
  "seq": 2,
  "data": {
    "client_msg_id": "550e8400-e29b-41d4-a716-446655440000",
    "conv_id": "conv-demo",
    "type": "text",
    "content": { "text": "你好，这是一条测试消息" }
  }
}

// ↓ 下行
{
  "op": "ackok",
  "seq": 2,
  "data": {
    "msg_id": "<服务端生成的 uuid>",
    "client_msg_id": "550e8400-e29b-41d4-a716-446655440000",
    "conv_id": "conv-demo",
    "ts": 1745673600123
  }
}

// ↓ 下行（fanout 后，会话成员的在线端都会收到；发送端也可能收到一份）
{
  "op": "msg",
  "data": {
    "msg_id": "<服务端生成的 uuid>",
    "conv_id": "conv-demo",
    "from_uid": 1,
    "type": "text",
    "content": { "text": "你好，这是一条测试消息" },
    "created_at": 1745673600123
  }
}
```

`ackok` 会带回原始 `seq`，用于和本次 `send` 请求配对；`msg` 是会话级新消息事件，不绑定发送请求的 `seq`。

**已读上报**

```json
// ↑ 上行
{
  "op": "read",
  "seq": 3,
  "data": {
    "conv_id": "conv-demo",
    "msg_id": "<当前会话最新可见消息的 uuid>"
  }
}
```

`read` 成功时当前版本不额外回包；失败时会返回带原始 `seq` 的 `error{code:"read_failed"}`。客户端再次拉 `GET /api/conversations` 时，对应会话的 `last_read_msg_id` 会更新，`unread` 会变成 `0`。

**错误回包**

```json
{
  "op": "error",
  "seq": 3,
  "data": {
    "code": "bad_frame | bad_data | unknown_op | send_failed | read_failed",
    "message": "<错误描述>"
  }
}
```

| code         | 触发场景                               |
| ------------ | -------------------------------------- |
| `bad_frame`  | 外层 Frame JSON 解析失败               |
| `bad_data`   | `data` 内层 payload 不符合 op 的结构   |
| `unknown_op` | 服务端不识别该 op（版本不匹配 / 拼错） |
| `send_failed` | `send` 业务处理失败，例如参数非法、非会话成员、落库失败 |
| `read_failed` | `read` 业务处理失败，例如参数非法、非会话成员、消息不属于该会话 |

### 连接生命周期约定

- **客户端心跳**：建议每 30s 发一次 `op:ping`，超时 10s 没有 `pong` 视为掉线重连。
- **服务端心跳**：服务端每约 54s 通过 WS 控制帧发一次 ping，60s 内没收到任何帧则关闭连接。gorilla 在控制帧层自动处理，浏览器 JS 不会显式收到。
- **单帧上限**：1MB（`maxMessageSize`），超过会被服务端关闭连接。
- **send 缓冲**：每连接 64 帧，下行写不过来会被踢（避免慢客户端拖死服务端）。

## 调试工具

### `/debug/ws-tester`

```
GET /debug/ws-tester
```

内嵌的网页测试端，提供：

- 一键调用 `/auth/login` 拿 token
- 一键 Upgrade `/ws/wslogin`
- 模板化发送 `send` 帧 / `ping` 帧 / 已读上报 / 任意原始 Frame
- 一键创建或选择 C2C / 群聊会话、拉群成员、邀请成员、退出群聊、拉历史消息、重复上一帧
- 收发日志面板

直接浏览器访问 `http://localhost:8080/debug/ws-tester` 即可使用。

> ⚠️ 当前没有挂鉴权中间件，**仅在开发环境暴露**，部署到外网前需要按 env 开关或加 IP 白名单。

## 邮箱发送Host的常见端口
项目使用 yeah.net 作为示例 smtp 的目的端口为465
