package models

import "time"

// Estados de gestión de un seguimiento de customer success.
const (
	FollowUpContacted = "contacted" // Ya se contactó al profesional
	FollowUpJustified = "justified" // La inactividad/ausencia está justificada
	FollowUpEscalated = "escalated" // Escalado (manager / empresa)
)

func IsValidFollowUpStatus(status string) bool {
	return status == FollowUpContacted || status == FollowUpJustified || status == FollowUpEscalated
}

// Tipos de seguimiento.
const (
	FollowUpKindInactivity = "inactivity"
	FollowUpKindAbsence    = "absence"
)

func IsValidFollowUpKind(kind string) bool {
	return kind == FollowUpKindInactivity || kind == FollowUpKindAbsence
}

// FollowUp es una entrada de la bitácora de gestión de customer success sobre
// un profesional (inactividad o ausencias). La entrada más reciente por
// (user, kind) es el estado vigente; el historial completo queda en la tabla.
type FollowUp struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_followups_user_kind" json:"user_id"`
	Kind      string    `gorm:"size:20;not null;index:idx_followups_user_kind" json:"kind"`
	Status    string    `gorm:"size:20;not null" json:"status"`
	Note      string    `gorm:"type:text" json:"note"`
	CreatedBy uint      `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (FollowUp) TableName() string {
	return "follow_ups"
}
