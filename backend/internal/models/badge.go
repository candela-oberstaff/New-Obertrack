package models

import "time"

// Origen de una insignia.
const (
	BadgeKindBlock   = "block"   // Aprobó un bloque de la inducción
	BadgeKindProgram = "program" // Completó un programa entero
	BadgeKindMerit   = "merit"   // Reconocimiento automático por cómo lo hizo
)

// UserBadge es una insignia ganada por una persona. Es PERMANENTE: vive en su
// propia tabla y no en la invitación, porque la invitación se reemplaza al
// recontratar y se reinicia desde Soporte, y un reconocimiento ganado no se
// pierde por eso. SourceKey identifica el origen ("block:12", "program:3",
// "merit:first_try:3") y, junto con el usuario, es único: reaprobar tras un
// reinicio no duplica.
type UserBadge struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"not null;index" json:"user_id"`
	Kind      string `gorm:"size:20;not null;index" json:"kind"`
	SourceKey string `gorm:"size:80;not null" json:"source_key"`
	// Título, descripción y aspecto se copian al otorgar: si el bloque se
	// renombra o se borra después, la insignia sigue diciendo lo que decía.
	Title       string  `gorm:"size:160;not null" json:"title"`
	Description string  `gorm:"type:text" json:"description"`
	Icon        string  `gorm:"size:40;not null;default:'Award'" json:"icon"`
	Color       string  `gorm:"size:20;not null;default:'orchid'" json:"color"`
	Score       float64 `gorm:"not null;default:0" json:"score"`
	// ProgramName es el programa en el que se ganó, para contexto.
	ProgramName string    `gorm:"size:160" json:"program_name"`
	EarnedAt    time.Time `gorm:"not null;index" json:"earned_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (UserBadge) TableName() string {
	return "user_badges"
}

// PendingBadge es una insignia que la persona todavía puede ganar: un bloque o
// programa de su inducción en curso. Se muestra en gris en el perfil, que
// motiva más que una lista vacía.
type PendingBadge struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// BadgeOverview es lo que ve el perfil: lo ganado y lo que falta.
type BadgeOverview struct {
	Earned  []UserBadge    `json:"earned"`
	Pending []PendingBadge `json:"pending"`
}
