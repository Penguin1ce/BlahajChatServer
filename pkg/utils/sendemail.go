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
	m.SetBody("text/html", "<h1>验证码!</h1><p>这是你的验证码 "+message+" 邮件</p><p>有效期1分钟</p>")
	d := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.ServerMail, cfg.Key)
	d.SSL = true
	if err := d.DialAndSend(m); err != nil {
		log.Println(err)
		return err
	}
	return nil
}
