package models

import "time"

// EmailSetting es el interruptor persistido de UN tipo de correo del sistema
// (por ejemplo "support_ticket" o "inactivity_alert"). Solo se guarda la fila
// cuando alguien cambia el valor: la ausencia de fila significa "activo", que
// es el comportamiento por defecto de todos los correos.
//
// Reemplaza a las constantes de código que se usaban para pausar correos: el
// equipo los enciende y apaga desde Configuración → Correos, sin redeploy.
type EmailSetting struct {
	// Key es la clave del catálogo (service.EmailKind...).
	Key string `gorm:"primaryKey;size:60" json:"key"`
	// Enabled NO lleva `default:true` en el tag a propósito: con un default,
	// GORM sustituye el valor CERO del bool (false) por ese default al
	// escribir, así que apagar un correo guardaba `true` y el interruptor no
	// apagaba nada. La columna sí conserva DEFAULT true en la base, que es lo
	// correcto para una fila insertada sin este campo.
	Enabled   bool      `gorm:"not null" json:"enabled"`
	UpdatedBy uint      `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (EmailSetting) TableName() string {
	return "email_settings"
}
