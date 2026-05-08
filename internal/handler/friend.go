package handler

import (
	"errors"
	"net/http"
	"strconv"

	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/service/friend"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"

	"github.com/gin-gonic/gin"
)

// ApplyFriendshipHandler 申请添加好友。
func ApplyFriendshipHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}

	var req requests.ApplyFriendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	apply, err := friend.ApplyFriend(c.Request.Context(), userID, req)
	if err != nil {
		failFriendError(c, err)
		return
	}
	response.OK(c, apply)
}

func ListFriendAppliesHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}

	applies, err := friend.ListPendingApplies(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, applies)
}

func AcceptFriendApplyHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}
	applyID, err := parseIDParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, consts.ParamError)
		return
	}

	conv, err := friend.AcceptApply(c.Request.Context(), userID, applyID)
	if err != nil {
		failFriendError(c, err)
		return
	}
	response.OK(c, toConversationResp(conv))
}

func RejectFriendApplyHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}
	applyID, err := parseIDParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, consts.ParamError)
		return
	}

	if err := friend.RejectApply(c.Request.Context(), userID, applyID); err != nil {
		failFriendError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func ListFriendsHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, consts.UserNotLogin)
		return
	}

	friends, err := friend.ListFriends(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, friends)
}

func currentUserID(c *gin.Context) (uint64, bool) {
	uid, exists := c.Get(consts.CtxUserID)
	userID, ok := uid.(uint64)
	return userID, exists && ok && userID != 0
}

func parseIDParam(c *gin.Context, key string) (uint64, error) {
	return strconv.ParseUint(c.Param(key), 10, 64)
}

func failFriendError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrCannotFriendSelf):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, errs.ErrApplyUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrFriendBlocked):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, errs.ErrAlreadyFriend), errors.Is(err, errs.ErrFriendApplyPending):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, errs.ErrFriendApplyNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrFriendApplyHandled):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, errs.ErrNoPermission):
		response.Fail(c, http.StatusForbidden, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, err.Error())
	}
}
