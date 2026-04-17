package consts

import "time"

const (
	CtxUserID             = "userID"
	CtxJTI                = "jti"
	CtxExp                = "exp"
	RedisSendEmailCodeKey = "sendEmailCode:"

	// 验证码有效期
	EmailCodeTTL = time.Minute

	// 这里是成功信息枚举
	SystemSendSuccess = "发送成功,请前往邮箱查收"

	// 这里是错误信息枚举
	SystemError     = "系统错误"
	SystemEmailBusy = "您申请验证邮箱太频繁啦,等等再试"
	SystemMailFail  = "邮件发送失败,请稍后再试"
)
