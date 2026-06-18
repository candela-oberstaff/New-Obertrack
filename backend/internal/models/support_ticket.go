package models

import "time"

// Estados del ciclo de vida de un ticket de soporte.
const (
	SupportStatusOpen     = "open"     // recién abierto, sin responsable
	SupportStatusAssigned = "assigned" // un agente lo tomó / fue asignado
	SupportStatusResolved = "resolved" // cerrado por un agente
)

// SupportTicket es la capa de gestión sobre un canal de soporte (1 a 1 con el
// canal). Permite que Customer Success / superadmins tomen, reasignen y resuelvan
// la solicitud manteniendo continuidad (un responsable visible para todo el equipo).
type SupportTicket struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	ChannelID   uint       `gorm:"uniqueIndex;not null" json:"channel_id"`
	TenantID    uint       `gorm:"index" json:"tenant_id"`
	RequesterID uint       `gorm:"not null;index" json:"requester_id"`
	Requester   *User      `gorm:"foreignKey:RequesterID" json:"requester,omitempty"`
	Status      string     `gorm:"size:20;not null;default:'open'" json:"status"`
	AssignedTo  *uint      `gorm:"index" json:"assigned_to,omitempty"`
	Assignee    *User      `gorm:"foreignKey:AssignedTo" json:"assignee,omitempty"`
	AssignedAt  *time.Time `json:"assigned_at,omitempty"`
	ResolvedBy  *uint      `json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (SupportTicket) TableName() string {
	return "support_tickets"
}
