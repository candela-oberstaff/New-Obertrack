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
	CalendarSyncMaxAttempts = 6
)

// calendarSyncBackoff es la espera ANTES de cada reintento: la posición i es lo
// que se espera tras el intento i+1. Escalona en vez de repetir un intervalo fijo
// porque el fallo típico de la API de Google (cuota agotada, incidente) dura
// minutos u horas, no segundos: con reintentos cada 20s los seis intentos se
// gastaban en dos minutos y el evento quedaba sin crear hasta que alguien
// volviera a tocar la tarea. Con este escalonado la ventana de recuperación pasa
// a ~2h40m, que cubre un incidente real sin dejar jobs zombis para siempre.
var calendarSyncBackoff = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	8 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
}

// CalendarSyncRetryDelay devuelve cuánto esperar tras `attempts` intentos
// fallidos. Se satura en el último escalón, así que ampliar
// CalendarSyncMaxAttempts nunca desborda la tabla.
func CalendarSyncRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(calendarSyncBackoff) {
		attempts = len(calendarSyncBackoff)
	}
	return calendarSyncBackoff[attempts-1]
}

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

	Status   string `gorm:"size:20;not null;default:'pending';index:idx_sync_status" json:"status"`
	Attempts int    `gorm:"not null;default:0" json:"attempts"`
	// NextAttemptAt es el instante a partir del cual el worker puede volver a
	// tomar el job. NULL significa "ya" (jobs recién encolados). Lo fija el worker
	// al fallar, con la espera de CalendarSyncRetryDelay. Sin esta columna el
	// reintento era inmediato y el backoff no podría sobrevivir a un reinicio.
	NextAttemptAt *time.Time `gorm:"index:idx_sync_next_attempt" json:"next_attempt_at,omitempty"`
	LastError     string     `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (CalendarSyncJob) TableName() string { return "calendar_sync_jobs" }
