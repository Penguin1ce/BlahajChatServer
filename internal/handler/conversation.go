package handler

import (
	"errors"
	"net/http"
	"strconv"

	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/service"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"

	"github.com/gin-gonic/gin"
)

func GetOrCreateC2C(c *gin.Context) {
	var req requests.GetOrCreateC2CReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	uid, exists := c.Get(consts.CtxUserID)
	userID, ok := uid.(uint64)
	if !exists || !ok || userID == 0 {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}

	conversation, err := service.GetOrCreateC2C(c.Request.Context(), userID, req.PeerUID)
	if err != nil {
		if errors.Is(err, errs.ErrFoundC2CPair) {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, toConversationResp(conversation))
}

func toConversationResp(conv *model.Conversation) response.Conversation {
	resp := response.Conversation{
		ConvID:    conv.ConvId,
		Type:      conv.Type,
		Name:      conv.Name,
		Avatar:    conv.Avatar,
		OwnerID:   conv.OwnerID,
		LastMsgID: conv.LastMsgID,
		LastMsgAt: conv.LastMsgAt.UnixMilli(),
	}
	if conv.PeerKey != nil {
		resp.PeerKey = *conv.PeerKey
	}
	return resp
}

func GetHistoryMessage(c *gin.Context) {
	uid, exists := c.Get(consts.CtxUserID)
	userID, ok := uid.(uint64)
	if !exists || !ok || userID == 0 {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}

	convID := c.Param("id")
	if convID == "" {
		response.Fail(c, http.StatusBadRequest, consts.LackConversionID)
		return
	}

	beforeID, err := parseUintQuery(c, "before_id", 0)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, consts.ParamError)
		return
	}
	limit, err := parseIntQuery(c, "limit", 20)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, consts.ParamError)
		return
	}

	msgs, err := service.GetHistoryMessageByID(c.Request.Context(), userID, convID, beforeID, limit)
	if err != nil {
		if errors.Is(err, errs.ErrNotMember) {
			response.Fail(c, http.StatusForbidden, err.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, msgs)
}

func parseUintQuery(c *gin.Context, key string, defaultValue uint64) (uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func parseIntQuery(c *gin.Context, key string, defaultValue int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}
