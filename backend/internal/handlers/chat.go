package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/websocket"
)

type ChatHandler struct {
	svc service.ChatService
	hub *websocket.ChatHub
}

func NewChatHandler(svc service.ChatService, hub *websocket.ChatHub) *ChatHandler {
	return &ChatHandler{svc: svc, hub: hub}
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := middleware.GetEmpleadorID(c)

	messages, err := h.svc.GetMessages(userID, role, isSuperadmin, empleadorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	empleadorID := middleware.GetEmpleadorID(c)

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.svc.SendMessage(userID, role, empleadorID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChatWSMessage{
		Type:      response.Type,
		Content:   response.Content,
		UserID:    response.UserID,
		CompanyID: response.CompanyID,
	})

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

func (h *ChatHandler) HandleWebSocket(c *gin.Context) {
	userID := middleware.GetUserID(c)
	h.hub.HandleConnection(c.Writer, c.Request, userID)
}
