package dao

import (
	"fmt"
	"log"

	"BlahajChatServer/config"
	"BlahajChatServer/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitMySQL() {
	c := config.CFG.DB
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Name)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接 MySQL 失败 ", err)
	}

	if err := db.AutoMigrate(&model.User{}); err != nil {
		log.Fatal("AutoMigrate 失败 ", err)
	}

	DB = db
}
