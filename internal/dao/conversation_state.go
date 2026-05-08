package dao

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"gorm.io/gorm"
)

// ListMembers 返回会话内所有成员 uid，用于消息扇出。
//
// 成员事实源按会话类型区分：
//   - C2C：conversations.peer_key
//   - Group：group_info.members
func ListMembers(ctx context.Context, convID string) ([]uint64, error) {
	conv, err := GetConvByID(ctx, convID)
	if err != nil {
		return nil, err
	}
	switch conv.Type {
	case model.ConvTypeC2C:
		return parseC2CMembers(conv)
	case model.ConvTypeGroup:
		return ListGroupMembers(ctx, convID)
	default:
		return nil, errs.ErrConvNotFound
	}
}

// IsMember 判断用户是否属于某个会话。
func IsMember(ctx context.Context, uid uint64, convID string) (bool, error) {
	members, err := ListMembers(ctx, convID)
	if err != nil {
		return false, err
	}
	for _, memberUID := range members {
		if memberUID == uid {
			return true, nil
		}
	}
	return false, nil
}

func parseC2CMembers(conv *model.Conversation) ([]uint64, error) {
	if conv.PeerKey == nil || *conv.PeerKey == "" {
		return nil, errs.ErrConvNotFound
	}
	parts := strings.Split(*conv.PeerKey, "_")
	if len(parts) != 2 {
		return nil, errs.ErrConvNotFound
	}
	uidA, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, errs.ErrConvNotFound
	}
	uidB, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, errs.ErrConvNotFound
	}
	return []uint64{uidA, uidB}, nil
}

func ListGroupMembers(ctx context.Context, convID string) ([]uint64, error) {
	groupInfo, err := GetGroupInfoByConvID(ctx, convID)
	if err != nil {
		return nil, err
	}
	var members []uint64
	if err := json.Unmarshal(groupInfo.Members, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func GetGroupInfoByConvID(ctx context.Context, convID string) (*model.GroupInfo, error) {
	var groupInfo model.GroupInfo
	if err := DB.WithContext(ctx).Where("conv_id = ?", convID).First(&groupInfo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrConvNotFound
		}
		return nil, err
	}
	return &groupInfo, nil
}

func IncrUnreadExcept(ctx context.Context, convID string, senderUID uint64) error {
	return IncrUnreadExceptTx(ctx, nil, convID, senderUID)
}

func IncrUnreadExceptTx(ctx context.Context, tx *gorm.DB, convID string, senderUID uint64) error {
	// 注意，"未读数 + 1"这个操作是并发安全的
	// 它是在数据库里直接执行：unread = unread + 1
	// 所以多个请求并发更新同一行时，MySQL/InnoDB 会对被更新的行加锁，更新会排队执行。
	return useDB(ctx, tx).
		Model(&model.ConversationState{}).
		Where("conv_id = ? AND uid <> ?", convID, senderUID).
		Update("unread", gorm.Expr("unread + ?", 1)).Error
}

// UpdateLastRead 更新用户在某个会话的已读位置，并清空未读数。
func UpdateLastRead(ctx context.Context, uid uint64, convID, msgID string) error {
	res := DB.WithContext(ctx).
		Model(&model.ConversationState{}).
		Where("uid = ? AND conv_id = ?", uid, convID).
		Updates(map[string]any{
			"last_read_msg_id": msgID,
			"unread":           0,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotMember
	}
	return nil
}

func GetConversationStatesByUID(ctx context.Context, uid uint64) ([]model.ConversationState, error) {
	var resp []model.ConversationState
	err := DB.WithContext(ctx).Where("uid = ?", uid).Find(&resp).Error
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func CreateConversationStatesTx(ctx context.Context, tx *gorm.DB, states []model.ConversationState) error {
	if len(states) == 0 {
		return nil
	}
	return useDB(ctx, tx).Create(&states).Error
}
