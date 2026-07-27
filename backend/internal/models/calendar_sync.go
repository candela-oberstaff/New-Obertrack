package models

import "time"

// CalendarEventLink enlaza una tarea con el evento que la representa en el Google
// Calendar de UN asignado concreto. La clave (task_id, user_id) es única: cada
// persona asignada tiene su propio evento en su propio calendario, así que una
// tarea con tres asignados conectados genera tres enlaces.
//
// Sin esta tabla no se podría actualizar ni borrar el evento correcto cuando la
// tarea cambia (solo sabríamos crear, y cada guardado duplicaría el evento).
type CalendarEventLink struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	TaskID        uint   `gorm:"not null;uniqueIndex:idx_task_user_event" json:"task_id"`
	UserID        uint   `gorm:"not null;uniqueIndex:idx_task_user_event" json:"user_id"`
	GoogleEventID string `gorm:"size:255;not null" json:"google_event_id"`
	// CalendarID se guarda por si en el futuro se permite elegir destino; hoy
	// siempre es 'primary'.
	CalendarID string    `gorm:"size:255;not null;default:'primary'" json:"calendar_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (CalendarEventLink) TableName() string { return "calendar_event_links" }

const (
	// CalendarSyncActionUpsert crea el evento si no existe o lo actualiza si ya.
	CalendarSyncActionUpsert = "upsert"
	// CalendarSyncActionDelete borra el evento en Google y su enlace local.
	CalendarSyncActionDelete = "delete"

	CalendarSyncStatusPending = "pending"
	CalendarSyncStatusDone    = "done"
	// CalendarSyncStatusFailed marca un job agotado (sin más reintentos). Se
	// conserva como bitácora para diagnóstico; no vuelve a ejecutarse.
	CalendarSyncStatusFailed = "failed"

	// CalendarSyncMaxAttempts acota los reintentos de un job antes de darlo por
	// fallido. Cubre caídas transitorias de la API de Google sin reintentar para
	// siempre un error permanente (p. ej. datos inválidos).
	CalendarSyncMaxAttempts = 5
)

// CalendarSyncJob es la bandeja de salida (outbox) de la sincronización con
// Google Calendar. Al mutar una tarea NO se llama a Google dentro del request:
// se encola un job aquí (rápido y transaccional) y un worker en segundo plano lo
// procesa con reintentos. Así una caída de la API de Google, o un reinicio del
// backend, no pierde ni bloquea la operación del usuario.
//
// Hay un job por (tarea, usuario) para que el fallo de un asignado —p. ej. uno
// que revocó el acceso— no frene la sincronización de los demás.
type CalendarSyncJob struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	TaskID uint   `gorm:"not null;index:idx_sync_task" json:"task_id"`
	UserID uint   `gorm:"not null" json:"user_id"`
	Action string `gorm:"size:20;not null" json:"action"`

	// GoogleEventID y CalendarID solo se rellenan en los jobs 'delete': el enlace
	// puede borrarse (p. ej. al eliminar la tarea) antes de que el worker corra,
	// así que el job lleva consigo lo necesario para borrar en Google. Los jobs
	// 'upsert' resuelven el evento desde calendar_event_links.
	GoogleEventID string `gorm:"size:255" json:"google_event_id,omitempty"`
	CalendarID    string `gorm:"size:255" json:"calendar_id,omitempty"`

	Status    string    `gorm:"size:20;not null;default:'pending';index:idx_sync_status" json:"status"`
	Attempts  int       `gorm:"not null;default:0" json:"attempts"`
	LastError string    `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CalendarSyncJob) TableName() string { return "calendar_sync_jobs" }
