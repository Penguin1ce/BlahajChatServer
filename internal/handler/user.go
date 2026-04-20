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
	response.OK(c, response.UserResp{
		ID:        u.ID,
		Email:     u.Email,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	})
}

func Login(c *gin.Context) {

}
