package requests

type RegisterReq struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6,max=64"`
	EmailCode string `json:"email_code" binding:"required,min=6,max=64"`
	Nickname  string `json:"nickname" binding:"max=32"`
}

type RegisterEmailCodeReq struct {
	Email string `json:"email" binding:"required,email"`
}

type LoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

type WebsocketLoginReq struct {
	AccessToken string `json:"access_token" binding:"required"`
}
