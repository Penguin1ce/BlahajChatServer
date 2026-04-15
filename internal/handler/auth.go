package handler

import (
	"errors"
	"net/http"
	"time"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/middleware"
	"BlahajChatServer/internal/service"

	"github.com/gin-gonic/gin"
)

type registerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Nickname string `json:"nickname" binding:"max=32"`
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

func Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := service.Register(c.Request.Context(), req.Email, req.Password, req.Nickname)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u})
}

func Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, tp, err := service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "token": tp})
}

func Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tp, err := service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tp})
}

func Logout(c *gin.Context) {
	var req logoutReq
	_ = c.ShouldBindJSON(&req)

	jti, _ := c.Get(middleware.CtxJTI)
	exp, _ := c.Get(middleware.CtxExp)
	jtiStr, _ := jti.(string)
	expT, _ := exp.(time.Time)

	_ = service.Logout(c.Request.Context(), req.RefreshToken, jtiStr, expT)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func Me(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxUserID)
	id, _ := uid.(uint64)
	u, err := dao.GetUserByID(id)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u})
}
