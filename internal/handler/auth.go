package handler

import (
	"errors"
	"net/http"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/service"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"

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

func GetEmailCode(c *gin.Context) {
	var req requests.RegisterEmailCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.SendEmailCode(c.Request.Context(), req.Email); err != nil {
		switch {
		case errors.Is(err, errs.ErrEmailCodeBusy):
			response.Fail(c, http.StatusTooManyRequests, consts.SystemEmailBusy)
		case errors.Is(err, errs.ErrSendMail):
			response.Fail(c, http.StatusBadGateway, consts.SystemMailFail)
		default:
			response.Fail(c, http.StatusInternalServerError, consts.SystemError)
		}
		return
	}
	response.OK(c, consts.SystemSendSuccess)
}

func Register(c *gin.Context) {
	var req requests.RegisterReq
	// TODO 添加注册功能
	_ = req
}

func Login(c *gin.Context) {

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
