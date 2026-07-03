package models

import "time"

// HiddenChannel marca que un usuario "cerró" (ocultó) un canal de su lista de
// chat. Es personal: no borra el canal ni afecta a otros. Un mensaje nuevo en el
// canal elimina estas filas (el canal reaparece para quien lo había ocultado).
type HiddenChannel struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	ChannelID uint      `gorm:"primaryKey" json:"channel_id"`
	HiddenAt  time.Time `json:"hidden_at"`
}

func (HiddenChannel) TableName() string {
	return "hidden_channels"
}
