package models

import (
	"time"

	"gorm.io/gorm"
)

type Contact struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Name            string         `gorm:"size:255" json:"name"`
	Phone           string         `gorm:"size:50;index" json:"phone"`
	WaID            string         `gorm:"size:100;index" json:"wa_id"` // WhatsApp internal ID (e.g. 123@lid or 123@c.us)
	Email           string         `gorm:"size:255;index" json:"email"`
	CompanyName     string         `gorm:"size:255" json:"company_name"`     // Optional company name
	ParentContactID *uint          `gorm:"index" json:"parent_contact_id"`    // For linking secondary contacts to a primary company contact
	ParentContact   *Contact       `gorm:"foreignKey:ParentContactID" json:"parent_contact,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Contact) TableName() string {
	return "contacts"
}

type TicketStage string

const (
	StageNew        TicketStage = "new"
	StageInProgress TicketStage = "in_progress"
	StageWaiting    TicketStage = "waiting"
	StageClosed     TicketStage = "closed"
)

type Ticket struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ContactID   uint           `gorm:"not null;index" json:"contact_id"`
	Contact     Contact        `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	Title       string         `gorm:"size:255" json:"title"`
	Channel     string         `gorm:"-" json:"channel,omitempty"` // Zoho channel: WhatsApp, Email, etc.
	Stage       TicketStage    `gorm:"type:varchar(50);default:'new';index" json:"stage"`
	Status      string         `gorm:"size:50;default:'open'" json:"status"` // open/closed
	AssignedTo  *uint          `gorm:"index" json:"assigned_to,omitempty"`
	Assignee    *User          `gorm:"foreignKey:AssignedTo" json:"assignee,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Messages []TicketMessage `json:"messages,omitempty"`
	ZohoID   string          `gorm:"-" json:"zoho_id"`
	Sentiment   string         `gorm:"-" json:"sentiment,omitempty"`
}

func (Ticket) TableName() string {
	return "tickets"
}

type SenderType string

const (
	SenderTypeAgent   SenderType = "agent"
	SenderTypeContact SenderType = "contact"
	SenderTypeSystem  SenderType = "system"
)

type MessageChannel string

const (
	ChannelWhatsApp MessageChannel = "whatsapp"
	ChannelEmail    MessageChannel = "email"
	ChannelNote     MessageChannel = "note"
)

type TicketMessage struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	TicketID   uint           `gorm:"not null;index" json:"ticket_id"`
	SenderType SenderType     `gorm:"type:varchar(20);not null" json:"sender_type"`
	SenderID   *uint          `json:"sender_id,omitempty"` // UserID if SenderType == agent
	Channel    MessageChannel `gorm:"type:varchar(20);not null" json:"channel"`
	Content    string         `gorm:"type:text" json:"content"`
	ExternalID string         `gorm:"size:255;index" json:"external_id"` // WAHA ID or Brevo Message-ID
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Sender *User `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
}

func (TicketMessage) TableName() string {
	return "ticket_messages"
}
