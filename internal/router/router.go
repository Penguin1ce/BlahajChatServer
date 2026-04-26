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
	}
}
