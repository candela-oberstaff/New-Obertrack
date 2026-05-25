package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/websocket"
)

type WahaWebhookPayload struct {
	Event   string `json:"event"`
	Session string `json:"session"`
	Payload struct {
		ID        string `json:"id"`
		From      string `json:"from"`
		To        string `json:"to"`
		Body      string `json:"body"`
		Type      string `json:"type"`
		FromMe    bool   `json:"fromMe"`
		Timestamp int64  `json:"timestamp"`
	} `json:"payload"`
}

type WahaHandler struct {
	DB *gorm.DB
}

func NewWahaHandler(db *gorm.DB) *WahaHandler {
	return &WahaHandler{
		DB: db,
	}
}

func (h *WahaHandler) HandleWebhook(c *gin.Context) {
	var payload WahaWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Only process incoming text messages
	if payload.Event != "message" || payload.Payload.FromMe || payload.Payload.Type != "chat" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	phone := strings.Split(payload.Payload.From, "@")[0]
	
	// 1. Find or create Contact
	var contact models.Contact
	res := h.DB.Where("phone = ?", phone).First(&contact)
	if res.Error != nil {
		// Create new contact
		contact = models.Contact{
			Phone: phone,
			Name:  "WA User " + phone, // Default name
		}
		h.DB.Create(&contact)
	}

	// 2. Find open Ticket for this contact, or create a new one
	var ticket models.Ticket
	res = h.DB.Where("contact_id = ? AND status = ?", contact.ID, "open").First(&ticket)
	if res.Error != nil {
		// Create new ticket
		ticket = models.Ticket{
			ContactID: contact.ID,
			Title:     "WA: " + phone,
			Stage:     models.StageNew,
			Status:    "open",
		}
		h.DB.Create(&ticket)
	}

	// 3. Create TicketMessage
	msg := models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: models.SenderTypeContact,
		Channel:    models.ChannelWhatsApp,
		Content:    payload.Payload.Body,
		ExternalID: payload.Payload.ID,
	}
	h.DB.Create(&msg)

	// 4. Notify via Websockets (to all managers/admins or the assigned user)
	websocket.GlobalNotifHub.BroadcastToAll("new_ticket_message", gin.H{
		"ticket_id": ticket.ID,
		"message":   msg,
	})

	log.Printf("Received WAHA message from %s for ticket %d", phone, ticket.ID)
	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
