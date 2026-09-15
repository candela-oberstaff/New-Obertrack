package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// Transferencia de chats de WhatsApp desde Obersuite. Un manager pasa una
// conversación a Customer Success y aquí aparece como ticket en la bandeja
// interna, con el mismo origen que las altas desde Obersuite. Misma tabla de
// códigos que /hire.
type ObersuiteTicketHandler struct {
	tickets service.TicketService
}

func NewObersuiteTicketHandler(tickets service.TicketService) *ObersuiteTicketHandler {
	return &ObersuiteTicketHandler{tickets: tickets}
}

type obersuiteTransferRequest struct {
	ExternalID        string     `json:"external_id"`
	CandidateName     string     `json:"candidate_name"`
	CandidatePhone    string     `json:"candidate_phone"`
	WahaChatID        string     `json:"waha_chat_id"`
	WahaSession       string     `json:"waha_session"`
	TransferredByName string     `json:"transferred_by_name"`
	TransferredAt     *time.Time `json:"transferred_at"`
	Reason            string     `json:"reason"`
	Context           string     `json:"context"`
	Source            string     `json:"source"`
	// Messages es LA fuente del chat: hasta 30, en orden cronológico. context
	// es solo respaldo legible y no se interpreta.
	Messages []obersuiteTransferMessage `json:"messages"`
}

type obersuiteTransferMessage struct {
	At   *time.Time `json:"at"`
	From string     `json:"from"`
	Text string     `json:"text"`
}

// Create es POST /api/integrations/obersuite/tickets.
func (h *ObersuiteTicketHandler) Create(c *gin.Context) {
	var req obersuiteTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el cuerpo no es JSON válido: " + err.Error()})
		return
	}
	msgs := make([]service.ObersuiteTransferMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, service.ObersuiteTransferMessage{At: m.At, From: m.From, Text: m.Text})
	}
	res, err := h.tickets.CreateObersuiteTransfer(service.ObersuiteTransferInput{
		ExternalID:        req.ExternalID,
		CandidateName:     req.CandidateName,
		CandidatePhone:    req.CandidatePhone,
		WahaChatID:        req.WahaChatID,
		WahaSession:       req.WahaSession,
		TransferredByName: req.TransferredByName,
		TransferredAt:     req.TransferredAt,
		Reason:            req.Reason,
		Context:           req.Context,
		Source:            req.Source,
		Messages:          msgs,
	})
	if err != nil {
		status, msg := hireStatus(err)
		if status >= 500 {
			rid := hireRequestID()
			log.Printf("[Obersuite] transferencia de chat falló (external_id=%q) request_id=%s: %v", req.ExternalID, rid, err)
			c.JSON(status, gin.H{
				"error":      "no se pudo crear el ticket por un problema en Obertrack. Vuelve a intentarlo en un momento.",
				"request_id": rid,
			})
			return
		}
		log.Printf("[Obersuite] transferencia de chat rechazada %d (external_id=%q): %s", status, req.ExternalID, msg)
		c.JSON(status, gin.H{"error": msg})
		return
	}
	log.Printf("[Obersuite] transferencia de chat external_id=%q → %s (ticket=%d)", req.ExternalID, res.Status, res.ID)
	c.JSON(http.StatusOK, res)
}
