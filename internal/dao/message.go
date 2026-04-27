package dao

import (
	"context"
	"errors"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"gorm.io/gorm"
)

func CreateMessage(ctx context.Context, msg *model.Message) error {
	return CreateMessageTx(ctx, nil, msg)
}

func useDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return DB.WithContext(ctx)
}

func CreateMessageTx(ctx context.Context, tx *gorm.DB, msg *model.Message) error {
	return useDB(ctx, tx).Create(msg).Error
}

// ListByConv 按 ID 倒序拉取会话历史。beforeID 为 0 时表示从最新一条开始。
func ListByConv(ctx context.Context, convID string, beforeID uint64, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := DB.WithContext(ctx).
		Where("conv_id = ?", convID).
		Order("id DESC").
		Limit(limit)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}

	var msgs []model.Message
	if err := q.Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func GetByMsgID(ctx context.Context, msgID string) (*model.Message, error) {
	var msg model.Message
	if err := DB.WithContext(ctx).Where("msg_id = ?", msgID).First(&msg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrMsgNotFound
		}
		return nil, err
	}
	return &msg, nil
}
