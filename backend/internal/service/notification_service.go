package service

import (
	"encoding/json"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/websocket"
)

type NotificationService interface {
	CreateNotification(userID uint, notifType, title, message string, data map[string]interface{}) error
	GetNotifications(userID uint) ([]models.Notification, error)
	MarkAsRead(id uint, userID uint) error
	MarkAllAsRead(userID uint) error
	GetUnreadCount(userID uint) (int64, error)
}

type notificationService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) CreateNotification(userID uint, notifType, title, message string, data map[string]interface{}) error {
	dataJSON := ""
	if data != nil {
		b, _ := json.Marshal(data)
		dataJSON = string(b)
	}

	notification := &models.Notification{
		UserID:  userID,
		Type:    notifType,
		Title:   title,
		Message: message,
		Data:    dataJSON,
	}

	if err := s.repo.Create(notification); err != nil {
		return err
	}

	// Emit WebSocket notification
	websocket.GlobalNotifHub.NotifyUser(userID, notifType, map[string]interface{}{
		"id":      notification.ID,
		"type":    notifType,
		"title":   title,
		"message": message,
		"data":    dataJSON,
	})

	return nil
}

func (s *notificationService) GetNotifications(userID uint) ([]models.Notification, error) {
	return s.repo.GetByUserID(userID, 50)
}

func (s *notificationService) MarkAsRead(id uint, userID uint) error {
	return s.repo.MarkAsRead(id, userID)
}

func (s *notificationService) MarkAllAsRead(userID uint) error {
	return s.repo.MarkAllAsRead(userID)
}

func (s *notificationService) GetUnreadCount(userID uint) (int64, error) {
	return s.repo.GetUnreadCount(userID)
}
