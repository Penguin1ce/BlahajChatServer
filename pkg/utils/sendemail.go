package utils

import (
	"BlahajChatServer/config"
	"log"

	"gopkg.in/gomail.v2"
)

func SentMail(email string, message string) error {
	cfg := config.GetConfig()
	m := gomail.NewMessage()
	m.SetHeader("From", config.GetConfig().MailConfig.ServerMail)
	m.SetHeader("To", email)
	m.SetHeader("Subject", "测试邮件")
	body := message
	m.SetBody("text/html", "<h1>Hello!</h1><p>这是 "+body+" 邮件</p>")
	d := gomail.NewDialer(cfg.SMTPHost, cfg.ServerPort, cfg.ServerMail, cfg.Key)
	d.SSL = true
	if err := d.DialAndSend(m); err != nil {
		log.Println(err)
		return err
	}
	return nil
}
