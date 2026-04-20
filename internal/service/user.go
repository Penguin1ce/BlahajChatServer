package service

import (
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func Register(ctx context.Context, req requests.RegisterReq) (*model.User, error) {
	user, err := dao.GetUserByEmailWithCtx(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return nil, errs.ErrEmailTaken
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New(consts.SystemError)
	}
	user = &model.User{
		Email:     req.Email,
		Nickname:  req.Nickname,
		Password:  string(hashedPassword),
		AvatarURL: consts.DefaultAvatarURL,
	}
	err = dao.CreateUserWithCtx(ctx, user)
	if err != nil {
		return nil, errors.New(consts.SystemError)
	}
	return user, nil
}

func Login(ctx context.Context, email, password string) (*model.User, *TokenPair, error) {
	user, err := dao.GetUserByEmailWithCtx(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, errs.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, nil, errs.ErrInvalidCredentials
	}
	// 生成TOKEN对
	tp, err := issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, tp, nil
}
