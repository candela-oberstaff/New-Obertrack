package models

import "time"

// Tipos de evento del ciclo de vida de una empresa (tenant). Se registran los
// que no quedan reflejados por otra tabla (suspensión / reactivación). El alta,
// las altas/bajas de empleados, horas y gestiones se derivan de sus tablas.
const (
	CompanyEventSuspended   = "suspended"   // Se suspendió el acceso de la empresa
	CompanyEventReactivated = "reactivated" // Se reactivó el acceso
	// CompanyEventNote es una anotación manual del equipo en el expediente
	// (una llamada, un acuerdo, un aviso). A diferencia del resto, no la
	// deriva el sistema de otra tabla: la escribe una persona, y por eso es
	// la única entrada que se puede borrar.
	CompanyEventNote = "note"
	// CompanyEventContact deja constancia de que el equipo contactó con la
	// empresa: un correo, un WhatsApp, una llamada o una reunión. Sin esto el
	// expediente cuenta lo que hizo el cliente pero no lo que hicimos nosotros,
	// y el siguiente agente que abre la ficha no sabe si ya se le escribió.
	// El canal va en Channel; Detail lleva el asunto o el resumen.
	CompanyEventContact = "contact"
)

// Canales por los que el equipo puede contactar con una empresa. Email y
// WhatsApp los registra el sistema al enviar; llamada y reunión los anota una
// persona a mano, porque ocurren fuera de la plataforma.
const (
	CompanyContactEmail    = "email"
	CompanyContactWhatsApp = "whatsapp"
	CompanyContactCall     = "call"
	CompanyContactMeeting  = "meeting"
)

// IsValidCompanyContactChannel valida el canal de un contacto con la empresa.
// Es una lista propia y no la de ContactLog (profesionales) porque las vías no
// coinciden: a una empresa se la llama y se la visita; con un profesional se usa
// el chat interno de la plataforma.
func IsValidCompanyContactChannel(ch string) bool {
	switch ch {
	case CompanyContactEmail, CompanyContactWhatsApp, CompanyContactCall, CompanyContactMeeting:
		return true
	}
	return false
}

// MaxCompanyNoteLength acota la nota para que el expediente siga siendo una
// línea de tiempo legible y no un cajón de texto largo.
const MaxCompanyNoteLength = 2000

// CompanyEvent es una entrada del expediente de la empresa: hitos del ciclo de
// vida que no tienen otra fuente con fecha (suspensiones, reactivaciones) y las
// notas que escribe el equipo.
type CompanyEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CompanyID uint      `gorm:"not null;index" json:"company_id"`
	Type      string    `gorm:"size:30;not null" json:"type"`
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	ByUserID  uint      `json:"by_user_id"`
	CreatedAt time.Time `json:"created_at"`
	// Pinned sube la nota a la cabecera del expediente, fuera de la
	// cronología: para avisos que siguen vigentes y hay que ver al entrar,
	// aunque se escribieran hace meses. Solo aplica a las notas.
	Pinned bool `gorm:"not null;default:false" json:"pinned"`
	// EditedAt marca que el texto se corrigió después de escribirlo. Es un
	// campo propio y no el updated_at de GORM a propósito: fijar o desfijar
	// una nota no es editarla, y en una auditoría no puede parecerlo.
	EditedAt *time.Time `json:"edited_at,omitempty"`
	// Channel solo aplica a los contactos (email, whatsapp, call, meeting).
	// Vacío en el resto de eventos.
	Channel string `gorm:"size:20" json:"channel,omitempty"`
}

func (CompanyEvent) TableName() string {
	return "company_events"
}
