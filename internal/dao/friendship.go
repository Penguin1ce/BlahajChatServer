package dao

import (
	"context"
	"errors"

	"BlahajChatServer/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NormalizeFriendPair(uidA, uidB uint64) (uint64, uint64) {
	// 1. 好友关系是无方向的，统一把较小 uid 放前面，避免 A-B / B-A 两条重复记录。
	if uidA > uidB {
		uidA, uidB = uidB, uidA
	}
	return uidA, uidB
}

func IsFriend(ctx context.Context, uidA, uidB uint64) (bool, error) {
	// 1. 先归一化好友对，保证查询条件和唯一索引一致。
	uidA, uidB = NormalizeFriendPair(uidA, uidB)
	var count int64

	// 2. 只把 status=normal 的关系视为当前好友。
	err := DB.WithContext(ctx).
		Model(&model.Friendship{}).
		Where("uid_a = ? AND uid_b = ? AND status = ?", uidA, uidB, model.FriendshipStatusNormal).
		Count(&count).Error
	return count > 0, err
}

func CreateFriendshipTx(ctx context.Context, tx *gorm.DB, uidA, uidB uint64) error {
	// 1. 统一好友对顺序，保证 uid_a + uid_b 唯一键能正确去重。
	uidA, uidB = NormalizeFriendPair(uidA, uidB)
	friendship := model.Friendship{
		UIDA:   uidA,
		UIDB:   uidB,
		Status: model.FriendshipStatusNormal,
	}

	// 2. 使用 OnConflict 做 upsert：
	//    - 没有这对好友：INSERT
	//    - 已有这对好友：把 status 恢复为 normal，并刷新 updated_at
	return useDB(ctx, tx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "uid_a"}, {Name: "uid_b"}},
			DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at"}),
		}).
		Create(&friendship).Error
}

func ListFriendshipsByUID(ctx context.Context, uid uint64) ([]model.Friendship, error) {
	// 1. 好友关系无方向，所以当前用户可能在 uid_a 或 uid_b。
	var friendships []model.Friendship
	err := DB.WithContext(ctx).
		Where("(uid_a = ? OR uid_b = ?) AND status = ?", uid, uid, model.FriendshipStatusNormal).
		Order("updated_at DESC").
		Find(&friendships).Error
	return friendships, err
}

func GetFriendship(ctx context.Context, uidA, uidB uint64) (*model.Friendship, error) {
	// 1. 查询单条好友关系前，也要先归一化好友对。
	uidA, uidB = NormalizeFriendPair(uidA, uidB)
	var friendship model.Friendship

	// 2. 查不到返回 nil，由 service 决定后续业务语义。
	if err := DB.WithContext(ctx).
		Where("uid_a = ? AND uid_b = ?", uidA, uidB).
		First(&friendship).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &friendship, nil
}
