package main

import (
	"fmt"
	"strconv"

	"BlahajChatServer/config"
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/router"
)

func main() {
	config.InitConfig()
	fmt.Println("Starting server... env =", config.CFG.Server.Env)
	dao.InitMySQL()
	dao.InitRedis()
	router.Init()
	if err := router.GE.Run(":" + strconv.Itoa(config.CFG.Server.Port)); err != nil {
		panic(err)
	}
}
