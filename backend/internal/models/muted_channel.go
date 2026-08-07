package models

import "time"

// MutedChannel silencia un canal para UN usuario: sus mensajes nuevos dejan de
// sonar la campana, pero el canal sigue en la lista y los contadores de
// no-leídos funcionan igual. Es personal (no afecta a otros miembros), como
// HiddenChannel. Las MENCIONES directas siguen sonando aunque el canal esté
// silenciado: silenciar el ruido no debe silenciar lo que te nombra.
type MutedChannel struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	ChannelID uint      `gorm:"primaryKey" json:"channel_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (MutedChannel) TableName() string {
	return "muted_channels"
}
