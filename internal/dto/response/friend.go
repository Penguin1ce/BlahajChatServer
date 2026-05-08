package response

type FriendApplyResp struct {
	ID        uint64 `json:"id"`
	FromUID   uint64 `json:"from_uid"`
	ToUID     uint64 `json:"to_uid"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"created_at"`
}

type FriendApplyListResp struct {
	Items []FriendApplyResp `json:"items"`
}

type FriendResp struct {
	UID       uint64 `json:"uid"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type FriendListResp struct {
	Items []FriendResp `json:"items"`
}
