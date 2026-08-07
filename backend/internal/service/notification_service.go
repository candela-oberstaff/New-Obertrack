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
	DeleteByTaskID(taskID uint) error
}

type notificationService struct {
	repo repository.NotificationRepository
	// pusher es Web Push del navegador (opcional, nil = deshabilitado). Solo se
	// usa cuando el usuario NO tiene la app abierta: con un socket vivo ya le
	// llegan la campanita y el toast.
	pusher *WebPushService
}

func NewNotificationService(repo repository.NotificationRepository, pusher *WebPushService) NotificationService {
	return &notificationService{repo: repo, pusher: pusher}
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

	// Sin la app abierta, el aviso sale por Web Push a sus navegadores
	// suscritos (asíncrono: hace HTTP hacia los push services).
	if s.pusher != nil && !websocket.GlobalNotifHub.IsOnline(userID) {
		link := ""
		if data != nil {
			if l, ok := data["link"].(string); ok {
				link = l
			}
		}
		go s.pusher.SendToUser(userID, title, message, link)
	}

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

func (s *notificationService) DeleteByTaskID(taskID uint) error {
	return s.repo.DeleteByTaskID(taskID)
}
