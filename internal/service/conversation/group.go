package conversation

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func CreateGroup(ctx context.Context, ownerUID uint64, req requests.CreateGroupReq) (*model.Conversation, error) {
	name := strings.TrimSpace(req.Name)
	avatar := strings.TrimSpace(req.Avatar)
	members, err := normalizeGroupMembers(ownerUID, req.MemberUIDs, true)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errs.ErrInvalidGroup
	}
	if err := ensureUsersExist(ctx, members); err != nil {
		return nil, err
	}

	now := time.Now()
	conv := &model.Conversation{
		ConvId:    uuid.NewString(),
		Type:      model.ConvTypeGroup,
		Name:      name,
		Avatar:    avatar,
		OwnerID:   ownerUID,
		LastMsgAt: now,
	}
	rawMembers, err := json.Marshal(members)
	if err != nil {
		return nil, err
	}
	groupInfo := &model.GroupInfo{
		ConvID:      conv.ConvId,
		Name:        name,
		Avatar:      avatar,
		OwnerID:     ownerUID,
		Members:     json.RawMessage(rawMembers),
		MemberCount: uint32(len(members)),
	}

	err = dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := dao.CreateConversationTx(ctx, tx, conv); err != nil {
			return err
		}
		if err := dao.CreateGroupInfoTx(ctx, tx, groupInfo); err != nil {
			return err
		}
		return dao.CreateConversationStatesTx(ctx, tx, statesForMembers(conv.ConvId, members))
	})
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func ListGroupMembers(ctx context.Context, uid uint64, convID string) (*response.GroupMemberListResp, error) {
	member, err := dao.IsMember(ctx, uid, convID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, errs.ErrNotMember
	}

	groupInfo, err := dao.GetGroupInfoByConvID(ctx, convID)
	if err != nil {
		return nil, err
	}
	members, err := decodeGroupMembers(groupInfo.Members)
	if err != nil {
		return nil, err
	}
	userByID, err := dao.GetUsersByIDs(ctx, members)
	if err != nil {
		return nil, err
	}

	items := make([]response.GroupMemberResp, 0, len(members))
	for _, memberUID := range members {
		user, ok := userByID[memberUID]
		if !ok {
			continue
		}
		items = append(items, response.GroupMemberResp{
			UID:       user.ID,
			Email:     user.Email,
			Nickname:  user.Nickname,
			AvatarURL: user.AvatarURL,
			Owner:     user.ID == groupInfo.OwnerID,
		})
	}
	return &response.GroupMemberListResp{Items: items}, nil
}

func AddGroupMembers(ctx context.Context, operatorUID uint64, convID string, req requests.AddGroupMembersReq) (*response.GroupMemberListResp, error) {
	candidates, err := normalizeGroupMembers(0, req.MemberUIDs, false)
	if err != nil {
		return nil, err
	}

	err = dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		groupInfo, err := dao.GetGroupInfoByConvIDForUpdateTx(ctx, tx, convID)
		if err != nil {
			return err
		}
		if groupInfo.OwnerID != operatorUID {
			return errs.ErrNoPermission
		}
		if err := ensureUsersExist(ctx, candidates); err != nil {
			return err
		}
		current, err := decodeGroupMembers(groupInfo.Members)
		if err != nil {
			return err
		}
		merged, added := mergeMembers(current, candidates)
		if !added {
			return nil
		}
		if err := dao.UpdateGroupMembersTx(ctx, tx, convID, merged); err != nil {
			return err
		}
		return dao.EnsureConversationStatesTx(ctx, tx, statesForMembers(convID, candidates))
	})
	if err != nil {
		return nil, err
	}
	return ListGroupMembers(ctx, operatorUID, convID)
}

func LeaveGroup(ctx context.Context, uid uint64, convID string) error {
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		groupInfo, err := dao.GetGroupInfoByConvIDForUpdateTx(ctx, tx, convID)
		if err != nil {
			return err
		}
		if groupInfo.OwnerID == uid {
			return errs.ErrGroupOwnerLeave
		}
		current, err := decodeGroupMembers(groupInfo.Members)
		if err != nil {
			return err
		}
		next, removed := removeMember(current, uid)
		if !removed {
			return errs.ErrNotMember
		}
		if err := dao.UpdateGroupMembersTx(ctx, tx, convID, next); err != nil {
			return err
		}
		return dao.DeleteConversationStateTx(ctx, tx, uid, convID)
	})
}

func normalizeGroupMembers(ownerUID uint64, input []uint64, requireOther bool) ([]uint64, error) {
	seen := make(map[uint64]struct{}, len(input)+1)
	members := make([]uint64, 0, len(input)+1)
	add := func(uid uint64) {
		if uid == 0 {
			return
		}
		if _, ok := seen[uid]; ok {
			return
		}
		seen[uid] = struct{}{}
		members = append(members, uid)
	}

	add(ownerUID)
	for _, uid := range input {
		add(uid)
	}
	if requireOther && len(members) < 2 {
		return nil, errs.ErrInvalidGroup
	}
	if !requireOther && len(members) == 0 {
		return nil, errs.ErrInvalidGroup
	}
	sort.Slice(members, func(i, j int) bool { return members[i] < members[j] })
	return members, nil
}

func ensureUsersExist(ctx context.Context, uids []uint64) error {
	userByID, err := dao.GetUsersByIDs(ctx, uids)
	if err != nil {
		return err
	}
	if len(userByID) != len(uids) {
		return errs.ErrGroupMemberNotFound
	}
	return nil
}

func decodeGroupMembers(raw json.RawMessage) ([]uint64, error) {
	var members []uint64
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func statesForMembers(convID string, members []uint64) []model.ConversationState {
	states := make([]model.ConversationState, 0, len(members))
	for _, uid := range members {
		states = append(states, model.ConversationState{UID: uid, ConvID: convID})
	}
	return states
}

func mergeMembers(current, candidates []uint64) ([]uint64, bool) {
	seen := make(map[uint64]struct{}, len(current)+len(candidates))
	merged := make([]uint64, 0, len(current)+len(candidates))
	for _, uid := range current {
		if uid == 0 {
			continue
		}
		seen[uid] = struct{}{}
		merged = append(merged, uid)
	}
	added := false
	for _, uid := range candidates {
		if uid == 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		merged = append(merged, uid)
		added = true
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })
	return merged, added
}

func removeMember(current []uint64, uid uint64) ([]uint64, bool) {
	next := make([]uint64, 0, len(current))
	removed := false
	for _, memberUID := range current {
		if memberUID == uid {
			removed = true
			continue
		}
		next = append(next, memberUID)
	}
	return next, removed
}
