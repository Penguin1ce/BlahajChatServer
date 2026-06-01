package consts

import "time"

const (
	CtxUserID             = "userID"
	CtxJTI                = "jti"
	CtxExp                = "exp"
	RedisSendEmailCodeKey = "sendEmailCode:"

	// redis做幂等的键
	ClientMessageKey = "clientMessageKey:"

	// cache-aside 缓存 key 前缀
	GroupMembersKey = "group:members:" // group:members:{convID} -> JSON []uint64
	UserProfileKey  = "user:profile:"  // user:profile:{uid}     -> JSON model.User（不含密码）

	// 用户的默认信息
	DefaultAvatarURL = "https://images.cdn.org/img/index/sticker.webp"

	// 验证码有效期
	EmailCodeTTL = 5 * time.Minute

	// client_msg_id 幂等 key 的有效期
	ClientMsgIDIdemTTL = 24 * time.Hour

	// 缓存 TTL：即使写时已删 key，仍靠 TTL 兜底漏删/进程崩溃导致的脏缓存
	GroupMembersTTL = 10 * time.Minute
	UserProfileTTL  = 30 * time.Minute

	// 这里是成功信息枚举
	SystemSendSuccess = "发送成功,请前往邮箱查收"

	// 这里是错误信息枚举
	SystemError      = "系统错误"
	SystemEmailBusy  = "您申请验证邮箱太频繁啦,等等再试"
	SystemMailFail   = "邮件发送失败,请稍后再试"
	EmailNotExist    = "该邮箱不存在"
	EmailCodeErr     = "邮箱验证码错误"
	EmailExist       = "该邮箱已存在"
	UserMessageError = "用户权限错误"
	UserNotLogin     = "用户未登录"
	LackConversionID = "缺少会话 ID"
	ParamError       = "参数错误"
)
