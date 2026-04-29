package service

import (
	"encoding/json"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"
	"context"
)

func GetOrCreateC2C(ctx context.Context, uidA, uidB uint64) (*model.Conversation, error) {
	return dao.GetOrCreateC2C(ctx, uidA, uidB)
}

func GetHistoryMessageByID(ctx context.Context, uid uint64, convID string, beforeID uint64, limit int) (*response.MessageListResp, error) {
	member, err := dao.IsMember(ctx, uid, convID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, errs.ErrNotMember
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	msgs, err := dao.ListByConv(ctx, convID, beforeID, limit+1)
	if err != nil {
		return nil, err
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	items := make([]response.MessageResp, 0, len(msgs))
	var nextBeforeID uint64
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		content := json.RawMessage(msg.Content)
		if !json.Valid(content) {
			return nil, errs.ErrInvalidMessage
		}
		items = append(items, response.MessageResp{
			ID:        msg.ID,
			MsgID:     msg.MsgID,
			ConvID:    msg.ConvID,
			FromUID:   msg.FromUID,
			Type:      msg.Type,
			Content:   content,
			ReplyTo:   msg.ReplyTo,
			Status:    msg.Status,
			CreatedAt: msg.CreatedAt.UnixMilli(),
		})
	}
	if len(msgs) > 0 {
		nextBeforeID = msgs[len(msgs)-1].ID
	}

	return &response.MessageListResp{
		Items:        items,
		NextBeforeID: nextBeforeID,
		HasMore:      hasMore,
	}, nil
}
