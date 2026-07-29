package repository

import (
	"errors"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// ErrMeetingNotFound: la sesión no existe, o no es del tenant que pregunta.
var ErrMeetingNotFound = errors.New("sesión no encontrada")

// MeetingFilter acota el listado. UserID es obligatorio en la práctica: una
// sesión solo la ven su organizador y sus invitados, nunca toda la empresa.
type MeetingFilter struct {
	TenantID uint
	UserID   uint
	// Past en false devuelve las sesiones que aún no han terminado (las
	// próximas); en true, el histórico.
	Past bool
	// TaskID filtra las sesiones enlazadas a una tarea concreta.
	TaskID uint
	Limit  int
}

type MeetingRepository interface {
	Create(session *models.MeetingSession) error
	GetByID(id uint) (*models.MeetingSession, error)
	// List devuelve las sesiones visibles para el usuario del filtro, con sus
	// invitados y el organizador precargados.
	List(f MeetingFilter) ([]models.MeetingSession, error)
	// UpdateFields aplica cambios puntuales por id. Se actualiza por id con un
	// modelo limpio —y no con Model(instancia).Updates— para no arrastrar las
	// asociaciones precargadas (Organizer, Attendees) y pisar sus claves.
	UpdateFields(id uint, updates map[string]interface{}) error
	// ReplaceAttendees deja la lista de invitados exactamente como se le pasa,
	// dentro de una transacción: un fallo a mitad no deja media lista aplicada.
	ReplaceAttendees(sessionID uint, attendees []models.MeetingAttendee) error
	Delete(id uint) error
	// IsParticipant indica si el usuario es organizador o invitado. Es lo que
	// decide si puede ver la sesión.
	IsParticipant(sessionID, userID uint) (bool, error)
}

type meetingRepository struct {
	db *gorm.DB
}

func NewMeetingRepository(db *gorm.DB) MeetingRepository {
	return &meetingRepository{db: db}
}

// Create guarda la sesión y sus invitados de una vez: los Attendees cuelgan de
// la asociación, así que GORM los inserta con el session_id ya resuelto.
func (r *meetingRepository) Create(session *models.MeetingSession) error {
	return r.db.Create(session).Error
}

func (r *meetingRepository) GetByID(id uint) (*models.MeetingSession, error) {
	var session models.MeetingSession
	err := r.db.Preload("Attendees").Preload("Organizer").First(&session, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMeetingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *meetingRepository) List(f MeetingFilter) ([]models.MeetingSession, error) {
	var sessions []models.MeetingSession

	q := r.db.Model(&models.MeetingSession{}).
		Preload("Attendees").
		Preload("Organizer").
		Where("tenant_id = ?", f.TenantID)

	// Visibilidad: organizador o invitado. La subconsulta evita el JOIN, que
	// duplicaría filas cuando hay varios invitados.
	if f.UserID != 0 {
		q = q.Where(
			"organizer_id = ? OR id IN (SELECT session_id FROM meeting_attendees WHERE user_id = ?)",
			f.UserID, f.UserID,
		)
	}
	if f.TaskID != 0 {
		q = q.Where("task_id = ?", f.TaskID)
	}

	// El corte es por series_ends_at y NO por end_at, que solo describe la
	// primera ocurrencia: filtrando por end_at, una sesión diaria desaparecía de
	// "próximas" en cuanto terminaba su primer día. NULL = serie sin fin, que
	// siempre está viva.
	//
	// Una reunión que empezó hace diez minutos y dura una hora sigue contando
	// como próxima: es a la que la gente se va a unir.
	now := time.Now()
	if f.Past {
		q = q.Where("series_ends_at IS NOT NULL AND series_ends_at < ?", now).
			Order("start_at DESC")
	} else {
		q = q.Where("series_ends_at IS NULL OR series_ends_at >= ?", now).
			Where("status = ?", models.MeetingStatusScheduled).
			// El orden final lo pone el servicio por próxima ocurrencia, que solo
			// se puede calcular en Go; este ORDER BY deja la lista estable y hace
			// que el LIMIT de /upcoming recoja las candidatas más cercanas.
			Order("start_at ASC")
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}

	return sessions, q.Find(&sessions).Error
}

func (r *meetingRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.MeetingSession{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *meetingRepository) ReplaceAttendees(sessionID uint, attendees []models.MeetingAttendee) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).
			Delete(&models.MeetingAttendee{}).Error; err != nil {
			return err
		}
		if len(attendees) == 0 {
			return nil
		}
		for i := range attendees {
			attendees[i].ID = 0
			attendees[i].SessionID = sessionID
		}
		return tx.Create(&attendees).Error
	})
}

func (r *meetingRepository) Delete(id uint) error {
	return r.db.Delete(&models.MeetingSession{}, id).Error
}

func (r *meetingRepository) IsParticipant(sessionID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.MeetingSession{}).
		Where("id = ?", sessionID).
		Where(
			"organizer_id = ? OR id IN (SELECT session_id FROM meeting_attendees WHERE user_id = ?)",
			userID, userID,
		).
		Count(&count).Error
	return count > 0, err
}
