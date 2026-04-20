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
