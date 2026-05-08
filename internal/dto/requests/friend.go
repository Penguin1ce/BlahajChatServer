package requests

type ApplyFriendReq struct {
	ToUID  uint64 `json:"to_uid" binding:"required"`
	Reason string `json:"reason"`
}
