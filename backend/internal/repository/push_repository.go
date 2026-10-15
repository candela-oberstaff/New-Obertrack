package repository

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PushRepository guarda las suscripciones Web Push y el par VAPID del servidor.
type PushRepository interface {
	// GetKeys devuelve el par VAPID, o (nil, nil) si aún no se generó.
	GetKeys() (*models.WebPushKeys, error)
	SaveKeys(keys *models.WebPushKeys) error

	// UpsertSubscription registra (o reasigna) la suscripción de un navegador.
	// El endpoint es único: si otro usuario inicia sesión en el mismo navegador,
	// la fila pasa a ser suya y el anterior deja de recibir avisos ahí.
	UpsertSubscription(sub *models.PushSubscription) error
	// DeleteSubscription borra la suscripción de un endpoint para un usuario.
	DeleteSubscription(userID uint, endpoint string) error
	// DeleteByEndpoint borra un endpoint muerto (el push service devolvió 404/410).
	DeleteByEndpoint(endpoint string) error
	ListSubscriptionsByUser(userID uint) ([]models.PushSubscription, error)

	// UpsertDevice registra (o reasigna) el token FCM de un teléfono.
	UpsertDevice(device *models.DeviceToken) error
	// DeleteDevice borra el token de un usuario (cierre de sesión).
	DeleteDevice(userID uint, token string) error
	// DeleteDeviceByToken borra un token que FCM dio por inexistente.
	DeleteDeviceByToken(token string) error
	ListDevicesByUser(userID uint) ([]models.DeviceToken, error)
}

type pushRepository struct {
	db *gorm.DB
}

func NewPushRepository(db *gorm.DB) PushRepository {
	return &pushRepository{db: db}
}

func (r *pushRepository) GetKeys() (*models.WebPushKeys, error) {
	var keys models.WebPushKeys
	err := r.db.First(&keys).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &keys, nil
}

func (r *pushRepository) SaveKeys(keys *models.WebPushKeys) error {
	return r.db.Create(keys).Error
}

func (r *pushRepository) UpsertSubscription(sub *models.PushSubscription) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "p256dh", "auth"}),
	}).Create(sub).Error
}

func (r *pushRepository) DeleteSubscription(userID uint, endpoint string) error {
	return r.db.Where("user_id = ? AND endpoint = ?", userID, endpoint).
		Delete(&models.PushSubscription{}).Error
}

func (r *pushRepository) DeleteByEndpoint(endpoint string) error {
	return r.db.Where("endpoint = ?", endpoint).Delete(&models.PushSubscription{}).Error
}

func (r *pushRepository) ListSubscriptionsByUser(userID uint) ([]models.PushSubscription, error) {
	var subs []models.PushSubscription
	err := r.db.Where("user_id = ?", userID).Find(&subs).Error
	return subs, err
}

func (r *pushRepository) UpsertDevice(device *models.DeviceToken) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "platform", "updated_at"}),
	}).Create(device).Error
}

func (r *pushRepository) DeleteDevice(userID uint, token string) error {
	return r.db.Where("user_id = ? AND token = ?", userID, token).
		Delete(&models.DeviceToken{}).Error
}

func (r *pushRepository) DeleteDeviceByToken(token string) error {
	return r.db.Where("token = ?", token).Delete(&models.DeviceToken{}).Error
}

func (r *pushRepository) ListDevicesByUser(userID uint) ([]models.DeviceToken, error) {
	var devices []models.DeviceToken
	err := r.db.Where("user_id = ?", userID).Find(&devices).Error
	return devices, err
}
