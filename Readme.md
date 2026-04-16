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

**在你这个项目里的流程：**

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

| | Access Token | Refresh Token |
|---|---|---|
| 寿命 | 15 分钟 | 30 天 |
| 存储位置 | 客户端内存 | 客户端持久存储 |
| 用途 | 每次请求鉴权 | Access Token 过期后换新的 |
| 存 Redis | 黑名单（注销时） | 主存储 |

简单说：**Access Token 负责日常通行，Refresh Token 负责续期。**