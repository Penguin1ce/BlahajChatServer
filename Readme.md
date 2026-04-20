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
ws://host/ws?token=<access_token>
```

## 邮箱发送Host的常见端口
项目使用 yeah.net 作为示例 smtp 的目的端口为465
