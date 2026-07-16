package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
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
	ticketSvc service.TicketService
}

func NewWahaHandler(ticketSvc service.TicketService) *WahaHandler {
	return &WahaHandler{ticketSvc: ticketSvc}
}

func (h *WahaHandler) HandleWebhook(c *gin.Context) {
	// Read raw body (do NOT log it: webhook payloads contain personal data).
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading WAHA webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var payload WahaWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Printf("WAHA Webhook unmarshal error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	from := payload.Payload.From
	if payload.Event != "message" ||
		payload.Payload.FromMe ||
		strings.Contains(from, "status@broadcast") ||
		strings.TrimSpace(payload.Payload.Body) == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	if err := h.ticketSvc.IngestWhatsApp(payload.Session, payload.Payload.From, payload.Payload.Body, payload.Payload.ID); err != nil {
		log.Printf("WAHA ingest error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
