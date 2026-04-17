package response

// TokenPairResp token 对
type TokenPairResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type LoginResp struct {
	User  UserResp      `json:"user"`
	Token TokenPairResp `json:"token"`
}

type RefreshResp struct {
	Token TokenPairResp `json:"token"`
}

type LogoutResp struct {
	OK bool `json:"ok"`
}
