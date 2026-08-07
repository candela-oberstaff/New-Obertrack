package models

import "time"

// ChatDigestLog registra el último correo de "tienes mensajes sin leer" que se
// le envió a un usuario. Existe para que el aviso no se repita en cada pasada
// del watcher: como máximo un correo por usuario por ventana (24 h), aunque
// siga sin conectarse.
type ChatDigestLog struct {
	UserID uint      `gorm:"primaryKey" json:"user_id"`
	SentAt time.Time `json:"sent_at"`
}

func (ChatDigestLog) TableName() string {
	return "chat_digest_logs"
}
