package service

import (
	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"context"
	"sort"
)

func GetConversationListByID(ctx context.Context, uid uint64) ([]response.ConversationListResp, error) {
	// uid 为当前用户的id，根据id去查用户的conversationList用于界面的展示
	userConvs, err := dao.GetUserConvsByUID(ctx, uid)
	if err != nil {
		return nil, err
	}

	convIDs := make([]string, 0, len(userConvs))
	for _, userConv := range userConvs {
		convIDs = append(convIDs, userConv.ConvID)
	}

	convByID, err := dao.GetConvsByIDs(ctx, convIDs)
	if err != nil {
		return nil, err
	}

	res := userConvToConversationList(userConvs, convByID)
	return res, nil
}

func userConvToConversationList(userConvs []model.UserConv, convByID map[string]model.Conversation) []response.ConversationListResp {
	resp := make([]response.ConversationListResp, 0, len(userConvs))
	for _, userConv := range userConvs {
		conv, ok := convByID[userConv.ConvID]
		if !ok {
			continue
		}

		lastMsgAt := int64(0)
		if !conv.LastMsgAt.IsZero() {
			lastMsgAt = conv.LastMsgAt.UnixMilli()
		}

		item := response.ConversationListResp{
			ConvID:        conv.ConvId,
			Type:          conv.Type,
			Name:          conv.Name,
			Avatar:        conv.Avatar,
			OwnerID:       conv.OwnerID,
			LastMsgID:     conv.LastMsgID,
			LastMsgAt:     lastMsgAt,
			LastReadMsgID: userConv.LastReadMsgID,
			Unread:        userConv.Unread,
			Pinned:        userConv.Pinned,
			Muted:         userConv.Muted,
		}
		if conv.PeerKey != nil {
			item.PeerKey = *conv.PeerKey
		}
		resp = append(resp, item)
	}

	sort.SliceStable(resp, func(i, j int) bool {
		if resp[i].Pinned != resp[j].Pinned {
			return resp[i].Pinned
		}
		return resp[i].LastMsgAt > resp[j].LastMsgAt
	})

	return resp
}
