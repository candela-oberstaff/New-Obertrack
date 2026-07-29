package repository

import (
	"errors"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// ErrCalendarLinkNotFound: no hay evento enlazado para esa (tarea, usuario).
var ErrCalendarLinkNotFound = errors.New("enlace de evento de calendario no encontrado")

type CalendarSyncRepository interface {
	// --- Enlaces tarea↔evento ---
	GetLink(taskID, userID uint) (*models.CalendarEventLink, error)
	// ListLinksForTask devuelve todos los enlaces de una tarea (uno por asignado
	// con evento). Se usa al borrar la tarea para encolar el borrado de cada uno.
	ListLinksForTask(taskID uint) ([]models.CalendarEventLink, error)
	UpsertLink(link *models.CalendarEventLink) error
	DeleteLink(taskID, userID uint) error
	// DeleteLinksForUser borra todos los enlaces de un usuario. Se usa al
	// desvincular su cuenta: sin cuenta no hay credencial con la que tocar esos
	// eventos, así que el enlace deja de significar nada.
	DeleteLinksForUser(userID uint) error

	// --- Outbox de jobs ---
	EnqueueJob(job *models.CalendarSyncJob) error
	// ClaimPendingJobs devuelve hasta `limit` jobs en estado 'pending' YA VENCIDOS
	// (next_attempt_at nulo o <= now), del más antiguo al más nuevo. Un job que
	// falló con reintentos disponibles vuelve a 'pending' con su próxima fecha
	// (lo hace MarkJobFailed), así que reaparece por aquí cuando toca; los
	// agotados quedan en 'failed' y ya no se seleccionan.
	ClaimPendingJobs(limit int, now time.Time) ([]models.CalendarSyncJob, error)
	MarkJobDone(jobID uint) error
	// MarkJobFailed incrementa el intento y guarda el error. retryAt es cuándo
	// puede volver a intentarse; nil significa que se agotó y el job queda en
	// 'failed' (no se reejecuta). Que la decisión viaje como fecha —y no como un
	// booleano aparte— hace imposible dejar un job 'pending' sin fecha de
	// reintento, o uno 'failed' con ella.
	MarkJobFailed(jobID uint, attempts int, errMsg string, retryAt *time.Time) error
	// SupersedePendingJobs marca como 'done' los jobs pendientes previos de la
	// misma (tarea, usuario): si una tarea se editó tres veces antes de que el
	// worker corriera, solo interesa el último estado, no reproducir cada cambio.
	SupersedePendingJobs(taskID, userID uint, exceptJobID uint) error
}

type calendarSyncRepository struct {
	db *gorm.DB
}

func NewCalendarSyncRepository(db *gorm.DB) CalendarSyncRepository {
	return &calendarSyncRepository{db: db}
}

func (r *calendarSyncRepository) GetLink(taskID, userID uint) (*models.CalendarEventLink, error) {
	var link models.CalendarEventLink
	err := r.db.Where("task_id = ? AND user_id = ?", taskID, userID).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCalendarLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *calendarSyncRepository) ListLinksForTask(taskID uint) ([]models.CalendarEventLink, error) {
	var links []models.CalendarEventLink
	err := r.db.Where("task_id = ?", taskID).Find(&links).Error
	return links, err
}

// UpsertLink guarda el enlace tras crear el evento, o actualiza el event id si
// por algún motivo cambió. Actualiza por (task_id, user_id) —no por instancia
// cargada— para no arrastrar campos viejos.
func (r *calendarSyncRepository) UpsertLink(link *models.CalendarEventLink) error {
	_, err := r.GetLink(link.TaskID, link.UserID)
	if errors.Is(err, ErrCalendarLinkNotFound) {
		return r.db.Create(link).Error
	}
	if err != nil {
		return err
	}
	return r.db.Model(&models.CalendarEventLink{}).
		Where("task_id = ? AND user_id = ?", link.TaskID, link.UserID).
		Updates(map[string]interface{}{
			"google_event_id": link.GoogleEventID,
			"calendar_id":     link.CalendarID,
		}).Error
}

func (r *calendarSyncRepository) DeleteLink(taskID, userID uint) error {
	return r.db.Where("task_id = ? AND user_id = ?", taskID, userID).
		Delete(&models.CalendarEventLink{}).Error
}

func (r *calendarSyncRepository) DeleteLinksForUser(userID uint) error {
	return r.db.Where("user_id = ?", userID).
		Delete(&models.CalendarEventLink{}).Error
}

func (r *calendarSyncRepository) EnqueueJob(job *models.CalendarSyncJob) error {
	return r.db.Create(job).Error
}

// ClaimPendingJobs selecciona los jobs procesables: pendientes y ya vencidos.
// El tope de intentos no hace falta repetirlo aquí porque MarkJobFailed ya deja
// en 'pending' lo que tiene reintentos y en 'failed' lo agotado. El worker es
// único (una goroutine), así que no hace falta SELECT ... FOR UPDATE; el orden por
// id garantiza FIFO.
//
// Un job esperando su backoff no bloquea la cola: el filtro lo deja fuera de la
// selección, así que los demás siguen procesándose con normalidad.
func (r *calendarSyncRepository) ClaimPendingJobs(limit int, now time.Time) ([]models.CalendarSyncJob, error) {
	var jobs []models.CalendarSyncJob
	err := r.db.
		Where("status = ?", models.CalendarSyncStatusPending).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Order("id ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func (r *calendarSyncRepository) MarkJobDone(jobID uint) error {
	return r.db.Model(&models.CalendarSyncJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":          models.CalendarSyncStatusDone,
			"next_attempt_at": nil,
			"last_error":      "",
		}).Error
}

func (r *calendarSyncRepository) MarkJobFailed(jobID uint, attempts int, errMsg string, retryAt *time.Time) error {
	status := models.CalendarSyncStatusPending
	if retryAt == nil {
		status = models.CalendarSyncStatusFailed
	}
	return r.db.Model(&models.CalendarSyncJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status": status,
			// next_attempt_at se escribe siempre, incluido el nil de un job
			// agotado: dejar la fecha del intento anterior en una fila 'failed'
			// solo confundiría al leer la tabla para diagnosticar.
			"next_attempt_at": retryAt,
			"attempts":        attempts,
			"last_error":      errMsg,
		}).Error
}

func (r *calendarSyncRepository) SupersedePendingJobs(taskID, userID, exceptJobID uint) error {
	return r.db.Model(&models.CalendarSyncJob{}).
		Where("task_id = ? AND user_id = ? AND status = ? AND id <> ?",
			taskID, userID, models.CalendarSyncStatusPending, exceptJobID).
		Update("status", models.CalendarSyncStatusDone).Error
}
