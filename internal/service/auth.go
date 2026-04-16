package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"BlahajChatServer/config"
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/model"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken         = errors.New("邮箱已被注册")
	ErrInvalidCredentials = errors.New("邮箱或密码错误")
	ErrInvalidRefresh     = errors.New("refresh token 无效或已过期")
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func refreshKey(token string) string { return "refresh:" + token }
func blacklistKey(jti string) string { return "blacklist:" + jti }

func Register(ctx context.Context, email, password, nickname string) (*model.User, error) {
	exist, err := dao.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{Email: email, Password: string(hash), Nickname: nickname}
	if err := dao.CreateUser(u); err != nil {
		return nil, err
	}
	return u, nil
}

func Login(ctx context.Context, email, password string) (*model.User, *TokenPair, error) {
	u, err := dao.GetUserByEmail(email)
	if err != nil {
		return nil, nil, err
	}
	if u == nil {
		return nil, nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	tp, err := issueTokenPair(ctx, u.ID)
	if err != nil {
		return nil, nil, err
	}
	return u, tp, nil
}

func Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	uidStr, err := dao.RDB.Get(ctx, refreshKey(refreshToken)).Result()
	if err == redis.Nil {
		return nil, ErrInvalidRefresh
	} else if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	// 轮换：删旧发新
	dao.RDB.Del(ctx, refreshKey(refreshToken))
	return issueTokenPair(ctx, uid)
}

func Logout(ctx context.Context, refreshToken, accessJTI string, accessExp time.Time) error {
	if refreshToken != "" {
		dao.RDB.Del(ctx, refreshKey(refreshToken))
	}
	if accessJTI != "" {
		ttl := time.Until(accessExp)
		if ttl > 0 {
			dao.RDB.Set(ctx, blacklistKey(accessJTI), 1, ttl)
		}
	}
	return nil
}

func IsAccessBlacklisted(ctx context.Context, jti string) bool {
	if jti == "" {
		return false
	}
	n, _ := dao.RDB.Exists(ctx, blacklistKey(jti)).Result()
	return n > 0
}

func issueTokenPair(ctx context.Context, userID uint64) (*TokenPair, error) {
	access, _, err := GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	refresh := randomToken(32)
	ttl := time.Duration(config.CFG.JWT.RefreshTTLDays) * 24 * time.Hour
	if err := dao.RDB.Set(ctx, refreshKey(refresh), userID, ttl).Err(); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(config.CFG.JWT.AccessTTLMinutes) * 60,
	}, nil
}
