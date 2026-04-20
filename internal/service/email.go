package service

import (
	"context"

	"BlahajChatServer/internal/redis"
	"BlahajChatServer/internal/zlog"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"
	"BlahajChatServer/pkg/utils"
)

// SendEmailCode 生成 6 位验证码，原子写入 Redis 并发送邮件。
// 若 key 已存在（冷却期内），返回 errs.ErrEmailCodeBusy；
// 邮件发送失败会回滚 key，返回 errs.ErrSendMail。
func SendEmailCode(ctx context.Context, email string) error {
	// key格式为sendEmailCode:email
	key := consts.RedisSendEmailCodeKey + email
	code := utils.SixUUID()

	ok, err := redis.SetNXValueByKeyExpire(key, code, consts.EmailCodeTTL)
	if err != nil {
		return err
	}
	if !ok {
		return errs.ErrEmailCodeBusy
	}

	if err := utils.SentMail(email, code); err != nil {
		redis.DelValueByKey(key)
		zlog.Error("发送验证码邮件失败", "email", email, "err", err)
		return errs.ErrSendMail
	}
	return nil
}
