package repository

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// ErrGoogleAccountNotFound: el usuario no tiene ninguna cuenta de Google
// vinculada. Es un estado normal (la mayoría no la vincula), no un fallo.
var ErrGoogleAccountNotFound = errors.New("cuenta de Google no vinculada")

type GoogleCalendarRepository interface {
	// GetByUser devuelve el vínculo del usuario o ErrGoogleAccountNotFound.
	GetByUser(userID uint) (*models.GoogleCalendarAccount, error)
	// Upsert crea el vínculo o actualiza el existente del mismo usuario. Es lo
	// que hace que reconectar la misma cuenta no genere una segunda fila.
	Upsert(account *models.GoogleCalendarAccount) error
	// UpdateFields aplica cambios puntuales por user_id. Se actualiza por
	// user_id (y no con Model(instancia).Updates) para no arrastrar campos de
	// una instancia cargada previamente.
	UpdateFields(userID uint, updates map[string]interface{}) error
	DeleteByUser(userID uint) error
}

type googleCalendarRepository struct {
	db *gorm.DB
}

func NewGoogleCalendarRepository(db *gorm.DB) GoogleCalendarRepository {
	return &googleCalendarRepository{db: db}
}

func (r *googleCalendarRepository) GetByUser(userID uint) (*models.GoogleCalendarAccount, error) {
	var account models.GoogleCalendarAccount
	err := r.db.Where("user_id = ?", userID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGoogleAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// Upsert resuelve el caso de reconexión: si el usuario ya tenía una cuenta
// vinculada (la misma u otra distinta), se sobrescriben sus credenciales en la
// fila existente. Conserva el calendario destino elegido cuando se reconecta la
// MISMA cuenta de Google, para no devolver al usuario a 'primary' sin avisar.
func (r *googleCalendarRepository) Upsert(account *models.GoogleCalendarAccount) error {
	existing, err := r.GetByUser(account.UserID)
	if errors.Is(err, ErrGoogleAccountNotFound) {
		return r.db.Create(account).Error
	}
	if err != nil {
		return err
	}

	if existing.GoogleSub == account.GoogleSub && existing.CalendarID != "" {
		account.CalendarID = existing.CalendarID
		account.CalendarName = existing.CalendarName
	}

	account.ID = existing.ID
	account.CreatedAt = existing.CreatedAt
	return r.db.Model(&models.GoogleCalendarAccount{}).
		Where("user_id = ?", account.UserID).
		Updates(map[string]interface{}{
			"google_sub":        account.GoogleSub,
			"google_email":      account.GoogleEmail,
			"refresh_token_enc": account.RefreshTokenEnc,
			"access_token_enc":  account.AccessTokenEnc,
			"access_expires_at": account.AccessExpiresAt,
			"scopes":            account.Scopes,
			"calendar_id":       account.CalendarID,
			"calendar_name":     account.CalendarName,
			"status":            account.Status,
			"last_error":        "",
			"connected_at":      account.ConnectedAt,
		}).Error
}

func (r *googleCalendarRepository) UpdateFields(userID uint, updates map[string]interface{}) error {
	return r.db.Model(&models.GoogleCalendarAccount{}).
		Where("user_id = ?", userID).
		Updates(updates).Error
}

func (r *googleCalendarRepository) DeleteByUser(userID uint) error {
	// Borrado duro: el modelo no tiene DeletedAt porque una credencial revocada
	// no debe quedar recuperable, y el uniqueIndex sobre user_id impediría
	// volver a vincular si la fila siguiera presente.
	return r.db.Where("user_id = ?", userID).
		Delete(&models.GoogleCalendarAccount{}).Error
}
