package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/websocket"
)

type BrevoInboundPayload struct {
	Items []struct {
		From struct {
			Address string `json:"Address"`
			Name    string `json:"Name"`
		} `json:"From"`
		To []struct {
			Address string `json:"Address"`
		} `json:"To"`
		Subject   string `json:"Subject"`
		TextBody  string `json:"TextBody"`
		MessageId string `json:"MessageId"`
	} `json:"items"`
}

type BrevoInboundHandler struct {
	DB *gorm.DB
}

func NewBrevoInboundHandler(db *gorm.DB) *BrevoInboundHandler {
	return &BrevoInboundHandler{
		DB: db,
	}
}

func (h *BrevoInboundHandler) HandleInbound(c *gin.Context) {
	var payload BrevoInboundPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	for _, item := range payload.Items {
		email := item.From.Address
		name := item.From.Name

		if email == "" {
			continue
		}

		// 1. Find or create Contact
		var contact models.Contact
		res := h.DB.Where("email = ?", email).First(&contact)
		if res.Error != nil {
			// Create new contact
			contact = models.Contact{
				Email: email,
				Name:  name,
			}
			if contact.Name == "" {
				contact.Name = email
			}
			h.DB.Create(&contact)
		}

		// 2. Find open Ticket for this contact, or create a new one
		// We could use the Subject to find an exact ticket, but simple logic for now: open ticket per contact
		var ticket models.Ticket
		res = h.DB.Where("contact_id = ? AND status = ?", contact.ID, "open").First(&ticket)
		if res.Error != nil {
			// Create new ticket
			ticket = models.Ticket{
				ContactID: contact.ID,
				Title:     item.Subject,
				Stage:     models.StageNew,
				Status:    "open",
			}
			if ticket.Title == "" {
				ticket.Title = "Email from " + email
			}
			h.DB.Create(&ticket)
		}

		// 3. Create TicketMessage
		msg := models.TicketMessage{
			TicketID:   ticket.ID,
			SenderType: models.SenderTypeContact,
			Channel:    models.ChannelEmail,
			Content:    item.TextBody,
			ExternalID: item.MessageId,
		}
		h.DB.Create(&msg)

		// 4. Notify via Websockets
		websocket.GlobalNotifHub.BroadcastToAll("new_ticket_message", gin.H{
			"ticket_id": ticket.ID,
			"message":   msg,
		})

		log.Printf("Received Inbound Email from %s for ticket %d", email, ticket.ID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
