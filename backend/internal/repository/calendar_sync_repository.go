package repository

import (
	"errors"

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

	// --- Outbox de jobs ---
	EnqueueJob(job *models.CalendarSyncJob) error
	// ClaimPendingJobs devuelve hasta `limit` jobs pendientes (o fallidos con
	// reintentos disponibles), del más antiguo al más nuevo.
	ClaimPendingJobs(limit int) ([]models.CalendarSyncJob, error)
	MarkJobDone(jobID uint) error
	// MarkJobFailed incrementa el intento y guarda el error; si se agotaron los
	// reintentos, deja el job en estado 'failed' (no se reejecuta).
	MarkJobFailed(jobID uint, attempts int, errMsg string, exhausted bool) error
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

func (r *calendarSyncRepository) EnqueueJob(job *models.CalendarSyncJob) error {
	return r.db.Create(job).Error
}

// ClaimPendingJobs selecciona jobs procesables: pendientes, o con intentos aún
// por debajo del tope. El worker es único (una goroutine), así que no hace falta
// SELECT ... FOR UPDATE; el orden por id garantiza FIFO.
func (r *calendarSyncRepository) ClaimPendingJobs(limit int) ([]models.CalendarSyncJob, error) {
	var jobs []models.CalendarSyncJob
	err := r.db.
		Where("status = ?", models.CalendarSyncStatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func (r *calendarSyncRepository) MarkJobDone(jobID uint) error {
	return r.db.Model(&models.CalendarSyncJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     models.CalendarSyncStatusDone,
			"last_error": "",
		}).Error
}

func (r *calendarSyncRepository) MarkJobFailed(jobID uint, attempts int, errMsg string, exhausted bool) error {
	status := models.CalendarSyncStatusPending
	if exhausted {
		status = models.CalendarSyncStatusFailed
	}
	return r.db.Model(&models.CalendarSyncJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     status,
			"attempts":   attempts,
			"last_error": errMsg,
		}).Error
}

func (r *calendarSyncRepository) SupersedePendingJobs(taskID, userID, exceptJobID uint) error {
	return r.db.Model(&models.CalendarSyncJob{}).
		Where("task_id = ? AND user_id = ? AND status = ? AND id <> ?",
			taskID, userID, models.CalendarSyncStatusPending, exceptJobID).
		Update("status", models.CalendarSyncStatusDone).Error
}
