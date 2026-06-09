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

// Ticket origin discriminator. "internal" tickets are Obertrack-generated
// alerts (e.g. work-hour rejections) stored locally; "zoho" is set on the DTO
// for tickets fetched live from Zoho Desk (never persisted).
const (
	OriginInternal = "internal"
	OriginZoho     = "zoho"
)

type Ticket struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ContactID   *uint          `gorm:"index" json:"contact_id,omitempty"`
	Contact     *Contact       `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	UserID      *uint          `gorm:"index" json:"user_id,omitempty"` // Professional referenced by an internal alert
	Origin      string         `gorm:"size:20;index" json:"origin"` // "internal" = Obertrack alert; channel-tagged otherwise
	Title       string         `gorm:"size:255" json:"title"`
	Description string         `gorm:"type:text" json:"description,omitempty"` // Internal alert body (reason/dates)

	// Denormalized fields for internal work-hour-rejection alerts (follow-up + report).
	ProfessionalEmail string `gorm:"size:255" json:"professional_email,omitempty"`
	ProfessionalPhone string `gorm:"size:50" json:"professional_phone,omitempty"`
	CompanyName       string `gorm:"size:255" json:"company_name,omitempty"` // Professional's employer company
	RejectedByName    string `gorm:"size:255" json:"rejected_by_name,omitempty"`
	Reason            string `gorm:"type:text" json:"reason,omitempty"`
	WorkDates         string `gorm:"size:255" json:"work_dates,omitempty"`
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

// TicketTransfer is an append-only audit record of a ticket reassignment,
// shared by both Zoho and internal tickets (discriminated by Origin).
type TicketTransfer struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Origin      string    `gorm:"size:20;index" json:"origin"` // internal | zoho
	TicketRef   string    `gorm:"size:64;index" json:"ticket_ref"` // zoho ticket id, or internal ticket id
	TicketTitle string    `gorm:"size:255" json:"ticket_title"`
	FromUserID  *uint     `gorm:"index" json:"from_user_id,omitempty"`
	FromName    string    `gorm:"size:255" json:"from_name"`
	ToUserID    *uint     `gorm:"index" json:"to_user_id,omitempty"`
	ToName      string    `gorm:"size:255" json:"to_name"`
	ByUserID    uint      `gorm:"index" json:"by_user_id"`
	ByName      string    `gorm:"size:255" json:"by_name"`
	Reason      string    `gorm:"type:text" json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (TicketTransfer) TableName() string {
	return "ticket_transfers"
}
