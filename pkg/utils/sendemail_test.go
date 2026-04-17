package utils

import (
	"BlahajChatServer/config"
	"testing"
)

func TestSentMail(t *testing.T) {
	config.InitConfig()
	cfg := config.GetConfig()
	t.Logf("SMTPHost: %q", cfg.SMTPHost)
	t.Logf("ServerMail: %q", cfg.MailConfig.ServerMail)
	err := SentMail(cfg.TestMail, "你的验证码为110011")
	if err != nil {
		t.Error(err)
		return
	}
}
