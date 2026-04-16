package response

import "time"

// UserResp 用户信息（不含密码）
type UserResp struct {
	ID        uint64    `json:"id"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TokenPairResp token 对
type TokenPairResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RegisterResp 注册接口响应
type RegisterResp struct {
	User UserResp `json:"user"`
}

// LoginResp 登录接口响应
type LoginResp struct {
	User  UserResp      `json:"user"`
	Token TokenPairResp `json:"token"`
}

// RefreshResp 刷新 token 接口响应
type RefreshResp struct {
	Token TokenPairResp `json:"token"`
}

// LogoutResp 登出接口响应
type LogoutResp struct {
	OK bool `json:"ok"`
}

// MeResp 当前用户信息接口响应
type MeResp struct {
	User UserResp `json:"user"`
}
