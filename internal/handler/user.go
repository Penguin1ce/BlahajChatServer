package handler

import (
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/redis"
	"BlahajChatServer/internal/service"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetEmailCode 获取注册验证码
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

// Register 用户注册
func Register(c *gin.Context) {
	var req requests.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	key := consts.RedisSendEmailCodeKey + req.Email
	if !redis.ExistsKey(key) {
		response.Fail(c, http.StatusBadRequest, consts.EmailNotExist)
		return
	}
	code, err := redis.GetValueByKey(key)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, consts.SystemError)
		return
	}
	if code != req.EmailCode {
		response.Fail(c, http.StatusBadRequest, consts.EmailCodeErr)
		return
	}
	u, err := service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errs.ErrEmailTaken) {
			response.Fail(c, http.StatusConflict, err.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, consts.SystemError)
		return
	}
	redis.DelValueByKey(key)
	response.OK(c, toUserResp(u))
}

// Login 用户的普通登录接口
func Login(c *gin.Context) {
	var req requests.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	u, tokenPair, err := service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, errs.ErrInvalidCredentials) {
			response.Fail(c, http.StatusUnauthorized, errs.ErrInvalidCredentials.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, consts.SystemError)
		return
	}
	tokens := toTokenPairResp(tokenPair)
	userResp := toUserResp(u)
	response.OK(c, response.LoginResp{
		User:  userResp,
		Token: tokens,
	})

}
