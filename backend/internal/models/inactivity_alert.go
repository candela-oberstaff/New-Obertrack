package models

import "time"

// InactivityAlert registra la última alerta enviada por inactividad de un
// profesional, para que el watcher diario no repita la misma alerta cada día.
type InactivityAlert struct {
	UserID        uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	DaysInactive  int       `json:"days_inactive"`
	LastAlertedAt time.Time `json:"last_alerted_at"`
}

func (InactivityAlert) TableName() string {
	return "inactivity_alerts"
}

// CompanyUsageAlert registra el último aviso de "empresa sin abrir la app",
// para que el vigía diario no repita la misma empresa cada 24 horas.
//
// Va en su propia tabla y no reutiliza inactivity_alerts porque la clave es
// otra —una empresa, no una persona— y mezclarlas obligaría a distinguir los
// dos tipos de id en cada consulta, con el riesgo de que un id de empresa
// silenciara el aviso de una persona que casualmente comparta número.
type CompanyUsageAlert struct {
	CompanyID     uint      `gorm:"primaryKey;autoIncrement:false" json:"company_id"`
	DaysStale     int       `json:"days_stale"`
	LastAlertedAt time.Time `json:"last_alerted_at"`
}

func (CompanyUsageAlert) TableName() string {
	return "company_usage_alerts"
}
