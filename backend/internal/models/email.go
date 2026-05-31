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
	ID              uint           `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	TemplateID      uint           `json:"template_id"`
	Template        EmailTemplate  `gorm:"foreignKey:TemplateID" json:"template"`
	Title           string         `json:"title"`
	Subject         string         `json:"subject"`
	Status          string         `json:"status"` // 'draft' | 'scheduled' | 'sent'
	Recipients      int            `json:"recipients"` // Total count
	RecipientList   string         `json:"recipient_list"` // JSON array or comma separated IDs
	SentAt          *time.Time     `json:"sent_at"`
	ScheduledAt     *time.Time     `json:"scheduled_at"`
	OpenRate        float64        `json:"open_rate"`
	ClickRate       float64        `json:"click_rate"`
	CreatedBy       uint           `json:"created_by"`
	BrevoCampaignID int64          `json:"brevo_campaign_id"` // To link with webhooks
}

type EmailEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	CampaignID uint      `json:"campaign_id" gorm:"index"`
	Email      string    `json:"email" gorm:"index"`
	Event      string    `json:"event"` // 'request', 'delivered', 'opened', 'click', 'unique_opened', 'invalid_email', 'deferred', 'hard_bounce', 'soft_bounce', 'spam', 'unsubscribed', 'blocked', 'error'
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Timestamp  time.Time `json:"timestamp"`
}
