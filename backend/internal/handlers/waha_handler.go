package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"


	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
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
	DB      *gorm.DB
	wahaSvc *service.WahaService
}

func NewWahaHandler(db *gorm.DB, wahaSvc *service.WahaService) *WahaHandler {
	return &WahaHandler{
		DB:      db,
		wahaSvc: wahaSvc,
	}
}

func (h *WahaHandler) HandleWebhook(c *gin.Context) {
	// Read raw body for logging
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading WAHA webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	// Log the raw JSON received
	log.Printf("PAYLOAD DE WAHA RECIBIDO: %s", string(bodyBytes))

	// Restore the body so it can be read again if needed
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var payload WahaWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Printf("WAHA Webhook unmarshal error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload", "details": err.Error()})
		return
	}

	// Only process incoming text messages (Corregido)
	if payload.Event != "message" || payload.Payload.FromMe {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	phone := strings.Split(payload.Payload.From, "@")[0]
	
	// Try to fetch actual name and phone from WAHA
	resolvedName := "WA User " + phone
	wahaContact, err := h.wahaSvc.GetContact(payload.Session, payload.Payload.From)
	if err == nil && wahaContact != nil {
		if wahaContact.Name != "" {
			resolvedName = wahaContact.Name
		}
		if wahaContact.Phone != "" {
			phone = wahaContact.Phone
		}
	} else {
		log.Printf("Could not fetch WAHA contact info for %s: %v", payload.Payload.From, err)
	}

	// 1. Find or create Contact
	var contact models.Contact
	res := h.DB.Where("phone = ?", phone).First(&contact)
	if res.Error != nil {
		// Create new contact
		contact = models.Contact{
			Phone: phone,
			Name:  resolvedName,
		}
		h.DB.Create(&contact)
	} else if contact.Name == "WA User "+phone && resolvedName != "WA User "+phone {
		// Update contact name if it was previously generic
		contact.Name = resolvedName
		h.DB.Save(&contact)
	}

	// 2. Find open Ticket for this contact updated in the last hour, or create a new one
	var ticket models.Ticket
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	res = h.DB.Where("contact_id = ? AND status = ? AND updated_at >= ?", contact.ID, "open", oneHourAgo).Order("updated_at desc").First(&ticket)
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

	// Touch/update Ticket's updated_at timestamp to extend the 1-hour window for consecutive messages
	h.DB.Model(&ticket).Update("updated_at", time.Now())

	// 4. Notify via Websockets (to all managers/admins or the assigned user)
	websocket.GlobalNotifHub.BroadcastToAll("new_ticket_message", gin.H{
		"ticket_id": ticket.ID,
		"message":   msg,
	})

	log.Printf("Received WAHA message from %s (%s) for ticket %d", contact.Name, fmt.Sprintf("+%s", phone), ticket.ID)
	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
