package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	IncidentStatusOpen   = "open"
	IncidentStatusClosed = "closed"

	IncidentResponsePendiente    = "pendiente"
	IncidentResponseContactado   = "contactado"
	IncidentResponseOk           = "ok"
	IncidentResponseSinRespuesta = "sin_respuesta"
)

type Incident struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	Kind        string         `gorm:"size:50" json:"kind"`
	Country     string         `gorm:"size:100" json:"country"`
	State       string         `gorm:"size:100" json:"state"`
	Status      string         `gorm:"size:20;not null;default:'open'" json:"status"`
	CreatedBy   uint           `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	ClosedAt    *time.Time     `json:"closed_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Incident) TableName() string {
	return "incidents"
}

type IncidentResponse struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	IncidentID uint      `gorm:"index;not null;uniqueIndex:idx_incident_user" json:"incident_id"`
	UserID     uint      `gorm:"index;not null;uniqueIndex:idx_incident_user" json:"user_id"`
	Status     string    `gorm:"size:20;not null;default:'pendiente'" json:"status"`
	Note       string    `gorm:"type:text" json:"note"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (IncidentResponse) TableName() string {
	return "incident_responses"
}

func IsValidIncidentResponseStatus(s string) bool {
	switch s {
	case IncidentResponsePendiente, IncidentResponseContactado, IncidentResponseOk, IncidentResponseSinRespuesta:
		return true
	}
	return false
}
