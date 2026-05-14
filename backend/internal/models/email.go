package models

import (
	"time"

	"gorm.io/gorm"
)

type EmailTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Title      string         `json:"title"`
	Subject    string         `json:"subject"`
	Content    string         `json:"content" gorm:"type:text"` // JSON blocks
	Type       string         `json:"type"`       // 'campaign' | 'transactional'
	IsActive   bool           `json:"is_active" gorm:"default:true"`
	CreatedBy  uint           `json:"created_by"`
}

type EmailCampaign struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	TemplateID uint           `json:"template_id"`
	Template   EmailTemplate  `gorm:"foreignKey:TemplateID" json:"template"`
	Title      string         `json:"title"`
	Subject    string         `json:"subject"`
	Status     string         `json:"status"` // 'draft' | 'scheduled' | 'sent'
	Recipients int            `json:"recipients"` // Total count
	RecipientList string      `json:"recipient_list"` // JSON array or comma separated IDs
	SentAt     *time.Time     `json:"sent_at"`
	ScheduledAt *time.Time    `json:"scheduled_at"`
	OpenRate   float64        `json:"open_rate"`
	ClickRate  float64        `json:"click_rate"`
	CreatedBy  uint           `json:"created_by"`
}
