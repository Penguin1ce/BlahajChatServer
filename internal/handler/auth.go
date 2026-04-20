package handler

import (
	"net/http"
	"time"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/service"
	"BlahajChatServer/pkg/consts"

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

func Refresh(c *gin.Context) {
	var req requests.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	tp, err := service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	response.OK(c, response.RefreshResp{Token: toTokenPairResp(tp)})
}

func Logout(c *gin.Context) {
	uid, _ := c.Get(consts.CtxUserID)
	userID, _ := uid.(uint64)
	jti, _ := c.Get(consts.CtxJTI)
	accessJTI, _ := jti.(string)
	exp, _ := c.Get(consts.CtxExp)
	accessExp, _ := exp.(time.Time)

	var req requests.LogoutReq
	_ = c.ShouldBindJSON(&req)

	_ = service.Logout(c.Request.Context(), userID, req.RefreshToken, accessJTI, accessExp)
	response.OK(c, response.LogoutResp{OK: true})
}

func Me(c *gin.Context) {
	uid, exists := c.Get(consts.CtxUserID)
	id, ok := uid.(uint64)
	if !exists || !ok || id == 0 {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	u, err := dao.GetUserByID(id)
	if err != nil || u == nil {
		response.Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	response.OK(c, response.MeResp{User: toUserResp(u)})
}
