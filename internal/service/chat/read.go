package chat

import (
	"context"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/wspayload"
	"BlahajChatServer/pkg/errs"
)

func HandleRead(ctx context.Context, uid uint64, readData wspayload.ReadData) error {
	// 1.先校验参数不能为空：
	if readData.ConvID == "" || readData.MsgID == "" {
		return errs.ErrInvalidMessage
	}

	// 2.校验当前用户是不是这个会话成员：
	member, err := dao.IsMember(ctx, uid, readData.ConvID)
	if err != nil {
		return err
	}
	if !member {
		return errs.ErrNotMember
	}

	// 3.查询这条消息是否存在：
	msg, err := dao.GetMessageByID(ctx, readData.MsgID)
	if err != nil {
		return err
	}
	// 4.校验消息是否真的属于这个会话：
	if msg.ConvID != readData.ConvID {
		return errs.ErrInvalidMessage
	}

	// 5.最后更新当前用户的已读状态：
	return dao.UpdateLastRead(ctx, uid, readData.ConvID, readData.MsgID)
}
