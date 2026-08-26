package repository

import (
	"strconv"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(notification *models.Notification) error
	GetByUserID(userID uint, limit int) ([]models.Notification, error)
	MarkAsRead(id uint, userID uint) error
	MarkAllAsRead(userID uint) error
	GetUnreadCount(userID uint) (int64, error)
	DeleteByTaskID(taskID uint) error
	// ExistsRecentForTask indica si ya se avisó a este usuario sobre esta tarea
	// desde `since`. Es la base de la deduplicación de avisos: el módulo de Tareas
	// notifica por su cuenta desde siempre, y una regla de workflow sobre el mismo
	// cambio duplicaría la campanita.
	ExistsRecentForTask(userID uint, taskID uint, since time.Time) (bool, error)
	// ExistsRecentNativeForTask sólo mira los avisos NATIVOS (los que no vienen de una
	// automatización), que son los que la deduplicación tiene que atrapar.
	ExistsRecentNativeForTask(userID uint, taskID uint, since time.Time) (bool, error)
}

type notificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(notification *models.Notification) error {
	return r.db.Create(notification).Error
}

func (r *notificationRepository) GetByUserID(userID uint, limit int) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&notifications).Error
	return notifications, err
}

func (r *notificationRepository) MarkAsRead(id uint, userID uint) error {
	now := time.Now()
	return r.db.Model(&models.Notification{}).Where("id = ? AND user_id = ?", id, userID).Update("read_at", &now).Error
}

func (r *notificationRepository) MarkAllAsRead(userID uint) error {
	now := time.Now()
	return r.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Update("read_at", &now).Error
}

func (r *notificationRepository) GetUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&count).Error
	return count, err
}

func (r *notificationRepository) DeleteByTaskID(taskID uint) error {
	return r.db.Where(taskIDJSONWhere, taskIDJSONPattern(taskID)).Delete(&models.Notification{}).Error
}

func (r *notificationRepository) ExistsRecentForTask(userID uint, taskID uint, since time.Time) (bool, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Where(taskIDJSONWhere, taskIDJSONPattern(taskID)).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

// ExistsRecentNativeForTask es la anterior acotada a los avisos NATIVOS de tareas.
//
// La deduplicación existe para que el aviso que manda Tareas por su cuenta y el que
// manda una regla no lleguen los dos por el mismo cambio. Aplicada entre dos avisos de
// REGLA callaba decisiones distintas del motor —mover la misma tarjeta dos veces en un
// minuto producía dos hechos y un solo aviso—, y desde fuera eso no se distingue de
// una automatización rota.
func (r *notificationRepository) ExistsRecentNativeForTask(userID uint, taskID uint, since time.Time) (bool, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Where("type <> ?", "workflow").
		Where(taskIDJSONWhere, taskIDJSONPattern(taskID)).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

// taskIDJSONWhere compara el task_id por TEXTO en vez de con los operadores JSON de
// Postgres, porque la columna admite filas antiguas cuyo contenido no es JSON válido
// y ->> reventaría sobre ellas en lugar de simplemente no coincidir.
//
// El cast a text es obligatorio: `data` es de tipo json, y json no tiene operador
// LIKE. Sin él, Postgres responde "operator does not exist: json ~~ unknown" y la
// consulta falla ENTERA. Así estuvo DeleteByTaskID desde su origen: como su error se
// descarta en taskService.Delete, borrar una tarea nunca llegó a borrar sus
// notificaciones y nadie se enteró.
const taskIDJSONWhere = "data::text LIKE ?"

func taskIDJSONPattern(taskID uint) string {
	return "%\"task_id\":" + strconv.FormatUint(uint64(taskID), 10) + "%"
}
