package router

import (
	"BlahajChatServer/internal/handler"
	"BlahajChatServer/internal/middleware"
	"BlahajChatServer/internal/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var GE *gin.Engine

func Init() {
	GE = gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	GE.Use(cors.New(corsConfig))

	GE.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	debug := GE.Group("/debug")
	{
		debug.GET("/ws-tester", handler.WSTesterPage)
	}

	wss := GE.Group("/ws", middleware.JWTAuth())
	{
		wss.GET("/wslogin", ws.WSLoginHandler)
	}

	auth := GE.Group("/auth")
	{
		auth.POST("/getcode", handler.GetEmailCode)
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
		auth.POST("/refresh", handler.Refresh)
		auth.POST("/logout", middleware.JWTAuth(), handler.Logout)
	}

	api := GE.Group("/api", middleware.JWTAuth())
	{
		api.GET("/me", handler.Me)

		// 这里是会话列表相关接口
		api.GET("conversations", handler.GetConversationList)
		api.POST("conversations/c2c", handler.GetOrCreateC2C)
		api.POST("conversations/group", handler.CreateGroupConversation)
		api.GET("conversations/:id/members", handler.ListGroupMembers)
		api.PUT("conversations/:id/members", handler.AddGroupMembers)
		api.DELETE("conversations/:id/members/me", handler.LeaveGroup)
		api.GET("conversations/:id/messages", handler.GetHistoryMessage)

		// 这里是好友列表相关接口
		api.GET("friends", handler.ListFriendsHandler)
		api.POST("friends/apply", handler.ApplyFriendshipHandler)
		api.GET("friends/applies", handler.ListFriendAppliesHandler)
		api.POST("friends/applies/:id/accept", handler.AcceptFriendApplyHandler)
		api.POST("friends/applies/:id/reject", handler.RejectFriendApplyHandler)
	}
}
