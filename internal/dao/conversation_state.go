package dao

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/redis"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListMembers 返回会话内所有成员 uid，用于消息扇出。
//
// 成员事实源按会话类型区分：
//   - C2C：conversations.peer_key
//   - Group：group_info.members
func ListMembers(ctx context.Context, convID string) ([]uint64, error) {
	// 1. 先读取会话类型，因为 C2C 和群聊的成员事实源不同。
	conv, err := GetConvByID(ctx, convID)
	if err != nil {
		return nil, err
	}

	// 2. 按会话类型分派成员读取逻辑。
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
	// 1. 复用 ListMembers，确保成员判断和 fanout 使用同一套事实源。
	members, err := ListMembers(ctx, convID)
	if err != nil {
		return false, err
	}

	// 2. 在线性成员列表里查当前 uid；群聊后续可换缓存优化。
	for _, memberUID := range members {
		if memberUID == uid {
			return true, nil
		}
	}
	return false, nil
}

func parseC2CMembers(conv *model.Conversation) ([]uint64, error) {
	// 1. C2C 必须有 peer_key，格式是 "minUID_maxUID"。
	if conv.PeerKey == nil || *conv.PeerKey == "" {
		return nil, errs.ErrConvNotFound
	}
	parts := strings.Split(*conv.PeerKey, "_")
	if len(parts) != 2 {
		return nil, errs.ErrConvNotFound
	}

	// 2. 把 peer_key 两段解析成 uid，作为这个 C2C 的成员列表。
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
	// 1. 先查缓存（发消息/扇出每次都走这里，是最热的读路径）。
	if cached, ok, _ := redis.GetCache(groupMembersKey(convID)); ok {
		var members []uint64
		if json.Unmarshal([]byte(cached), &members) == nil {
			return members, nil
		}
	}

	// 2. 未命中查 DB，群聊成员事实源是 group_info.members。
	//    ErrConvNotFound 等错误直接透出，不回写缓存（避免缓存穿透写空值）。
	groupInfo, err := GetGroupInfoByConvID(ctx, convID)
	if err != nil {
		return nil, err
	}

	// 3. members 存 JSON uid 数组，这里反序列化成 []uint64 供 fanout 使用。
	var members []uint64
	if err := json.Unmarshal(groupInfo.Members, &members); err != nil {
		return nil, err
	}

	// 4. 回写缓存，带 TTL 兜底。
	if raw, err := json.Marshal(members); err == nil {
		_ = redis.SetValueByKeyExpire(groupMembersKey(convID), string(raw), consts.GroupMembersTTL)
	}
	return members, nil
}

func groupMembersKey(convID string) string {
	return consts.GroupMembersKey + convID
}

// InvalidateGroupMembers 在群成员发生变更（加人/退群/解散等）的事务提交后调用，
// 删除成员缓存。必须在事务 commit 之后调用，提交前删会被并发读重新灌入旧数据。
func InvalidateGroupMembers(convID string) {
	redis.DelValueByKey(groupMembersKey(convID))
}

func GetGroupInfoByConvID(ctx context.Context, convID string) (*model.GroupInfo, error) {
	// 1. 按 conv_id 查群资料；找不到时对上层表现为会话不存在/不可用。
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
	// 1. 空列表直接返回，方便调用方批量构造后统一调用。
	if len(states) == 0 {
		return nil
	}

	// 2. 创建个人会话状态；这里不忽略冲突，适合“明确必须新建”的场景。
	return useDB(ctx, tx).Create(&states).Error
}

func EnsureConversationStatesTx(ctx context.Context, tx *gorm.DB, states []model.ConversationState) error {
	// 1. 空列表直接返回，保持幂等辅助函数容易调用。
	if len(states) == 0 {
		return nil
	}

	// 2. OnConflict DoNothing 表示“没有就插入，已有就跳过”，用于补齐会话状态。
	return useDB(ctx, tx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&states).Error
}
