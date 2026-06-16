package models

import "time"

// Tipos de evento del ciclo de vida de una empresa (tenant). Se registran los
// que no quedan reflejados por otra tabla (suspensión / reactivación). El alta,
// las altas/bajas de empleados, horas y gestiones se derivan de sus tablas.
const (
	CompanyEventSuspended   = "suspended"   // Se suspendió el acceso de la empresa
	CompanyEventReactivated = "reactivated" // Se reactivó el acceso
)

// CompanyEvent es una entrada del expediente de la empresa: hitos del ciclo de
// vida que no tienen otra fuente con fecha (suspensiones, reactivaciones).
type CompanyEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CompanyID uint      `gorm:"not null;index" json:"company_id"`
	Type      string    `gorm:"size:30;not null" json:"type"`
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	ByUserID  uint      `json:"by_user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (CompanyEvent) TableName() string {
	return "company_events"
}
