package service

import (
	"errors"
	"time"

	"BlahajChatServer/config"

	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	UserID uint64 `json:"uid"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userID uint64) (string, string, error) {
	ttl := time.Duration(config.CFG.JWT.AccessTTLMinutes) * time.Minute
	jti := randomToken(16)
	claims := AccessClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(config.CFG.JWT.Secret))
	return s, jti, err
}

func ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.CFG.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*AccessClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
