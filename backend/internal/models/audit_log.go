package models

import "time"

// AuditLog is an append-only record of a mutating action in the app, used by the
// superadmin audit viewer. Only metadata is stored (no request bodies).
type AuditLog struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Kind discriminates "activity" (HTTP action: who/IP/route) from "data"
	// (a row change captured at the DB layer).
	Kind       string    `gorm:"size:20;index" json:"kind"`
	ActorID    *uint     `gorm:"index" json:"actor_id,omitempty"`
	ActorEmail string    `gorm:"size:255;index" json:"actor_email"`
	ActorRole  string    `gorm:"size:50" json:"actor_role"`
	TenantID   *uint     `gorm:"index" json:"tenant_id,omitempty"`
	Action     string    `gorm:"size:100;index" json:"action"`
	Module     string    `gorm:"size:50;index" json:"module"`
	// EntityType/EntityID correlate the event with a model row (table + PK).
	EntityType string    `gorm:"size:64;index" json:"entity_type,omitempty"`
	EntityID   string    `gorm:"size:64;index" json:"entity_id,omitempty"`
	Changes    string    `gorm:"type:text" json:"changes,omitempty"` // best-effort JSON of changed fields
	Method     string    `gorm:"size:10" json:"method"`
	Path       string    `gorm:"size:255" json:"path"`
	TargetID   string    `gorm:"size:64" json:"target_id,omitempty"`
	Status     int       `gorm:"index" json:"status"`
	Success    bool      `json:"success"`
	IP         string    `gorm:"size:64" json:"ip"`
	UserAgent  string    `gorm:"size:512" json:"user_agent"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
