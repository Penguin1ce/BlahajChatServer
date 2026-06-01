package dao

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/redis"
	"BlahajChatServer/pkg/consts"

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
	// 1. 先查缓存。注意缓存里的 User 不含密码（json:"-"），仅供展示/存在性校验，
	//    需要密码的鉴权路径走 GetUserByEmail* 而非这里。
	if cached, ok, _ := redis.GetCache(userProfileKey(id)); ok {
		var u model.User
		if json.Unmarshal([]byte(cached), &u) == nil {
			return &u, nil
		}
	}

	// 2. 未命中查 DB，记录不存在返回 (nil, nil)，不回写缓存。
	var u model.User
	if err := DB.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// 3. 回写缓存。
	cacheUser(u)
	return &u, nil
}

func GetUsersByIDs(ctx context.Context, ids []uint64) (map[uint64]model.User, error) {
	// 1. 空列表直接返回空 map，避免生成无意义的 IN () 查询。
	if len(ids) == 0 {
		return map[uint64]model.User{}, nil
	}

	// 2. 逐个查缓存，未命中的收集起来一次性回表（仍是单条 IN 查询，不会退化成 N+1）。
	userByID := make(map[uint64]model.User, len(ids))
	miss := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if cached, ok, _ := redis.GetCache(userProfileKey(id)); ok {
			var u model.User
			if json.Unmarshal([]byte(cached), &u) == nil {
				userByID[id] = u
				continue
			}
		}
		miss = append(miss, id)
	}
	if len(miss) == 0 {
		return userByID, nil
	}

	// 3. 未命中的一次性查出来并回写缓存。
	var users []model.User
	if err := DB.WithContext(ctx).Where("id IN ?", miss).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		userByID[user.ID] = user
		cacheUser(user)
	}
	return userByID, nil
}

func userProfileKey(uid uint64) string {
	return consts.UserProfileKey + strconv.FormatUint(uid, 10)
}

// cacheUser 把用户资料写入缓存。User 的 Password 带 json:"-"，序列化时自动剔除。
func cacheUser(u model.User) {
	if raw, err := json.Marshal(u); err == nil {
		_ = redis.SetValueByKeyExpire(userProfileKey(u.ID), string(raw), consts.UserProfileTTL)
	}
}

// InvalidateUser 在用户资料变更（改昵称/头像等）后调用删缓存。
// 当前还没有"修改资料"写路径，先备好；新增该功能时务必在写库后调用本函数。
func InvalidateUser(uid uint64) {
	redis.DelValueByKey(userProfileKey(uid))
}

func SearchUsers(ctx context.Context, keyword string, limit int) ([]model.User, error) {
	if keyword == "" {
		return []model.User{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	like := "%" + keyword + "%"
	q := DB.WithContext(ctx).
		Where("email LIKE ? OR nickname LIKE ?", like, like).
		Order("id DESC").
		Limit(limit)

	if uid, err := strconv.ParseUint(keyword, 10, 64); err == nil {
		q = q.Or("id = ?", uid)
	}

	var users []model.User
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
