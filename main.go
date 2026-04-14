package main

import (
	"BlahajChatServer/config"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	config.InitConfig()
	fmt.Println("Starting server...")
	fmt.Println(config.CFG.Server.Env)
	router := gin.Default()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	router.Run(":" + strconv.Itoa(config.CFG.Server.Port)) // listens on 0.0.0.0:8080 by default
}
