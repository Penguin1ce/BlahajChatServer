package dao

import (
	"context"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"gorm.io/gorm"
)

// ListMembers 返回会话内所有成员 uid，用于消息扇出。
func ListMembers(ctx context.Context, convID string) ([]uint64, error) {
	var uids []uint64
	// Pluck 是 GORM 提供的一个查询方法，用来从数据库表中只查询单个列的值，并把结果填充到一个切片里。
	err := DB.WithContext(ctx).
		Model(&model.UserConv{}).
		Where("conv_id = ?", convID).
		Pluck("uid", &uids).Error
	if err != nil {
		return nil, err
	}
	return uids, nil
}

// IsMember 判断用户是否属于某个会话。
func IsMember(ctx context.Context, uid uint64, convID string) (bool, error) {
	var count int64
	err := DB.WithContext(ctx).
		Model(&model.UserConv{}).
		Where("uid = ? AND conv_id = ?", uid, convID).
		Count(&count).Error
	return count > 0, err
}

func IncrUnreadExcept(ctx context.Context, convID string, senderUID uint64) error {
	return IncrUnreadExceptTx(ctx, nil, convID, senderUID)
}

func IncrUnreadExceptTx(ctx context.Context, tx *gorm.DB, convID string, senderUID uint64) error {
	return useDB(ctx, tx).
		Model(&model.UserConv{}).
		Where("conv_id = ? AND uid <> ?", convID, senderUID).
		Update("unread", gorm.Expr("unread + ?", 1)).Error
}

// UpdateLastRead 更新用户在某个会话的已读位置，并清空未读数。
func UpdateLastRead(ctx context.Context, uid uint64, convID, msgID string) error {
	res := DB.WithContext(ctx).
		Model(&model.UserConv{}).
		Where("uid = ? AND conv_id = ?", uid, convID).
		Updates(map[string]any{
			"last_read_msg_id": msgID,
			"unread":           0,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotMember
	}
	return nil
}
