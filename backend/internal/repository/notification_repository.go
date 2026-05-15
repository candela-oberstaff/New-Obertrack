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
	return r.db.Where("data LIKE ?", "%\"task_id\":"+strconv.FormatUint(uint64(taskID), 10)+"%").Delete(&models.Notification{}).Error
}
