package models

import "time"

// EmailRecipient representa un destinatario configurado para un tipo de correo.
type EmailRecipient struct {
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
}

// EmailSetting es la configuración persistida de UN tipo de correo del sistema.
type EmailSetting struct {
	Key        string    `gorm:"primaryKey;size:60" json:"key"`
	Enabled    bool      `gorm:"not null" json:"enabled"`
	Frequency  string    `gorm:"size:20;not null;default:'diaria'" json:"frequency"`
	DayOfMonth int       `gorm:"not null;default:1" json:"day_of_month"`
	Weekday    int       `gorm:"not null;default:1" json:"weekday"`
	Hour       int       `gorm:"not null;default:8" json:"hour"`
	Minute     int       `gorm:"not null;default:0" json:"minute"`
	Timezone   string    `gorm:"size:64;not null;default:'America/Santiago'" json:"timezone"`
	Recipients string    `gorm:"type:text" json:"recipients"` // JSON array string
	UpdatedBy  uint      `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (EmailSetting) TableName() string {
	return "email_settings"
}
