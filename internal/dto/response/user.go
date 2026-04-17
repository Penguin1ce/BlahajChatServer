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

type RegisterResp struct {
	User UserResp `json:"user"`
}

type MeResp struct {
	User UserResp `json:"user"`
}
