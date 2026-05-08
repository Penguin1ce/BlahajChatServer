package dao

import (
	"context"
	"errors"
	"time"

	"BlahajChatServer/internal/model"

	"gorm.io/gorm"
)

func CreateFriendApply(ctx context.Context, apply *model.FriendApply) error {
	// 1. 插入一条 pending 好友申请；调用方负责提前做重复申请校验。
	return DB.WithContext(ctx).Create(apply).Error
}

func GetPendingFriendApplyBetween(ctx context.Context, uidA, uidB uint64) (*model.FriendApply, error) {
	// 1. 查询两人之间任意方向是否已有 pending 申请，避免 A->B 和 B->A 同时堆积。
	var apply model.FriendApply
	err := DB.WithContext(ctx).
		Where("status = ? AND ((from_uid = ? AND to_uid = ?) OR (from_uid = ? AND to_uid = ?))",
			model.FriendApplyStatusPending, uidA, uidB, uidB, uidA).
		Order("created_at DESC").
		First(&apply).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	// 2. 找到 pending 申请时返回申请本身，service 用它判断重复申请。
	return &apply, nil
}

func GetFriendApplyByID(ctx context.Context, id uint64) (*model.FriendApply, error) {
	// 1. 按主键读取申请；不存在返回 nil，方便 service 映射成业务错误。
	var apply model.FriendApply
	if err := DB.WithContext(ctx).First(&apply, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &apply, nil
}

func ListPendingFriendAppliesToUID(ctx context.Context, uid uint64) ([]model.FriendApply, error) {
	// 1. 只列出当前用户收到的待处理申请，按创建时间倒序给前端展示。
	var applies []model.FriendApply
	err := DB.WithContext(ctx).
		Where("to_uid = ? AND status = ?", uid, model.FriendApplyStatusPending).
		Order("created_at DESC").
		Find(&applies).Error
	return applies, err
}

func UpdateFriendApplyStatusTx(ctx context.Context, tx *gorm.DB, id uint64, status string) error {
	// 1. handled_at 记录本次处理时间，无论同意还是拒绝都要写。
	now := time.Now()

	// 2. 只允许从 pending 状态更新，防止重复同意/拒绝把历史状态覆盖掉。
	res := useDB(ctx, tx).
		Model(&model.FriendApply{}).
		Where("id = ? AND status = ?", id, model.FriendApplyStatusPending).
		Updates(map[string]any{
			"status":     status,
			"handled_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	// 3. RowsAffected > 0 表示申请状态已经成功流转。
	return nil
}
