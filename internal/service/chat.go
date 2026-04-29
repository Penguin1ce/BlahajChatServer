package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/wspayload"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/internal/redis"
	"BlahajChatServer/pkg/consts"
	"BlahajChatServer/pkg/errs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 1. 生成 msgID
// 2. Redis SETNX 幂等 key -> msgID
// 3. 如果 key 已存在：查旧 msgID，返回旧 MsgData
// 4. dao.IsMember 检查成员资格
// 5. MySQL 事务：
//    - CreateMessageTx
//    - UpdateLastMsgTx
//    - IncrUnreadExceptTx
// 6. 返回 MsgData

// HandleSend 返回值分别为 MsgData, 是否为新消息, 错误
func HandleSend(ctx context.Context, uid uint64, d wspayload.SendData) (*wspayload.MsgData, bool, error) {
	if err := validateSendData(d); err != nil {
		return nil, false, err
	}

	// 1. 转换为Redis字段的key
	key := sendIdemKey(uid, d.ClientMsgID)
	// 2. 当前暂时用uuid
	msgID := uuid.NewString()
	// 3. 原子操作
	ok, err := redis.SetNXValueByKeyExpire(key, msgID, consts.ClientMsgIDIdemTTL)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		// 这里说明消息已经在redis的幂等里了
		oldMsgID, err := redis.GetValueByKey(key)
		if err != nil {
			return nil, false, err
		}
		msgData, err := waitExistingMsgData(ctx, oldMsgID)
		return msgData, false, err
	}

	// 4. 检查是否是conversion的member
	member, err := dao.IsMember(ctx, uid, d.ConvID)
	if err != nil {
		redis.DelValueByKey(key)
		return nil, false, err
	}
	if !member {
		redis.DelValueByKey(key)
		return nil, false, errs.ErrNotMember
	}

	now := time.Now()
	msg := &model.Message{
		MsgID:     msgID,
		ConvID:    d.ConvID,
		CreatedAt: now,
		FromUID:   uid,
		Type:      d.Type,
		Content:   string(d.Content),
		ReplyTo:   d.ReplyTo,
		Status:    model.MsgStatusNormal,
	}

	// 5. 开启事务，在gorm的事务下，如果返回err则立即撤回
	err = dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先用事务创建message
		if err := dao.CreateMessageTx(ctx, tx, msg); err != nil {
			return err
		}
		// 更新最后一个未读消息
		if err := dao.UpdateLastMsgTx(ctx, tx, d.ConvID, msgID, now); err != nil {
			return err
		}
		// 原子更新每个人的未读消息
		return dao.IncrUnreadExceptTx(ctx, tx, d.ConvID, uid)
	})
	if err != nil {
		redis.DelValueByKey(key)
		return nil, false, err
	}

	msgData, err := messageToMsgData(msg, d.Mentions)
	return msgData, true, err
}

func sendIdemKey(uid uint64, clientMsgID string) string {
	return fmt.Sprintf("%s%d:%s", consts.ClientMessageKey, uid, clientMsgID)
}

func validateSendData(d wspayload.SendData) error {
	if d.ClientMsgID == "" || d.ConvID == "" || d.Type == "" || len(d.Content) == 0 {
		return errs.ErrInvalidMessage
	}
	switch d.Type {
	case model.MsgTypeText, model.MsgTypeImage, model.MsgTypeFile, model.MsgTypeAudio:
	default:
		return errs.ErrInvalidMessage
	}
	if !json.Valid(d.Content) {
		return errs.ErrInvalidMessage
	}
	return nil
}

func waitExistingMsgData(ctx context.Context, msgID string) (*wspayload.MsgData, error) {
	if msgID == "" {
		return nil, errs.ErrMsgNotFound
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		msg, err := dao.GetByMsgID(ctx, msgID)
		if err == nil {
			return messageToMsgData(msg, nil)
		}
		if !errors.Is(err, errs.ErrMsgNotFound) {
			return nil, err
		}
		lastErr = err
		// 第1次查询 → 没找到 → 等50ms
		// 第2次查询 → 没找到 → 等50ms
		// 第3次查询 → 没找到 → 返回 lastErr
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func messageToMsgData(msg *model.Message, mentions []uint64) (*wspayload.MsgData, error) {
	content := json.RawMessage(msg.Content)
	if !json.Valid(content) {
		return nil, errs.ErrInvalidMessage
	}

	return &wspayload.MsgData{
		MsgID:     msg.MsgID,
		ConvID:    msg.ConvID,
		FromUID:   msg.FromUID,
		Type:      msg.Type,
		Content:   content,
		ReplyTo:   msg.ReplyTo,
		Mentions:  mentions,
		CreatedAt: msg.CreatedAt.UnixMilli(),
	}, nil
}
