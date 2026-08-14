package models

import "time"

// SupervisorEscalationLog registra el último aviso de "hay jornadas pendientes
// hace demasiado" que se le mandó a un supervisor. Existe para que el escalado
// no se repita en cada pasada del watcher: como máximo un aviso por supervisor
// por ventana, aunque las jornadas sigan sin aprobarse.
//
// Mismo patrón que ChatDigestLog: una fila por usuario que se pisa a sí misma,
// en vez de guardar el historial de cada aviso. Lo que hace falta saber es
// "¿cuándo fue el último?", no "¿cuántos hubo?".
type SupervisorEscalationLog struct {
	UserID uint      `gorm:"primaryKey" json:"user_id"`
	SentAt time.Time `json:"sent_at"`
}

func (SupervisorEscalationLog) TableName() string {
	return "supervisor_escalation_logs"
}
