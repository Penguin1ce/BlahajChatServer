package friend

import (
	"context"
	"errors"
	"strings"

	"BlahajChatServer/internal/dao"
	"BlahajChatServer/internal/dto/requests"
	"BlahajChatServer/internal/dto/response"
	"BlahajChatServer/internal/model"
	"BlahajChatServer/pkg/errs"

	"gorm.io/gorm"
)

func ApplyFriend(ctx context.Context, fromUID uint64, req requests.ApplyFriendReq) (*response.FriendApplyResp, error) {
	// 1. 基础参数校验：不能添加自己，也不能缺少任一方 uid。
	if fromUID == 0 || req.ToUID == 0 || fromUID == req.ToUID {
		return nil, errs.ErrCannotFriendSelf
	}

	// 2. 校验目标用户是否真实存在，避免申请发给不存在的 uid。
	toUser, err := dao.GetUserByIDWithCtx(ctx, req.ToUID)
	if err != nil {
		return nil, err
	}
	if toUser == nil {
		return nil, errs.ErrApplyUserNotFound
	}

	// 3. 检查任意方向是否存在拉黑关系；有拉黑就不允许发申请。
	blocked, err := dao.IsBlockedLook(ctx, fromUID, req.ToUID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, errs.ErrFriendBlocked
	}

	// 4. 已经是好友时直接返回，避免重复申请。
	isFriend, err := dao.IsFriend(ctx, fromUID, req.ToUID)
	if err != nil {
		return nil, err
	}
	if isFriend {
		return nil, errs.ErrAlreadyFriend
	}

	// 5. 两人之间已经有 pending 申请时，不再重复插入。
	pending, err := dao.GetPendingFriendApplyBetween(ctx, fromUID, req.ToUID)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return nil, errs.ErrFriendApplyPending
	}

	// 6. 创建一条待处理的好友申请，FromUID 永远以后端登录态为准。
	apply := &model.FriendApply{
		FromUID: fromUID,
		ToUID:   req.ToUID,
		Status:  model.FriendApplyStatusPending,
		Reason:  strings.TrimSpace(req.Reason),
	}
	if err := dao.CreateFriendApply(ctx, apply); err != nil {
		return nil, err
	}
	resp := friendApplyToResp(*apply)
	return &resp, nil
}

func ListPendingApplies(ctx context.Context, uid uint64) (*response.FriendApplyListResp, error) {
	// 1. 只查询当前用户收到的 pending 申请。
	applies, err := dao.ListPendingFriendAppliesToUID(ctx, uid)
	if err != nil {
		return nil, err
	}

	// 2. 把 model 转成对外响应 DTO，避免直接暴露数据库结构。
	items := make([]response.FriendApplyResp, 0, len(applies))
	for _, apply := range applies {
		items = append(items, friendApplyToResp(apply))
	}
	return &response.FriendApplyListResp{Items: items}, nil
}

func AcceptApply(ctx context.Context, uid uint64, applyID uint64) (*model.Conversation, error) {
	// 1. 先查询申请是否存在。
	apply, err := dao.GetFriendApplyByID(ctx, applyID)
	if err != nil {
		return nil, err
	}
	if apply == nil {
		return nil, errs.ErrFriendApplyNotFound
	}

	// 2. 只能处理 pending 状态的申请，避免重复同意。
	if apply.Status != model.FriendApplyStatusPending {
		return nil, errs.ErrFriendApplyHandled
	}

	// 3. 只有申请接收方本人可以同意。
	if apply.ToUID != uid {
		return nil, errs.ErrNoPermission
	}

	// 4. 同意前再次检查拉黑关系，防止申请发出后状态变化。
	blocked, err := dao.IsBlockedLook(ctx, apply.FromUID, apply.ToUID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, errs.ErrFriendBlocked
	}

	var conv *model.Conversation
	// 5. 同意申请是一个原子业务：改申请状态、写好友关系、创建 C2C 会话必须同成同败。
	err = dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 5.1 把好友申请从 pending 改成 accepted。
		if err := dao.UpdateFriendApplyStatusTx(ctx, tx, apply.ID, model.FriendApplyStatusAccepted); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.ErrFriendApplyHandled
			}
			return err
		}

		// 5.2 写入 friendships；如果旧记录是 deleted，会恢复成 normal。
		if err := dao.CreateFriendshipTx(ctx, tx, apply.FromUID, apply.ToUID); err != nil {
			return err
		}

		// 5.3 创建或获取双方 C2C 会话，并确保两边都有 conversation_state。
		var err error
		conv, err = dao.GetOrCreateC2CTx(ctx, tx, apply.FromUID, apply.ToUID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func RejectApply(ctx context.Context, uid uint64, applyID uint64) error {
	// 1. 查询申请是否存在。
	apply, err := dao.GetFriendApplyByID(ctx, applyID)
	if err != nil {
		return err
	}
	if apply == nil {
		return errs.ErrFriendApplyNotFound
	}

	// 2. 只能拒绝 pending 申请，已经处理过的申请不再重复处理。
	if apply.Status != model.FriendApplyStatusPending {
		return errs.ErrFriendApplyHandled
	}

	// 3. 只有申请接收方本人可以拒绝。
	if apply.ToUID != uid {
		return errs.ErrNoPermission
	}

	// 4. 把申请状态更新为 rejected。
	if err := dao.UpdateFriendApplyStatusTx(ctx, nil, apply.ID, model.FriendApplyStatusRejected); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrFriendApplyHandled
		}
		return err
	}
	return nil
}

func ListFriends(ctx context.Context, uid uint64) (*response.FriendListResp, error) {
	// 1. 查询当前用户参与的正常好友关系。
	friendships, err := dao.ListFriendshipsByUID(ctx, uid)
	if err != nil {
		return nil, err
	}

	// 2. 从无方向好友对里取出“对方”的 uid。
	friendUIDs := make([]uint64, 0, len(friendships))
	for _, friendship := range friendships {
		if friendship.UIDA == uid {
			friendUIDs = append(friendUIDs, friendship.UIDB)
		} else {
			friendUIDs = append(friendUIDs, friendship.UIDA)
		}
	}

	// 3. 批量查询好友用户信息，避免 N+1 查询。
	userByID, err := dao.GetUsersByIDs(ctx, friendUIDs)
	if err != nil {
		return nil, err
	}

	// 4. 按好友关系顺序组装响应。
	items := make([]response.FriendResp, 0, len(friendUIDs))
	for _, friendUID := range friendUIDs {
		user, ok := userByID[friendUID]
		if !ok {
			continue
		}
		items = append(items, response.FriendResp{
			UID:       user.ID,
			Email:     user.Email,
			Nickname:  user.Nickname,
			AvatarURL: user.AvatarURL,
		})
	}
	return &response.FriendListResp{Items: items}, nil
}

func friendApplyToResp(apply model.FriendApply) response.FriendApplyResp {
	return response.FriendApplyResp{
		ID:        apply.ID,
		FromUID:   apply.FromUID,
		ToUID:     apply.ToUID,
		Status:    apply.Status,
		Reason:    apply.Reason,
		CreatedAt: apply.CreatedAt.UnixMilli(),
	}
}
