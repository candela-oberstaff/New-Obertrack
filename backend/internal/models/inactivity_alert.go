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
