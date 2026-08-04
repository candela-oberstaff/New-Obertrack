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
// for tickets fetched live from Zoho Desk (never persisted); "obersuite" marks
// the arrival of someone contratado en Obersuite, para que la incorporación se
// vea en la bandeja y no solo en el panel de profesionales.
const (
	OriginInternal  = "internal"
	OriginZoho      = "zoho"
	OriginObersuite = "obersuite"
)

// IsLocalOrigin distingue los tickets que viven en nuestra base de los que se
// leen en vivo de Zoho. Solo los locales admiten notas, cambio de etapa y
// reasignación desde la ficha; los de Zoho se gestionan allí.
func IsLocalOrigin(origin string) bool {
	return origin == OriginInternal || origin == OriginObersuite
}

type Ticket struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ContactID   *uint          `gorm:"index" json:"contact_id,omitempty"`
	Contact     *Contact       `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	UserID      *uint          `gorm:"index" json:"user_id,omitempty"` // Professional referenced by an internal alert
	Origin      string         `gorm:"size:20;index" json:"origin"` // "internal" = Obertrack alert; channel-tagged otherwise
	// Session es la sesión de WAHA (el número de WhatsApp) de la que proviene la
	// conversación. Sin esto, cambiar de sesión dejaba los chats de la cuenta
	// anterior mezclados en la bandeja, sin manera de separarlos. Vacío en los
	// tickets que no son de WhatsApp.
	Session string `gorm:"size:100;index" json:"session,omitempty"`
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
	// ExternalID NO lleva etiqueta `index`: el índice de esta columna es un ÚNICO
	// PARCIAL creado a mano por migración (`idx_ticket_messages_external_id`, solo
	// para external_id no vacío y filas vivas). Si se declara aquí, AutoMigrate lo
	// reemplaza por un índice normal y se pierde la unicidad —que es lo que hace
	// que el ON CONFLICT de CreateMessageIfNew deduplique—, con el resultado de que
	// cada reimportación del historial duplica los mensajes.
	ExternalID string `gorm:"size:255" json:"external_id"` // WAHA ID or Brevo Message-ID
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// --- Bandeja de salida (solo mensajes salientes de WhatsApp) ---
	//
	// Un envío a WAHA no ocurre dentro del request: el mensaje se guarda como
	// 'pending' y un worker lo entrega con reintentos. Así un reinicio del backend
	// —o una caída de WAHA— no pierde la respuesta del agente, y el request no
	// queda bloqueado los ~6s que tardan el espaciado antibaneo y el tecleo.
	//
	// Vacío = no aplica: mensajes entrantes, notas internas y todo el histórico
	// anterior al outbox, que se consideran ya entregados.
	DeliveryStatus   string     `gorm:"size:20;index:idx_msg_delivery" json:"delivery_status,omitempty"`
	DeliveryAttempts int        `gorm:"not null;default:0" json:"delivery_attempts,omitempty"`
	// NextAttemptAt es el instante a partir del cual el worker puede retomar el
	// envío. NULL = "ya". Sobrevive al reinicio, que es lo que hace que el backoff
	// sea real y no un sleep en memoria.
	NextAttemptAt *time.Time `gorm:"index:idx_msg_delivery" json:"next_attempt_at,omitempty"`
	DeliveryError string     `gorm:"type:text" json:"delivery_error,omitempty"`

	Sender *User `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
}

const (
	// DeliveryPending: encolado, todavía no aceptado por WAHA.
	DeliveryPending = "pending"
	// DeliverySent: WAHA lo aceptó y devolvió su ID.
	DeliverySent = "sent"
	// DeliveryFailed: agotados los reintentos. No vuelve a intentarse solo; queda
	// visible en el chat para que el agente decida reenviarlo.
	DeliveryFailed = "failed"

	// DeliveryMaxAttempts acota los reintentos de un mensaje saliente.
	DeliveryMaxAttempts = 6
)

// whatsappDeliveryBackoff es la espera ANTES de cada reintento; la posición i es
// lo que se espera tras el intento i+1. Se escalona porque los fallos reales de
// WAHA (contenedor reiniciándose, sesión caída que hay que revincular) duran
// minutos, no segundos: reintentar cada 15s solo gasta los seis intentos antes de
// que nadie haya podido reaccionar.
var whatsappDeliveryBackoff = []time.Duration{
	15 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
}

// DeliveryRetryDelay devuelve cuánto esperar tras `attempts` intentos fallidos.
// Se satura en el último escalón, así que subir DeliveryMaxAttempts nunca
// desborda la tabla.
func DeliveryRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(whatsappDeliveryBackoff) {
		attempts = len(whatsappDeliveryBackoff)
	}
	return whatsappDeliveryBackoff[attempts-1]
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
