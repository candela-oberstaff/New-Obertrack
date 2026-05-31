package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TicketHandler struct {
	DB       *gorm.DB
	wahaSvc  *service.WahaService
	brevoSvc *service.BrevoService
}

func NewTicketHandler(db *gorm.DB, wahaSvc *service.WahaService, brevoSvc *service.BrevoService) *TicketHandler {
	return &TicketHandler{
		DB:       db,
		wahaSvc:  wahaSvc,
		brevoSvc: brevoSvc,
	}
}

func (h *TicketHandler) GetTickets(c *gin.Context) {
	var tickets []models.Ticket
	requester, _ := c.Get("user_id")
	requesterID := uint(0)
	if v, ok := requester.(uint); ok {
		requesterID = v
	}
	isSuper := false
	if middleware.IsSuperadmin(c) {
		isSuper = true
	}

	query := h.DB.Preload("Contact").Preload("Assignee").Order("updated_at desc")
	if !isSuper {
		query = query.Where("assigned_to = ?", requesterID)
	}
	if err := query.Find(&tickets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets"})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

func (h *TicketHandler) GetTicket(c *gin.Context) {
	id := c.Param("id")
	var ticket models.Ticket
	if err := h.DB.Preload("Contact").Preload("Assignee").Preload("Messages").First(&ticket, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	requester, _ := c.Get("user_id")
	requesterID := uint(0)
	if v, ok := requester.(uint); ok {
		requesterID = v
	}
	if !middleware.IsSuperadmin(c) {
		if ticket.AssignedTo == nil || *ticket.AssignedTo != requesterID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	c.JSON(http.StatusOK, ticket)
}

func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	id := c.Param("id")
	var ticket models.Ticket

	if err := h.DB.First(&ticket, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	requester, _ := c.Get("user_id")
	requesterID := uint(0)
	if v, ok := requester.(uint); ok {
		requesterID = v
	}
	if !middleware.IsSuperadmin(c) {
		if ticket.AssignedTo == nil || *ticket.AssignedTo != requesterID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	var updateData struct {
		Stage      models.TicketStage `json:"stage"`
		Status     string             `json:"status"`
		AssignedTo *uint              `json:"assigned_to"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if updateData.Stage != "" {
		ticket.Stage = updateData.Stage
	}
	if updateData.Status != "" {
		ticket.Status = updateData.Status
	}
	if updateData.AssignedTo != nil {
		ticket.AssignedTo = updateData.AssignedTo
	}

	h.DB.Save(&ticket)
	c.JSON(http.StatusOK, ticket)
}

type SendMessageRequest struct {
	Content string                `json:"content"`
	Channel models.MessageChannel `json:"channel"`
}

func (h *TicketHandler) SendMessage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, _ := c.Get("user_id")

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var ticket models.Ticket
	if err := h.DB.Preload("Contact").First(&ticket, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	uid := userID.(uint)
	// Allow only assigned agent or superadmin to send on behalf of agent
	if !middleware.IsSuperadmin(c) {
		if ticket.AssignedTo == nil || *ticket.AssignedTo != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	// Send external message if not note
	var externalID string
	switch req.Channel {
	case models.ChannelWhatsApp:
		session := h.wahaSvc.GetSession()
		if err := h.wahaSvc.SendMessage(session, ticket.Contact.Phone, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send WAHA message"})
			return
		}
	case models.ChannelEmail:
		if err := h.brevoSvc.SendEmail(ticket.Contact.Email, ticket.Contact.Name, ticket.Title, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send Email"})
			return
		}
	}

	// Persist message
	msg := models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: models.SenderTypeAgent,
		SenderID:   &uid,
		Channel:    req.Channel,
		Content:    req.Content,
		ExternalID: externalID,
	}
	h.DB.Create(&msg)

	c.JSON(http.StatusOK, msg)
}
