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

## 登录接口逻辑
客户端先走普通 HTTP 登录接口 → 拿到 access_token
连接 WebSocket 时把 token 带上：ws://host/ws?token=xxx
服务端在 Upgrade 前跑 JWTAuth() 中间件校验
校验通过才 Upgrade，之后从 context 读 userID

Access Token 就是证明"你是谁"的凭证。

**具体作用：**

1. **身份认证** — 每次请求带上它，服务端解析出 `userID`，知道是哪个用户在操作
2. **无状态校验** — 服务端不需要查数据库，直接验签就能确认有效性（JWT 自带签名）
3. **短期有效** — 当前配置 15 分钟过期，泄露了危害有限

**在这个项目里的流程：**

```
登录 → 拿到 access_token + refresh_token
        ↓
每次 HTTP 请求：Header 带 Authorization: Bearer <access_token>
        ↓
JWTAuth 中间件解析 → 写入 ctx (userID / jti / exp)
        ↓
handler 直接用 userID，不用再查"当前是谁"
```

**和 Refresh Token 的分工：**

|          | Access Token     | Refresh Token             |
| -------- | ---------------- | ------------------------- |
| 寿命     | 15 分钟          | 30 天                     |
| 存储位置 | 客户端内存       | 客户端持久存储            |
| 用途     | 每次请求鉴权     | Access Token 过期后换新的 |
| 存 Redis | 黑名单（注销时） | 主存储                    |

简单说：**Access Token 负责日常通行，Refresh Token 负责续期。**

## 邮箱发送Host的常见端口
项目使用 yeah.net 作为示例 smtp 的目的端口为465
