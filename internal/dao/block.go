package dao

import (
	"context"

	"BlahajChatServer/internal/model"
)

func IsBlocked(ctx context.Context, uid, blockedUID uint64) (bool, error) {
	// 1. 查询单向拉黑关系：uid 是否拉黑了 blockedUID。
	var count int64
	err := DB.WithContext(ctx).
		Model(&model.Block{}).
		Where("uid = ? AND blocked_uid = ?", uid, blockedUID).
		Count(&count).Error
	return count > 0, err
}

func IsBlockedLook(ctx context.Context, uidA, uidB uint64) (bool, error) {
	// 1. 好友申请场景需要看双向：A 拉黑 B 或 B 拉黑 A 都不能继续。
	var count int64
	err := DB.WithContext(ctx).
		Model(&model.Block{}).
		Where("(uid = ? AND blocked_uid = ?) OR (uid = ? AND blocked_uid = ?)", uidA, uidB, uidB, uidA).
		Count(&count).Error
	return count > 0, err
}
