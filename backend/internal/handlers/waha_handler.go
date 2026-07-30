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
		HasMedia  bool   `json:"hasMedia"`
		Media     *struct {
			Mimetype string `json:"mimetype"`
		} `json:"media"`
	} `json:"payload"`
}

func (p *WahaWebhookPayload) mimeType() string {
	if p.Payload.Media == nil {
		return ""
	}
	return p.Payload.Media.Mimetype
}

type WahaHandler struct {
	ticketSvc service.TicketService
	wahaSvc   *service.WahaService
}

func NewWahaHandler(ticketSvc service.TicketService, wahaSvc *service.WahaService) *WahaHandler {
	return &WahaHandler{ticketSvc: ticketSvc, wahaSvc: wahaSvc}
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

	// La instancia de WAHA puede alojar varias sesiones y el HMAC es compartido
	// entre todas: sin este filtro, los mensajes de una sesión ajena entrarían al
	// mismo buzón de tickets.
	if expected := h.wahaSvc.GetSession(); payload.Session != expected {
		log.Printf("WAHA webhook: sesión inesperada %q (configurada: %q) — ignorado", payload.Session, expected)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// Se aceptan tanto 'message' (entrantes) como 'message.any', que además trae lo
	// enviado desde el teléfono. Sin esto el hilo quedaba a media conversación: se
	// veía lo que escribía el contacto pero no las respuestas dadas desde el móvil.
	if payload.Event != "message" && payload.Event != "message.any" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// El interlocutor es el contacto del otro lado: en un saliente, el destinatario.
	peer := payload.Payload.From
	if payload.Payload.FromMe {
		peer = payload.Payload.To
	}
	if peer == "" || strings.Contains(peer, "status@broadcast") || strings.HasSuffix(peer, "@g.us") {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// Un adjunto sin texto llega con body vacío. En vez de descartarlo (el operador
	// nunca se enteraría de que el contacto mandó una foto o un documento), se
	// ingesta con un marcador legible.
	body := strings.TrimSpace(payload.Payload.Body)
	if body == "" {
		body = service.MediaPlaceholder(payload.Payload.Type, payload.mimeType(), payload.Payload.HasMedia)
	}
	if body == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	if err := h.ticketSvc.IngestWhatsApp(payload.Session, peer, body, payload.Payload.ID, payload.Payload.FromMe); err != nil {
		log.Printf("WAHA ingest error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
