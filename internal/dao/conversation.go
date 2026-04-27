package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetConvByID 根据ConvId查询会话
func GetConvByID(ctx context.Context, convID string) (*model.Conversation, error) {
	var conv model.Conversation
	if err := DB.WithContext(ctx).Where("conv_id = ?", convID).First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrConvNotFound
		}
		return nil, err
	}
	return &conv, nil
}

func GetOrCreateC2C(ctx context.Context, uidA, uidB uint64) (*model.Conversation, error) {
	if uidA == 0 || uidB == 0 || uidA == uidB {
		return nil, errs.ErrFoundC2CPair
	}
	peerKey := makePeerKey(uidA, uidB)
	// 如果有该对话则直接返回
	var conv model.Conversation
	err := DB.WithContext(ctx).Where("peer_key = ?", peerKey).First(&conv).Error
	if err == nil {
		return &conv, nil
	}
	// 如果不是记录没有的错误则直接返回错误
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// slow path：会话不存在，事务里建会话 + 两行 user_conv
	newConv := model.Conversation{
		ConvId:    uuid.NewString(),
		Type:      model.ConvTypeC2C,
		PeerKey:   &peerKey,
		LastMsgAt: time.Now(),
	}
	err = DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newConv).Error; err != nil {
			// 唯一键冲突：并发的另一个请求刚建好，捡现成的
			var existing model.Conversation
			if e := tx.Where("peer_key = ?", peerKey).First(&existing).Error; e == nil {
				newConv = existing
				return nil
			}
			return err
		}
		members := []model.UserConv{
			{UID: uidA, ConvID: newConv.ConvId},
			{UID: uidB, ConvID: newConv.ConvId},
		}
		return tx.Create(&members).Error
	})
	if err != nil {
		return nil, err
	}
	return &newConv, nil

}

// UpdateLastMsg 把会话的最后一条消息 ID 和时间刷成新的，当前逻辑是只允许更新的更新
// 由 service.HandleSend 在落库 messages 后调用。
func UpdateLastMsg(ctx context.Context, convID, msgID string, ts time.Time) error {
	return UpdateLastMsgTx(ctx, nil, convID, msgID, ts)
}

func UpdateLastMsgTx(ctx context.Context, tx *gorm.DB, convID, msgID string, ts time.Time) error {
	return useDB(ctx, tx).
		Model(&model.Conversation{}).
		Where("conv_id = ? AND last_msg_at <= ?", convID, ts). // ← 守卫
		Updates(map[string]any{
			"last_msg_id": msgID,
			"last_msg_at": ts,
		}).Error
}

func makePeerKey(uidA, uidB uint64) string {
	if uidA > uidB {
		uidA, uidB = uidB, uidA
	}
	return fmt.Sprintf("%d_%d", uidA, uidB)
}
