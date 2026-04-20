package requests

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

type WebsocketLoginReq struct {
	AccessToken string `json:"access_token" binding:"required"`
}
