package handler

import (
	"net/http"

	"BlahajChatServer/internal/dao"
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

}

func Logout(c *gin.Context) {

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
