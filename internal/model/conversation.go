package model

type Conversation struct {
	ConvId  uint64 `gorm:"" json:"conv_id"`
	Type    string `gorm:"size:2" json:"type"`
	PeerKey uint64 `gorm:"" json:"peer_key"`
}

func (Conversation) TableName() string { return "conversations" }
