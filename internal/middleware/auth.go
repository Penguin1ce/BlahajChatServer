package middleware

import (
	"BlahajChatServer/pkg/consts"
	"strings"

	"BlahajChatServer/internal/service"

	"github.com/gin-gonic/gin"
)

// JWTAuth 从 Authorization: Bearer <token> 或 ?token= 提取并校验
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "缺少 token"})
			return
		}
		claims, err := service.ParseAccessToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "token 无效: " + err.Error()})
			return
		}
		if service.IsAccessBlacklisted(c.Request.Context(), claims.ID) {
			c.AbortWithStatusJSON(401, gin.H{"error": "token 已失效"})
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
