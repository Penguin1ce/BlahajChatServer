# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## 项目概览

Blahaj Chat Server 是一个 SwiftUI 聊天室客户端对应的 Go 后端。当前不考虑上架，登录采用邮箱 + 密码。主要依赖：Gin、GORM (MySQL)、go-redis、WebSocket（待接入）。

## 常用命令

```bash
# 本地开发（需先 cp config/config.example.toml config/config.toml 并填好参数）
go run ./cmd/server

# 整仓编译检查
go build ./...

# 依赖整理
go mod tidy

# 单元测试（如有，禁止执行 go test ./...）
go test ./internal/service -run TestLogin   # 跑单个测试

# 一键起 MySQL + Redis + Server（容器）
docker compose up -d --build
```

使用 docker compose 时，`config/config.toml` 内的 `database.host` 要改为 `mysql`、`redis.host` 改为 `redis`（容器网络名）。本地直连用 `127.0.0.1`。

## 架构

启动流程集中在 `cmd/server/main.go`：`config.InitConfig` → `dao.InitMySQL`（含 `AutoMigrate`）→ `dao.InitRedis` → `router.Init` → `router.GE.Run`。路由不再放 `init()` 里，改成显式 `Init()`，以保证 DAO 全局变量已就绪。

分层（严格单向依赖 handler → service → dao → model）：

- `config/` — TOML 配置，全局 `config.CFG`
- `internal/model/` — GORM 数据模型（`User` 表名固定为 `users`）
- `internal/dao/` — 数据访问层。`DB` (gorm) 和 `RDB` (go-redis) 都是包级全局变量，被 service 层直接使用
- `internal/service/` — 业务逻辑。`jwt.go` 签发/解析 HS256 Access Token；`auth.go` 注册/登录/刷新/登出
- `internal/handler/` — Gin handler，只做参数绑定和错误码映射
- `internal/middleware/` — `JWTAuth()` 从 `Authorization: Bearer` 或 `?token=` 提取，校验后把 `userID/jti/exp` 写入 gin.Context
- `internal/router/` — 路由聚合，导出全局 `GE *gin.Engine`

### 认证模型

- **Access Token**：JWT（HS256），短 TTL（默认 15min），无状态校验，claim 里带 `jti`
- **Refresh Token**：32 字节随机 hex，长 TTL（默认 30d），存 Redis `refresh:{token}` → `userID`
- **主动注销**：删除 Redis 中的 refresh key，并把 Access Token 的 `jti` 写入 `blacklist:{jti}`，TTL = Access 剩余时间
- **刷新**采用轮换策略：旧 refresh 立即失效，返回新对
- **密码**：bcrypt（DefaultCost），绝不明文

JWT 的 Secret、TTL、Redis 连接信息全部来自 `config.CFG`，改动时先看 `config/config.go` 的结构体定义。

### WebSocket 接入约定（待实现）

在 WS handshake 的 Gin 路由上直接挂 `middleware.JWTAuth()`，客户端通过 `ws://.../ws?token=xxx` 连接；Upgrade 前拒绝未授权，Upgrade 后从 context 读 `middleware.CtxUserID`。

## 注意事项

- `config/config.toml` 不入库（`.gitignore` 已覆盖），只维护 `config.example.toml`
- `dao.InitMySQL` 会对 `model.User` 执行 `AutoMigrate`，新增模型需在这里注册
- 不要执行 `go test ./...`；需要测试时只跑相关包或具体用例
- 回复使用中文（user 偏好）
