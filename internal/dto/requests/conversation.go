package requests

type GetOrCreateC2CReq struct {
	PeerUID uint64 `json:"peer_uid" binding:"required"`
}

type CreateGroupReq struct {
	Name       string   `json:"name" binding:"required,max=64"`
	Avatar     string   `json:"avatar" binding:"max=255"`
	MemberUIDs []uint64 `json:"member_uids"`
}

type AddGroupMembersReq struct {
	MemberUIDs []uint64 `json:"member_uids"`
}
