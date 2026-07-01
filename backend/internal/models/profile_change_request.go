package models

import "time"

const (
	ProfileChangePending  = "pending"
	ProfileChangeApplied  = "applied"
	ProfileChangeRejected = "rejected"
)

var ProfileLockedFields = []string{
	"name", "phone_number", "country", "state", "city", "location", "job_title", "identity_document",
}

type ProfileChangeRequest struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"index;not null" json:"user_id"`
	SupportTicketID *uint      `gorm:"index" json:"support_ticket_id,omitempty"`
	Changes         string     `gorm:"type:text" json:"changes"`
	Note            string     `gorm:"type:text" json:"note"`
	Status          string     `gorm:"size:20;not null;default:'pending'" json:"status"`
	ReviewedBy      *uint      `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (ProfileChangeRequest) TableName() string {
	return "profile_change_requests"
}
