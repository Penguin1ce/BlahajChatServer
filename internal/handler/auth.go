package handler

import (
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/middleware"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/service"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func toUserResp(u *model.User) response.UserResp {
	return response.UserResp{
		ID:        u.ID,
		Email:     u.Email,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func toTokenPairResp(tp *service.TokenPair) response.TokenPairResp {
	return response.TokenPairResp{
		AccessToken:  tp.AccessToken,
		RefreshToken: tp.RefreshToken,
		ExpiresIn:    tp.ExpiresIn,
	}
}

func Register(c *gin.Context) {
	var req requests.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, err)
		return
	}
	u, err := service.Register(c.Request.Context(), req.Email, req.Password, req.Nickname)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			response.Err(c, http.StatusConflict, err)
			return
		}
		response.Err(c, http.StatusInternalServerError, err)
		return
	}
	response.OK(c, response.RegisterResp{User: toUserResp(u)})
}

func Login(c *gin.Context) {
	var req requests.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, err)
		return
	}
	u, tp, err := service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Err(c, http.StatusUnauthorized, err)
			return
		}
		response.Err(c, http.StatusInternalServerError, err)
		return
	}
	response.OK(c, response.LoginResp{
		User:  toUserResp(u),
		Token: toTokenPairResp(tp),
	})
}

func Refresh(c *gin.Context) {
	var req requests.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, err)
		return
	}
	tp, err := service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Err(c, http.StatusUnauthorized, err)
		return
	}
	response.OK(c, response.RefreshResp{Token: toTokenPairResp(tp)})
}

func Logout(c *gin.Context) {
	var req requests.LogoutReq
	_ = c.ShouldBindJSON(&req)

	jti, _ := c.Get(middleware.CtxJTI)
	exp, _ := c.Get(middleware.CtxExp)
	jtiStr, _ := jti.(string)
	expT, _ := exp.(time.Time)

	_ = service.Logout(c.Request.Context(), req.RefreshToken, jtiStr, expT)
	response.OK(c, response.LogoutResp{OK: true})
}

func Me(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxUserID)
	id, _ := uid.(uint64)
	u, err := dao.GetUserByID(id)
	if err != nil || u == nil {
		response.ErrMsg(c, http.StatusNotFound, "用户不存在")
		return
	}
	response.OK(c, response.MeResp{User: toUserResp(u)})
}
