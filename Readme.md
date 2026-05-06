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

### 拉取当前用户会话列表

```
GET /api/conversations
```

返回当前登录用户加入的会话列表。会话列表由 `conversations` 和 `user_conv` 组合得到：

- `conversations` 提供会话公共信息：`conv_id`、`type`、`peer_key`、`name`、`avatar`、`owner_id`、`last_msg_id`、`last_msg_at`
- `user_conv` 提供当前用户自己的状态：`last_read_msg_id`、`unread`、`pinned`、`muted`
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

### 拉取历史消息

```
GET /api/conversations/:id/messages?before_id=0&limit=20
```

| Query       | 类型   | 必填 | 说明                                          |
| ----------- | ------ | ---- | --------------------------------------------- |
| `before_id` | number | 否   | 游标；传 `0` 表示拉最新一页                    |
| `limit`     | number | 否   | 每页数量，默认 `20`，最大 `100`                |

服务端会先校验当前用户是否属于该会话；不是成员时返回 `403`。

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
| ↑ 上行 | `read`   | 🚧 规划中 | `{conv_id, msg_id}`                                                            | 已读上报                       |
| ↑ 上行 | `recall` | 🚧 规划中 | `{msg_id}`                                                                     | 撤回                           |
| ↑ 上行 | `typing` | 🚧 规划中 | `{conv_id}`                                                                    | 输入中（不落库，直接 fan-out） |
| ↓ 下行 | `pong`   | ✅ 已实现 | 无                                                                             | `ping` 的回应                  |
| ↓ 下行 | `ackok`  | ✅ 已实现 | `{msg_id, client_msg_id, conv_id, ts}`                                         | `send` 成功回执                |
| ↓ 下行 | `error`  | ✅ 已实现 | `{code, message}`                                                              | 上一帧处理失败                 |
| ↓ 下行 | `msg`    | ✅ 已实现 | `{msg_id, conv_id, from_uid, type, content, reply_to?, mentions?, created_at}` | 推送新消息（多端同步）         |
| ↓ 下行 | `notify` | 🚧 规划中 | `{conv_id, reader_uid, msg_id}`                                                | 已读回执通知                   |
| ↓ 下行 | `kick`   | 🚧 规划中 | -                                                                              | 服务端主动踢下线（重复登录等） |

> 当前已经打通 `send -> ackok -> Kafka fanout -> msg` 主链路。`send` 会走 `service.HandleSend` 做幂等、成员校验、消息落库、会话最后消息更新和未读数更新；`ack` / `read` / `recall` / `typing` 等上行帧还没有接入 dispatch，目前会返回 `error{code:"unknown_op"}`。

### `send` / `msg` 行为说明

- `send` 成功后，服务端先返回 `ackok` 给当前发送连接。`ackok` 只表示这次发送请求已经被服务端处理成功，当前实现里也就是消息已经落到 MySQL。
- `client_msg_id` 由客户端生成（建议 UUID v4），用于本地"发送中"草稿匹配，也用于服务端 Redis `SETNX` 幂等去重。重复发送命中幂等时，只会补回当前连接的 `ackok`，不会再次 fan-out `msg`。
- 新消息落库成功后，WS 层会查询会话成员，生成 `ChatEvent{MsgID, ConvID, Targets, Frame}`，再调用 `bus.Global.Publish` 写入 Kafka。
- Kafka 只承担在线 fanout 事件通道，不是消息事实源；事实源仍是 MySQL 的 `messages` 表。Kafka publish 失败时，消息可能已经落库，客户端可通过历史消息接口补偿。
- 每个服务实例的 Kafka consumer 都会消费事件，并只向本机 `Hub` 上在线的目标用户连接投递 `msg`。不在线的用户不会被 Kafka "补发"，上线后走历史消息接口。
- `Targets` 包含会话所有成员，所以发送者自己的其它端会收到 `msg`；当前发送连接也会收到一份 `msg`。客户端需要按 `msg_id` 或 `client_msg_id` 做本地去重和状态合并。

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

**错误回包**

```json
{
  "op": "error",
  "seq": 3,
  "data": {
    "code": "bad_frame | bad_data | unknown_op | send_failed",
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
- 模板化发送 `send` 帧 / `ping` 帧 / 任意原始 Frame
- 收发日志面板

直接浏览器访问 `http://localhost:8080/debug/ws-tester` 即可使用。

> ⚠️ 当前没有挂鉴权中间件，**仅在开发环境暴露**，部署到外网前需要按 env 开关或加 IP 白名单。

## 邮箱发送Host的常见端口
项目使用 yeah.net 作为示例 smtp 的目的端口为465
