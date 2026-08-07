package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/websocket"
)

type NotificationHandler struct {
	svc     service.NotificationService
	pushSvc *service.WebPushService
}

func NewNotificationHandler(svc service.NotificationService, pushSvc *service.WebPushService) *NotificationHandler {
	return &NotificationHandler{svc: svc, pushSvc: pushSvc}
}

// GetPushKey entrega la clave pública VAPID con la que el navegador se
// suscribe a Web Push. Vacía = la función no está disponible.
func (h *NotificationHandler) GetPushKey(c *gin.Context) {
	key := ""
	if h.pushSvc != nil {
		key = h.pushSvc.PublicKey()
	}
	c.JSON(http.StatusOK, gin.H{"public_key": key})
}

// SubscribePush registra la suscripción Web Push del navegador del usuario.
func (h *NotificationHandler) SubscribePush(c *gin.Context) {
	if h.pushSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Web Push no está disponible"})
		return
	}
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
		Keys     struct {
			P256dh string `json:"p256dh" binding:"required"`
			Auth   string `json:"auth" binding:"required"`
		} `json:"keys" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Suscripción inválida"})
		return
	}
	if err := h.pushSvc.Subscribe(middleware.GetUserID(c), req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar la suscripción"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Suscripción registrada"})
}

// UnsubscribePush elimina la suscripción del navegador (p. ej. al desactivar).
func (h *NotificationHandler) UnsubscribePush(c *gin.Context) {
	if h.pushSvc == nil {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Falta el endpoint"})
		return
	}
	if err := h.pushSvc.Unsubscribe(middleware.GetUserID(c), req.Endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo eliminar la suscripción"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Suscripción eliminada"})
}

func (h *NotificationHandler) HandleWebSocket(c *gin.Context) {
	userID := middleware.GetUserID(c)

	conn, err := websocket.NotificationUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	websocket.GlobalNotifHub.HandleConnection(conn, userID)
}

func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID := middleware.GetUserID(c)

	notifications, err := h.svc.GetNotifications(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := h.svc.MarkAsRead(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if err := h.svc.MarkAllAsRead(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}

func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	count, err := h.svc.GetUnreadCount(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get unread count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}
