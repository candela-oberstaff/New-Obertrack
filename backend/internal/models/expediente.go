package models

import (
	"time"

	"gorm.io/gorm"
)

// Visibilidad de una entrada del expediente. El expediente sirve a DOS
// audiencias: la empresa (historial laboral / RR.HH.) y el profesional (CV
// vivo). Algunas entradas son internas de la empresa; otras se comparten con
// el profesional para que las vea en su expediente personal.
const (
	ExpedientePrivate = "private" // Solo la empresa (RR.HH.) la ve
	ExpedienteShared  = "shared"  // También visible para el profesional
)

// Tipos de nota del expediente.
const (
	NoteKindNote       = "note"       // Anotación libre (seguimiento, incidencia)
	NoteKindEvaluation = "evaluation" // Evaluación de desempeño (puede traer rating)
)

// EmploymentNote es una evaluación o anotación que la empresa registra sobre un
// profesional durante un empleo. Forma parte del expediente laboral.
type EmploymentNote struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	EmploymentID uint           `gorm:"not null;index" json:"employment_id"`
	AuthorID     uint           `gorm:"not null" json:"author_id"`
	Kind         string         `gorm:"size:20;not null;default:'note'" json:"kind"`
	Rating       *int           `json:"rating,omitempty"` // 1..5, solo en evaluaciones
	Content      string         `gorm:"type:text;not null" json:"content"`
	Visibility   string         `gorm:"size:20;not null;default:'private'" json:"visibility"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (EmploymentNote) TableName() string {
	return "employment_notes"
}

// EmploymentDocument es un archivo adjunto al expediente de un empleo
// (contrato, certificado, evaluación firmada, etc.). El binario se sube por el
// flujo de uploads existente; aquí guardamos solo los metadatos y el vínculo.
type EmploymentDocument struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	EmploymentID uint           `gorm:"not null;index" json:"employment_id"`
	UploadedBy   uint           `gorm:"not null" json:"uploaded_by"`
	Title        string         `gorm:"size:255" json:"title"`
	FileName     string         `gorm:"size:255;not null" json:"file_name"`
	FileURL      string         `gorm:"size:512;not null" json:"file_url"`
	FileSize     int64          `json:"file_size"`
	MimeType     string         `gorm:"size:128" json:"mime_type"`
	Visibility   string         `gorm:"size:20;not null;default:'private'" json:"visibility"`
	// ExpiresAt marca el vencimiento del documento (contratos, certificados...).
	// Nullable: la mayoría de los documentos no vencen.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// ExpiryAlertedAt evita repetir la alerta de vencimiento; se limpia al
	// renovar (cambiar expires_at). Interno, no se expone.
	ExpiryAlertedAt *time.Time     `gorm:"index" json:"-"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (EmploymentDocument) TableName() string {
	return "employment_documents"
}

// Canales de contacto registrados sobre un profesional.
const (
	ContactEmail    = "email"
	ContactWhatsApp = "whatsapp"
	ContactChat     = "chat"
)

func IsValidContactChannel(ch string) bool {
	return ch == ContactEmail || ch == ContactWhatsApp || ch == ContactChat
}

// ContactLog registra un intento de contacto (email, WhatsApp o chat interno)
// del equipo hacia un profesional. Forma parte del historial del expediente.
// Como email/WhatsApp abren un cliente externo, esto refleja el intento (el
// clic en "contactar"), no la entrega.
type ContactLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	ByUserID  uint      `gorm:"not null" json:"by_user_id"`
	Channel   string    `gorm:"size:20;not null" json:"channel"`
	Note      string    `gorm:"type:text" json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (ContactLog) TableName() string {
	return "contact_logs"
}
