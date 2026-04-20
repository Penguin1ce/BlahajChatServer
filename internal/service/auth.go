package service

import (
	redis2 "BlahajChatServer/internal/redis"
	"BlahajChatServer/pkg/errs"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"BlahajChatServer/config"

	"github.com/redis/go-redis/v9"
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

func refreshKey(token string) string   { return "refresh:" + token }
func blacklistKey(jti string) string   { return "blacklist:" + jti }
func userSessionKey(uid uint64) string { return "userSession:" + strconv.FormatUint(uid, 10) }

func Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	uidStr, err := redis2.RDB.Get(ctx, refreshKey(refreshToken)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, errs.ErrInvalidRefresh
	} else if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		return nil, errs.ErrInvalidRefresh
	}
	// 轮换：删旧发新
	redis2.RDB.Del(ctx, refreshKey(refreshToken))
	redis2.RDB.SRem(ctx, userSessionKey(uid), refreshToken)
	return issueTokenPair(ctx, uid)
}

func Logout(ctx context.Context, userID uint64, refreshToken, accessJTI string, accessExp time.Time) error {
	if refreshToken != "" {
		redis2.RDB.Del(ctx, refreshKey(refreshToken))
		redis2.RDB.SRem(ctx, userSessionKey(userID), refreshToken)
	}
	if accessJTI != "" {
		ttl := time.Until(accessExp)
		if ttl > 0 {
			redis2.RDB.Set(ctx, blacklistKey(accessJTI), 1, ttl)
		}
	}
	return nil
}

// LogoutAll 踢掉该用户的所有设备会话，同时拉黑当前 Access Token。
func LogoutAll(ctx context.Context, userID uint64, accessJTI string, accessExp time.Time) error {
	sessionKey := userSessionKey(userID)
	tokens, err := redis2.RDB.SMembers(ctx, sessionKey).Result()
	if err == nil {
		for _, t := range tokens {
			redis2.RDB.Del(ctx, refreshKey(t))
		}
		redis2.RDB.Del(ctx, sessionKey)
	}
	if accessJTI != "" {
		ttl := time.Until(accessExp)
		if ttl > 0 {
			redis2.RDB.Set(ctx, blacklistKey(accessJTI), 1, ttl)
		}
	}
	return nil
}

func IsAccessBlacklisted(ctx context.Context, jti string) bool {
	if jti == "" {
		return false
	}
	n, _ := redis2.RDB.Exists(ctx, blacklistKey(jti)).Result()
	return n > 0
}

func issueTokenPair(ctx context.Context, userID uint64) (*TokenPair, error) {
	access, _, err := GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	refresh := randomToken(32)
	ttl := time.Duration(config.GetConfig().JWT.RefreshTTLDays) * 24 * time.Hour
	if err := redis2.RDB.Set(ctx, refreshKey(refresh), userID, ttl).Err(); err != nil {
		return nil, err
	}
	redis2.RDB.SAdd(ctx, userSessionKey(userID), refresh)
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(config.GetConfig().JWT.AccessTTLMinutes) * 60,
	}, nil
}
