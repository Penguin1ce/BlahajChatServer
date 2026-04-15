package config

import (
	"log"

	"github.com/BurntSushi/toml"
)

type Server struct {
	Port int    `toml:"port"`
	Env  string `toml:"env"`
}

type DB struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Name     string `toml:"name"`
}

type Redis struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`
}

type JWT struct {
	Secret           string `toml:"secret"`
	AccessTTLMinutes int    `toml:"access_ttl_minutes"`
	RefreshTTLDays   int    `toml:"refresh_ttl_days"`
}

type Config struct {
	Server `toml:"server"`
	DB     `toml:"database"`
	Redis  `toml:"redis"`
	JWT    `toml:"jwt"`
}

var CFG Config

func InitConfig() {
	_, err := toml.DecodeFile("./config/config.toml", &CFG)
	if err != nil {
		log.Fatal("初始化环境失败 ", err)
	}
}
