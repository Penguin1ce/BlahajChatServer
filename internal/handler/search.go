package handler

import (
	"net/http"

	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/service/conversation"
	userservice "BlahajChatServer/internal/service/user"
	"BlahajChatServer/pkg/consts"

	"github.com/gin-gonic/gin"
)

func SearchUsers(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}

	limit, err := parseIntQuery(c, "limit", 20)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, consts.ParamError)
		return
	}

	users, err := userservice.SearchUsers(c.Request.Context(), userID, c.Query("q"), limit)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, users)
}

func SearchGroups(c *gin.Context) {
	limit, err := parseIntQuery(c, "limit", 20)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, consts.ParamError)
		return
	}

	groups, err := conversation.SearchGroups(c.Request.Context(), c.Query("q"), limit)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, groups)
}
