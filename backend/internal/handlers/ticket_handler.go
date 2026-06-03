package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TicketHandler struct {
	svc service.TicketService
}

func NewTicketHandler(svc service.TicketService) *TicketHandler {
	return &TicketHandler{svc: svc}
}

// ticketErrorStatus maps domain errors to HTTP status codes.
func ticketErrorStatus(err error) int {
	switch {
	case errors.Is(err, apperrors.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apperrors.ErrAccessDenied):
		return http.StatusForbidden
	case errors.Is(err, apperrors.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, apperrors.ErrExternalSend):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func (h *TicketHandler) GetTickets(c *gin.Context) {
	if middleware.GetUserRole(c) != string(models.UserTypeCustomerSuccess) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}
	tickets, err := h.svc.List(middleware.GetUserID(c), middleware.GetUserRole(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets"})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

func (h *TicketHandler) GetTicket(c *gin.Context) {
	if middleware.GetUserRole(c) != string(models.UserTypeCustomerSuccess) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	ticket, err := h.svc.Get(uint(id), middleware.GetUserID(c), middleware.GetUserRole(c))
	if err != nil {
		c.JSON(ticketErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ticket)
}

func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	if middleware.GetUserRole(c) != string(models.UserTypeCustomerSuccess) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Stage      models.TicketStage `json:"stage"`
		Status     string             `json:"status"`
		AssignedTo *uint              `json:"assigned_to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket, err := h.svc.Update(uint(id), middleware.GetUserID(c), middleware.GetUserRole(c), req.Stage, req.Status, req.AssignedTo)
	if err != nil {
		c.JSON(ticketErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ticket)
}

func (h *TicketHandler) SendMessage(c *gin.Context) {
	if middleware.GetUserRole(c) != string(models.UserTypeCustomerSuccess) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req struct {
		Content string                `json:"content"`
		Channel models.MessageChannel `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	msg, err := h.svc.SendAgentMessage(uint(id), middleware.GetUserID(c), middleware.GetUserRole(c), req.Content, req.Channel)
	if err != nil {
		c.JSON(ticketErrorStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, msg)
}
