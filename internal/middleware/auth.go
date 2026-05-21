package middleware

import (
	"net/http"
	"strings"

	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/service/auth"
	"BlahajChatServer/pkg/consts"

	"github.com/gin-gonic/gin"
)

// JWTAuth 从 Authorization: Bearer <token> 或 ?token= 提取并校验
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			response.Abort(c, http.StatusUnauthorized, "缺少 token")
			return
		}
		claims, err := auth.ParseAccessToken(tokenStr)
		if err != nil {
			response.Abort(c, http.StatusUnauthorized, "token 无效: "+err.Error())
			return
		}
		if auth.IsAccessBlacklisted(c.Request.Context(), claims.ID) {
			response.Abort(c, http.StatusUnauthorized, "token 已失效")
			return
		}
		c.Set(consts.CtxUserID, claims.UserID)
		c.Set(consts.CtxJTI, claims.ID)
		if claims.ExpiresAt != nil {
			c.Set(consts.CtxExp, claims.ExpiresAt.Time)
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return c.Query("token")
}
