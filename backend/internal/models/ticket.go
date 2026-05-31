package models

import (
	"time"

	"gorm.io/gorm"
)

type Contact struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:255" json:"name"`
	Phone     string         `gorm:"size:50;index" json:"phone"`
	Email     string         `gorm:"size:255;index" json:"email"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
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
	Stage       TicketStage    `gorm:"type:varchar(50);default:'new';index" json:"stage"`
	Status      string         `gorm:"size:50;default:'open'" json:"status"` // open/closed
	AssignedTo  *uint          `gorm:"index" json:"assigned_to,omitempty"`
	Assignee    *User          `gorm:"foreignKey:AssignedTo" json:"assignee,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Messages []TicketMessage `json:"messages,omitempty"`
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
