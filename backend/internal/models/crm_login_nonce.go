package models

import "time"

// CrmLoginNonce registra cada token del CRM ya canjeado (acceso embebido a
// Tareas). El jti es la clave primaria: el canje lo consume con un INSERT y la
// restricción única es la que impide reutilizar un token, también con dos
// canjes simultáneos del mismo enlace.
//
// ExpiresAt es exp + leeway: pasado ese momento el token ya no valida por
// firma, así que la fila se puede borrar sin reabrir la reutilización.
type CrmLoginNonce struct {
	JTI       string    `gorm:"type:uuid;primaryKey" json:"jti"`
	Kid       string    `gorm:"size:64;not null" json:"kid"`
	Email     string    `gorm:"size:255;not null" json:"email"`
	CompanyID uint      `gorm:"not null" json:"company_id"`
	UsedAt    time.Time `gorm:"not null" json:"used_at"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
}

func (CrmLoginNonce) TableName() string { return "crm_login_nonces" }
