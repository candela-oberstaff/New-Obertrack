package handlers

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
	"gorm.io/gorm"
)

// WhatsAppHandler handles the dedicated WhatsApp‑chat section.
// It exposes per‑agent and unassigned ticket lists, conversation threads,
// ticket assignment, and message sending – all backed by Zoho Desk.
type WhatsAppHandler struct {
	DB      *gorm.DB
	zohoSvc *service.ZohoService
}

func NewWhatsAppHandler(db *gorm.DB, zohoSvc *service.ZohoService) *WhatsAppHandler {
	return &WhatsAppHandler{DB: db, zohoSvc: zohoSvc}
}

func canUseWhatsAppInbox(c *gin.Context) bool {
	return middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess)
}

// ─── DTOs ────────────────────────────────────────────────────────────────────

// WhatsAppTicketDTO is the shape returned for each chat preview in the list.
type WhatsAppTicketDTO struct {
	ZohoID       string    `json:"zoho_id"`
	ContactName  string    `json:"contact_name"`
	ContactPhone string    `json:"contact_phone"`
	Subject      string    `json:"subject"`
	Status       string    `json:"status"`
	AssigneeID   string    `json:"assignee_id,omitempty"`
	ModifiedTime time.Time `json:"modified_time"`
}

// WhatsAppMessageDTO is a single chat bubble.
type WhatsAppMessageDTO struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	Direction   string    `json:"direction"` // "incoming" | "outgoing"
	AuthorName  string    `json:"author_name"`
	AuthorType  string    `json:"author_type"` // "contact" | "agent" | "system"
	CreatedTime time.Time `json:"created_time"`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// resolveZohoAgentID reads the current user's ZohoAgentID from the DB.
// It lazily fetches it from Zoho when missing, using the user's email.
func (h *WhatsAppHandler) resolveZohoAgentID(c *gin.Context) (string, error) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return "", err
	}

	if user.ZohoAgentID != "" {
		return user.ZohoAgentID, nil
	}

	// Lazy-sync: match by email against Zoho Desk agents
	zohoID, _, err := h.zohoSvc.GetAgentByEmail(user.Email)
	if err != nil {
		log.Printf("[WhatsApp] Could not sync Zoho agent ID for user %d (%s): %v", userID, user.Email, err)
		return "", err
	}

	// Persist for subsequent requests
	h.DB.Model(&user).Update("zoho_agent_id", zohoID)
	return zohoID, nil
}

func mapTicketToDTO(t service.ZohoTicket) WhatsAppTicketDTO {
	return WhatsAppTicketDTO{
		ZohoID:       t.ID,
		ContactName:  t.ContactName,
		ContactPhone: t.Phone,
		Subject:      t.Subject,
		Status:       t.Status,
		AssigneeID:   t.AssigneeID,
		ModifiedTime: t.ModifiedTime,
	}
}

// ─── GET /api/chats/me ────────────────────────────────────────────────────────

// GetMyChats returns the WhatsApp tickets assigned to the logged‑in agent.
func (h *WhatsAppHandler) GetMyChats(c *gin.Context) {
	if !canUseWhatsAppInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}

	zohoAgentID, err := h.resolveZohoAgentID(c)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "No se pudo obtener tu ID de agente en Zoho. Asegurate de que tu correo esté registrado allí.",
		})
		return
	}

	modifiedSince := c.Query("modifiedSince")
	var modifiedTimeRange string
	if modifiedSince != "" {
		// Expects ISO8601, e.g. 2026-06-01T14:30:00.000Z
		// Zoho wants modifiedTimeRange as comma-separated: start,end
		// We use current time as end.
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		modifiedTimeRange = modifiedSince + "," + now
	}

	tickets, err := h.zohoSvc.ListWhatsAppTickets(zohoAgentID, "open", modifiedTimeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al consultar Zoho Desk: " + err.Error()})
		return
	}

	dtos := make([]WhatsAppTicketDTO, 0, len(tickets))
	for _, t := range tickets {
		dtos = append(dtos, mapTicketToDTO(t))
	}

	// Ensure front‑end receives them newest‑first
	sort.Slice(dtos, func(i, j int) bool {
		return dtos[i].ModifiedTime.After(dtos[j].ModifiedTime)
	})

	c.JSON(http.StatusOK, dtos)
}

// ─── GET /api/chats/unassigned ───────────────────────────────────────────────

// GetUnassignedChats returns incoming WhatsApp tickets not yet assigned to any agent.
func (h *WhatsAppHandler) GetUnassignedChats(c *gin.Context) {
	if !canUseWhatsAppInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}

	modifiedSince := c.Query("modifiedSince")
	var modifiedTimeRange string
	if modifiedSince != "" {
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		modifiedTimeRange = modifiedSince + "," + now
	}

	tickets, err := h.zohoSvc.ListWhatsAppTickets("unassigned", "open", modifiedTimeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al consultar Zoho Desk: " + err.Error()})
		return
	}

	dtos := make([]WhatsAppTicketDTO, 0, len(tickets))
	for _, t := range tickets {
		dtos = append(dtos, mapTicketToDTO(t))
	}

	sort.Slice(dtos, func(i, j int) bool {
		return dtos[i].ModifiedTime.After(dtos[j].ModifiedTime)
	})

	c.JSON(http.StatusOK, dtos)
}

// ─── GET /api/chats/:ticketId/messages ───────────────────────────────────────

