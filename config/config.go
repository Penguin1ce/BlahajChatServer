package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const defaultConfigPath = "config/config.toml"

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
	SMTPPort   int    `toml:"smtp_port"`
}

type TestValues struct {
	TestMail string `toml:"test_mail"`
}

type Log struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	File   string `toml:"file"`
}

type Kafka struct {
	Brokers []string `toml:"brokers"`
	Topic   string   `toml:"topic"`
	GroupID string   `toml:"group_id"`
}

type Config struct {
	Server     `toml:"server"`
	DB         `toml:"database"`
	Redis      `toml:"redis"`
	JWT        `toml:"jwt"`
	MailConfig `toml:"mail"`
	TestValues `toml:"test_values"`
	Log        `toml:"log"`
	Kafka      `toml:"kafka"`
}

var CFG Config

func InitConfig() {
	path := resolveConfigPath()

	_, err := toml.DecodeFile(path, &CFG)
	if err != nil {
		log.Fatal("初始化环境失败 ", "path=", path, " err=", err)
	}
}

func GetConfig() Config {
	return CFG
}

func resolveConfigPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}

	dir, err := os.Getwd()
	if err != nil {
		return defaultConfigPath
	}
	for {
		path := filepath.Join(dir, defaultConfigPath)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return defaultConfigPath
		}
		dir = parent
	}
}
