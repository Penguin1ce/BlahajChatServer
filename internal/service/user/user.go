package user

import (
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/service/auth"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"
	"context"
	"errors"
	"strings"

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

func Login(ctx context.Context, email, password string) (*model.User, *auth.TokenPair, error) {
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
	tp, err := auth.IssueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, tp, nil
}

func SearchUsers(ctx context.Context, currentUID uint64, keyword string, limit int) (*response.UserSearchListResp, error) {
	keyword = strings.TrimSpace(keyword)
	users, err := dao.SearchUsers(ctx, keyword, limit)
	if err != nil {
		return nil, err
	}

	items := make([]response.UserSearchResp, 0, len(users))
	for _, u := range users {
		if u.ID == currentUID {
			continue
		}
		items = append(items, response.UserSearchResp{
			UID:       u.ID,
			Email:     u.Email,
			Nickname:  u.Nickname,
			AvatarURL: u.AvatarURL,
		})
	}
	return &response.UserSearchListResp{Items: items}, nil
}
