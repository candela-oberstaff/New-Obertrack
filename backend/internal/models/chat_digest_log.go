package models

import "time"

// ChatDigestLog registraba el último correo de "tienes mensajes sin leer"
// enviado a cada usuario. Ese correo se RETIRÓ del sistema (2026-08-27) y su
// tabla se elimina en la migración 202608271200_drop_chat_digest; el struct
// solo permanece porque las migraciones históricas lo referencian.
type ChatDigestLog struct {
	UserID uint      `gorm:"primaryKey" json:"user_id"`
	SentAt time.Time `json:"sent_at"`
}

func (ChatDigestLog) TableName() string {
	return "chat_digest_logs"
}
