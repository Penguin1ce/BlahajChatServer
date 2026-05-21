package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetConvByID 根据ConvId查询会话
func GetConvByID(ctx context.Context, convID string) (*model.Conversation, error) {
	var conv model.Conversation
	if err := DB.WithContext(ctx).Where("conv_id = ?", convID).First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrConvNotFound
		}
		return nil, err
	}
	return &conv, nil
}

func GetConvsByIDs(ctx context.Context, convIDs []string) (map[string]model.Conversation, error) {
	if len(convIDs) == 0 {
		return map[string]model.Conversation{}, nil
	}

	var convs []model.Conversation
	if err := DB.WithContext(ctx).Where("conv_id IN ?", convIDs).Find(&convs).Error; err != nil {
		return nil, err
	}

	convByID := make(map[string]model.Conversation, len(convs))
	for _, conv := range convs {
		convByID[conv.ConvId] = conv
	}
	return convByID, nil
}

func CreateConversationTx(ctx context.Context, tx *gorm.DB, conv *model.Conversation) error {
	return useDB(ctx, tx).Create(conv).Error
}

func GetOrCreateC2C(ctx context.Context, uidA, uidB uint64) (*model.Conversation, error) {
	// 1. 非事务场景直接委托给事务版本；tx=nil 时内部会自己开启事务。
	return GetOrCreateC2CTx(ctx, nil, uidA, uidB)
}

func GetOrCreateC2CTx(ctx context.Context, tx *gorm.DB, uidA, uidB uint64) (*model.Conversation, error) {
	// 1. 基础参数校验：C2C 必须是两个不同的有效用户。
	if uidA == 0 || uidB == 0 || uidA == uidB {
		return nil, errs.ErrFoundC2CPair
	}

	// 2. peer_key 使用排序后的 uid，保证 A-B 和 B-A 命中同一个单聊。
	peerKey := makePeerKey(uidA, uidB)
	var conv model.Conversation
	db := useDB(ctx, tx)

	// 3. 快路径：会话已经存在时直接返回，并补齐双方 conversation_state。
	err := db.Where("peer_key = ?", peerKey).First(&conv).Error
	if err == nil {
		if err := EnsureConversationStatesTx(ctx, tx, []model.ConversationState{
			{UID: uidA, ConvID: conv.ConvId},
			{UID: uidB, ConvID: conv.ConvId},
		}); err != nil {
			return nil, err
		}
		return &conv, nil
	}

	// 4. 不是“未找到”的数据库错误直接返回。
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 5. 慢路径：会话不存在，准备创建 conversations + 两行 conversation_state。
	newConv := model.Conversation{
		ConvId:    uuid.NewString(),
		Type:      model.ConvTypeC2C,
		PeerKey:   &peerKey,
		LastMsgAt: time.Now(),
	}
	create := func(tx *gorm.DB) error {
		// 5.1 先创建 C2C 会话本体。
		if err := useDB(ctx, tx).Create(&newConv).Error; err != nil {
			// 5.2 并发创建时可能撞 peer_key 唯一键；撞了就捡现成会话并补状态。
			var existing model.Conversation
			if e := useDB(ctx, tx).Where("peer_key = ?", peerKey).First(&existing).Error; e == nil {
				newConv = existing
				return EnsureConversationStatesTx(ctx, tx, []model.ConversationState{
					{UID: uidA, ConvID: newConv.ConvId},
					{UID: uidB, ConvID: newConv.ConvId},
				})
			}
			return err
		}

		// 5.3 新会话创建成功后，给双方各建一条个人会话状态。
		states := []model.ConversationState{
			{UID: uidA, ConvID: newConv.ConvId},
			{UID: uidB, ConvID: newConv.ConvId},
		}
		return CreateConversationStatesTx(ctx, tx, states)
	}

	// 6. 如果外层已经有事务，就复用外层事务；否则这里自己开事务。
	if tx != nil {
		err = create(tx)
	} else {
		err = DB.WithContext(ctx).Transaction(create)
	}
	if err != nil {
		return nil, err
	}
	return &newConv, nil
}

// UpdateLastMsg 把会话的最后一条消息 ID 和时间刷成新的，当前逻辑是只允许更新的更新
// 由 chat.HandleSend 在落库 messages 后调用。
func UpdateLastMsg(ctx context.Context, convID, msgID string, ts time.Time) error {
	return UpdateLastMsgTx(ctx, nil, convID, msgID, ts)
}

func UpdateLastMsgTx(ctx context.Context, tx *gorm.DB, convID, msgID string, ts time.Time) error {
	return useDB(ctx, tx).
		Model(&model.Conversation{}).
		Where("conv_id = ? AND last_msg_at <= ?", convID, ts). // ← 守卫
		Updates(map[string]any{
			"last_msg_id": msgID,
			"last_msg_at": ts,
		}).Error
}

func makePeerKey(uidA, uidB uint64) string {
	// 1. 统一 uid 顺序，让同一对用户永远生成同一个 peer_key。
	if uidA > uidB {
		uidA, uidB = uidB, uidA
	}
	return fmt.Sprintf("%d_%d", uidA, uidB)
}
