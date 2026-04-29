package requests

type GetOrCreateC2CReq struct {
	PeerUID uint64 `json:"peer_uid" binding:"required"`
}
