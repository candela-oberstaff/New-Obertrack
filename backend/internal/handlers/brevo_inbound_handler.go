package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
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
	ticketSvc service.TicketService
}

func NewBrevoInboundHandler(ticketSvc service.TicketService) *BrevoInboundHandler {
	return &BrevoInboundHandler{ticketSvc: ticketSvc}
}

func (h *BrevoInboundHandler) HandleInbound(c *gin.Context) {
	var payload BrevoInboundPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	for _, item := range payload.Items {
		if item.From.Address == "" {
			continue
		}
		if err := h.ticketSvc.IngestEmail(item.From.Address, item.From.Name, item.Subject, item.TextBody, item.MessageId); err != nil {
			log.Printf("Brevo inbound ingest error for %s: %v", item.From.Address, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