// GetMessages returns the full conversation thread for a ticket.
// It uses the /conversations endpoint (GetTicketThreads) for the clean chat log.
func (h *WhatsAppHandler) GetMessages(c *gin.Context) {
	if !canUseWhatsAppInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}

	ticketID := c.Param("ticketId")
	if ticketID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticketId es requerido"})
		return
	}

	threads, err := h.zohoSvc.GetTicketThreads(ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener conversación: " + err.Error()})
		return
	}

	var messages []WhatsAppMessageDTO

	for _, thread := range threads {
		// Try to get individual sub‑messages (richer granularity for IM channels)
		subMsgs, err := h.zohoSvc.GetThreadMessages(thread.ID)
		if err == nil && len(subMsgs) > 0 {
			for _, sub := range subMsgs {
				if sub.Summary == "" {
					continue
				}

				direction := "incoming"
				authorType := "contact"
				if sub.Author != nil && strings.EqualFold(sub.Author.Type, "AGENT") {
					direction = "outgoing"
					authorType = "agent"
				} else if strings.EqualFold(sub.Direction, "OUT") {
					direction = "outgoing"
					authorType = "agent"
				}

				authorName := ""
				if sub.Author != nil {
					authorName = sub.Author.Name
				}

				messages = append(messages, WhatsAppMessageDTO{
					ID:          sub.ID,
					Content:     stripHTMLWA(sub.Summary),
					Direction:   direction,
					AuthorName:  authorName,
					AuthorType:  authorType,
					CreatedTime: sub.CreatedTime,
				})
			}
		} else {
			// Fallback: use thread‑level data
			direction := "incoming"
			authorType := "contact"
			switch strings.ToLower(thread.AuthorType) {
			case "agent":
				direction = "outgoing"
				authorType = "agent"
			case "system":
				authorType = "system"
			}

			content := thread.Content
			if content == "" {
				content = thread.Summary
			}
			if content == "" {
				continue
			}

			// Determine channel from thread.Channel for consistency
			_ = thread.Channel // Keep for reference, though WhatsApp section is typically WhatsApp

			messages = append(messages, WhatsAppMessageDTO{
				ID:          thread.ID,
				Content:     stripHTMLWA(content),
				Direction:   direction,
				AuthorName:  thread.AuthorName,
				AuthorType:  authorType,
				CreatedTime: thread.CreatedTime,
			})
		}
	}

	// Chronological order (oldest first, like WhatsApp)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedTime.Before(messages[j].CreatedTime)
	})

	c.JSON(http.StatusOK, messages)
}

// ─── PATCH /api/chats/:ticketId/assign ───────────────────────────────────────

// AssignToMe assigns an unassigned ticket to the current agent.
func (h *WhatsAppHandler) AssignToMe(c *gin.Context) {
	if !canUseWhatsAppInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}

	ticketID := c.Param("ticketId")
	if ticketID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticketId es requerido"})
		return
	}

	zohoAgentID, err := h.resolveZohoAgentID(c)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "No se pudo obtener tu ID de agente en Zoho.",
		})
		return
	}

	if err := h.zohoSvc.AssignTicket(ticketID, zohoAgentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo asignar el ticket: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Chat asignado exitosamente",
		"ticket_id":     ticketID,
		"zoho_agent_id": zohoAgentID,
	})
}

// ─── POST /api/chats/:ticketId/send ──────────────────────────────────────────

type WhatsAppSendRequest struct {
	Content string `json:"content" binding:"required"`
}

// SendMessage sends a message through Zoho Desk's public comment API,
// which routes it back to the client via the ticket's active channel (WhatsApp).
func (h *WhatsAppHandler) SendMessage(c *gin.Context) {
	if !canUseWhatsAppInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to Customer Success role"})
		return
	}

	ticketID := c.Param("ticketId")
	if ticketID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticketId es requerido"})
		return
	}

	var req WhatsAppSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El campo 'content' es requerido"})
		return
	}

	thread, err := h.zohoSvc.ReplyWhatsAppLiveChat(ticketID, req.Content, c.GetString("email"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "No se pudo enviar el mensaje por Zoho: " + err.Error()})
		return
	}

	createdAt := time.Now()
	if thread != nil && !thread.CreatedTime.IsZero() {
		createdAt = thread.CreatedTime
	}

	c.JSON(http.StatusOK, WhatsAppMessageDTO{
		ID:          thread.ID,
		Content:     req.Content,
		Direction:   "outgoing",
		AuthorName:  "",
		AuthorType:  "agent",
		CreatedTime: createdAt,
	})
}

// ─── Sync endpoint ────────────────────────────────────────────────────────────

// SyncAgentID is an optional convenience endpoint to force‑refresh the
// ZohoAgentID for the current user (useful after a new agent is created in Zoho).
func (h *WhatsAppHandler) SyncAgentID(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
		return
	}

	zohoID, info, err := h.zohoSvc.GetAgentByEmail(user.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No se encontró un agente en Zoho con ese correo",
			"details": err.Error(),
		})
		return
	}

	h.DB.Model(&user).Update("zoho_agent_id", zohoID)

	c.JSON(http.StatusOK, gin.H{
		"zoho_agent_id": zohoID,
		"agent_name":    info.Name,
		"agent_email":   info.Email,
	})
}

// stripHTMLWA is a local copy of stripHTML for this package (avoids cross-package import).
func stripHTMLWA(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")

	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}
