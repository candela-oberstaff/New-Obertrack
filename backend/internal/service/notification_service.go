package service

import (
	"encoding/json"
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/websocket"
)

// taskIDFrom extrae el task_id del payload de un aviso. Devuelve false cuando el
// aviso no va sobre una tarea (bienvenidas, horas, soporte...), que es la señal de
// que no hay nada que deduplicar.
func taskIDFrom(data map[string]interface{}) (uint, bool) {
	if data == nil {
		return 0, false
	}
	switch v := data["task_id"].(type) {
	case uint:
		return v, v > 0
	case int:
		return uint(v), v > 0
	case int64:
		return uint(v), v > 0
	case float64:
		return uint(v), v > 0
	}
	return 0, false
}

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

// notifyDedupWindow es cuánto tiene que pasar para que un segundo aviso sobre la
// MISMA tarea al MISMO usuario deje de considerarse un duplicado.
//
// Existe porque el módulo de Tareas avisa por su cuenta desde siempre (asignados y
// empleador, en cada edición) y una regla de automatización sobre ese mismo cambio
// produciría dos campanitas por un solo hecho. Se deduplica en el DESTINO y no
// suprimiendo el aviso nativo en origen: saber si una regla cubre este cambio exige
// evaluar sus condiciones, y eso ocurre en el worker, no dentro del request.
//
// Un minuto es suficiente —el worker despierta a los pocos segundos de encolarse— y
// lo bastante corto como para no tragarse dos cambios reales seguidos.
const notifyDedupWindow = time.Minute

func (s *notificationService) CreateNotification(userID uint, notifType, title, message string, data map[string]interface{}) error {
	dataJSON := ""
	if data != nil {
		b, _ := json.Marshal(data)
		dataJSON = string(b)
	}

	// Deduplicación por (usuario, tarea) dentro de la ventana: gana el primero en
	// llegar. Se hace por tarea y no por tipo a propósito, porque el aviso nativo y
	// el de la regla llevan tipos distintos ("task_updated" vs "workflow") y
	// comparar el tipo no atraparía justo el caso que hay que atrapar.
	if taskID, ok := taskIDFrom(data); ok {
		recent, err := s.repo.ExistsRecentForTask(userID, taskID, time.Now().Add(-notifyDedupWindow))
		if err != nil {
			// Fallo al comprobar: se avisa igual. Un duplicado ocasional molesta;
			// perder el aviso porque la comprobación falló es peor.
			log.Printf("[notifications] no se pudo comprobar duplicados de la tarea %d para el usuario %d: %v", taskID, userID, err)
		} else if recent {
			return nil
		}
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
