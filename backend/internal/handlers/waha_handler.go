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

	// Only process incoming text/emoji messages.
	// We ignore statuses (event "message" but From is "status@broadcast") and non-chat types like reactions.
	if payload.Event != "message" || payload.Payload.FromMe || strings.Contains(payload.Payload.From, "status") || payload.Payload.Type != "chat" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// Raw WhatsApp ID (e.g. "275904655822850@lid" or "5491122334455@c.us")
	waID := payload.Payload.From
	// Numeric part only (used as a phone placeholder until we resolve the real number)
	phonePart := strings.Split(waID, "@")[0]

	// 1. Find or create Contact by wa_id
	var contact models.Contact
	res := h.DB.Where("wa_id = ?", waID).First(&contact)
	if res.Error != nil {
		// Also try by phone in case it was created before wa_id was tracked
		res = h.DB.Where("phone = ?", phonePart).First(&contact)
	}

	// Look up in users table to check if this phone number belongs to a professional or employer/company
	var dbUser models.User
	userFound := false
	// Clean both numbers to compare (ignoring non-digits like '+')
	cleanPhonePart := strings.TrimLeft(phonePart, "+")
	if h.DB.Where("REPLACE(phone_number, '+', '') = ?", cleanPhonePart).First(&dbUser).Error == nil {
		userFound = true
	}

	if res.Error != nil {
		// Contact does not exist. Create new one.
		contact = models.Contact{
			WaID:  waID,
			Phone: phonePart,
			Name:  "WA User " + phonePart,
		}
		if userFound {
			contact.Name = dbUser.Name
			if dbUser.UserType == models.UserTypeEmployer {
				contact.CompanyName = dbUser.CompanyName
			} else if dbUser.UserType == models.UserTypeProfessional {
				contact.CompanyName = "Profesional: " + dbUser.JobTitle
			}
		}
		h.DB.Create(&contact)
	} else {
		// Contact exists. Update its info if it matched a registered user now
		updates := map[string]interface{}{}
		if contact.WaID == "" {
			updates["wa_id"] = waID
		}
		if userFound {
			if strings.HasPrefix(contact.Name, "WA User ") {
				updates["name"] = dbUser.Name
			}
			if contact.CompanyName == "" {
				if dbUser.UserType == models.UserTypeEmployer {
					updates["company_name"] = dbUser.CompanyName
				} else if dbUser.UserType == models.UserTypeProfessional {
					updates["company_name"] = "Profesional: " + dbUser.JobTitle
				}
			}
		}
		if len(updates) > 0 {
			h.DB.Model(&contact).Updates(updates)
		}
	}

	// 2. Find open Ticket for this contact updated in the last hour, or create a new one
	var ticket models.Ticket
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	res = h.DB.Where("contact_id = ? AND status = ? AND updated_at >= ?", contact.ID, "open", oneHourAgo).Order("updated_at desc").First(&ticket)
	if res.Error != nil {
		// Create new ticket
		ticket = models.Ticket{
			ContactID: contact.ID,
			Title:     "WA: " + phonePart,
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

	log.Printf("Received WAHA message from %s (%s) for ticket %d", contact.Name, fmt.Sprintf("+%s", phonePart), ticket.ID)
	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
