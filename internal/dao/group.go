package dao

import (
	"context"
	"encoding/json"
	"errors"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func CreateGroupInfoTx(ctx context.Context, tx *gorm.DB, group *model.GroupInfo) error {
	return useDB(ctx, tx).Create(group).Error
}

func GetGroupInfoByConvIDForUpdateTx(ctx context.Context, tx *gorm.DB, convID string) (*model.GroupInfo, error) {
	var groupInfo model.GroupInfo
	if err := useDB(ctx, tx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("conv_id = ?", convID).
		First(&groupInfo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrConvNotFound
		}
		return nil, err
	}
	return &groupInfo, nil
}

func UpdateGroupMembersTx(ctx context.Context, tx *gorm.DB, convID string, members []uint64) error {
	raw, err := json.Marshal(members)
	if err != nil {
		return err
	}

	res := useDB(ctx, tx).
		Model(&model.GroupInfo{}).
		Where("conv_id = ?", convID).
		Updates(map[string]any{
			"members":      json.RawMessage(raw),
			"member_count": uint32(len(members)),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrConvNotFound
	}
	return nil
}

func SearchGroups(ctx context.Context, keyword string, limit int) ([]model.GroupInfo, error) {
	if keyword == "" {
		return []model.GroupInfo{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	like := "%" + keyword + "%"
	var groups []model.GroupInfo
	if err := DB.WithContext(ctx).
		Where("conv_id = ? OR name LIKE ?", keyword, like).
		Order("updated_at DESC").
		Limit(limit).
		Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func DeleteConversationStateTx(ctx context.Context, tx *gorm.DB, uid uint64, convID string) error {
	return useDB(ctx, tx).
		Where("uid = ? AND conv_id = ?", uid, convID).
		Delete(&model.ConversationState{}).Error
}
