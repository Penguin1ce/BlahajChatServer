// Package errs 集中管理业务层可返回给上层的 sentinel error。
// handler 层通过 errors.Is 匹配这些值，映射到合适的 HTTP 状态码与文案。
package errs

import "errors"

var (
	ErrEmailTaken          = errors.New("邮箱已被注册")
	ErrInvalidCredentials  = errors.New("邮箱或密码错误")
	ErrInvalidRefresh      = errors.New("refresh token 无效或已过期")
	ErrEmailCodeBusy       = errors.New("验证码发送过于频繁")
	ErrSendMail            = errors.New("邮件发送失败")
	ErrConvNotFound        = errors.New("对话没有找到")
	ErrFoundC2CPair        = errors.New("不正确的C2C对")
	ErrNotMember           = errors.New("不是会话成员")
	ErrMsgNotFound         = errors.New("消息不存在")
	ErrInvalidMessage      = errors.New("消息参数错误")
	ErrSystem              = errors.New("系统错误")
	ErrApplyUserNotFound   = errors.New("申请的好友不存在")
	ErrCannotFriendSelf    = errors.New("不能添加自己")
	ErrFriendBlocked       = errors.New("存在拉黑关系，不能添加好友")
	ErrAlreadyFriend       = errors.New("已经是好友")
	ErrFriendApplyPending  = errors.New("已有待处理的好友申请")
	ErrFriendApplyNotFound = errors.New("好友申请不存在")
	ErrFriendApplyHandled  = errors.New("好友申请已处理")
	ErrNoPermission        = errors.New("没有操作权限")
)
