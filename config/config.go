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

type MailConfig struct {
	Key        string `toml:"key"`
	SMTPHost   string `toml:"smtp_host"`
	ServerMail string `toml:"server_mail"`
	ServerPort int    `toml:"server_port"`
}

type TestValues struct {
	TestMail string `toml:"test_mail"`
}

type Config struct {
	Server     `toml:"server"`
	DB         `toml:"database"`
	Redis      `toml:"redis"`
	JWT        `toml:"jwt"`
	MailConfig `toml:"mail"`
	TestValues `toml:"test_values"`
}

var cfg Config

func InitConfig() {
	_, err := toml.DecodeFile("/Users/firefly/Developer/code/go/BlahajChatServer/config/config.toml", &cfg)
	if err != nil {
		log.Fatal("初始化环境失败 ", err)
	}
}

func GetConfig() Config {
	return cfg
}
