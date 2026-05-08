package dao

import (
	"context"
	"errors"

	"BlahajChatServer/internal/model"

	"gorm.io/gorm"
)

func CreateUser(u *model.User) error {
	return DB.Create(u).Error
}

func CreateUserWithCtx(ctx context.Context, u *model.User) error {
	return DB.WithContext(ctx).Create(u).Error
}

func GetUserByEmail(email string) (*model.User, error) {
	var u model.User
	if err := DB.Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func GetUserByEmailWithCtx(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	if err := DB.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func GetUserByID(id uint64) (*model.User, error) {
	var u model.User
	if err := DB.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func GetUserByIDWithCtx(ctx context.Context, id uint64) (*model.User, error) {
	// 1. 带 ctx 的按 ID 查询，给 service 层请求链路使用。
	var u model.User
	if err := DB.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func GetUsersByIDs(ctx context.Context, ids []uint64) (map[uint64]model.User, error) {
	// 1. 空列表直接返回空 map，避免生成无意义的 IN () 查询。
	if len(ids) == 0 {
		return map[uint64]model.User{}, nil
	}

	// 2. 一次性查出所有用户，避免好友列表 N+1 查询。
	var users []model.User
	if err := DB.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}

	// 3. 转成 map，方便 service 按好友 uid 快速取用户资料。
	userByID := make(map[uint64]model.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}
	return userByID, nil
}
