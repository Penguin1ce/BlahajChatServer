package dao

import (
	"fmt"
	"log"
	"time"

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

	configureConnPool(db, c)

	if err := db.AutoMigrate(
		&model.User{},
		&model.Friendship{},
		&model.FriendApply{},
		&model.Block{},
		&model.Conversation{},
		&model.GroupInfo{},
		&model.ConversationState{},
		&model.Message{},
	); err != nil {
		log.Fatal("AutoMigrate 失败 ", err)
	}

	DB = db
}

// configureConnPool 设置底层 *sql.DB 连接池。配置缺省时用一套保守默认值兜底，
// 避免默认无上限连接数打爆 MySQL，以及长连接撞上服务端 wait_timeout 拿到坏连接。
func configureConnPool(db *gorm.DB, c config.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("获取底层 sql.DB 失败 ", err)
	}

	maxOpen := c.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}
	maxIdle := c.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	lifetimeMin := c.ConnMaxLifetimeMinutes
	if lifetimeMin <= 0 {
		lifetimeMin = 30
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(lifetimeMin) * time.Minute)
}
