package handlers

import (
	"hash/fnv"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TicketHandler struct {
	db      *gorm.DB
	zohoSvc *service.ZohoService
}

func NewTicketHandler(db *gorm.DB, zohoSvc *service.ZohoService) *TicketHandler {
	return &TicketHandler{db: db, zohoSvc: zohoSvc}
}

func canUseSupportInbox(c *gin.Context) bool {
	return middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess)
}

func hashStringToUint(s string) uint {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return uint(h.Sum32())
}

func stageFromZoho(statusType, status string) models.TicketStage {
	switch strings.ToLower(statusType) {
	case "closed":
		return models.StageClosed
	case "onhold", "on hold":
		return models.StageWaiting
	case "open":
		return models.StageInProgress
	}

	switch strings.ToLower(status) {
	case "closed", "cerrado":
		return models.StageClosed
	case "on hold", "onhold", "esperando":
		return models.StageWaiting
	case "open", "abierto", "en progreso":
		return models.StageInProgress
	default:
		return models.StageNew
	}
}

func statusFromStage(stage models.TicketStage) string {
	switch stage {
	case models.StageClosed:
		return "Closed"
	case models.StageWaiting:
		return "On Hold"
	case models.StageInProgress, models.StageNew:
		return "Open"
	default:
		return ""
	}
}

func ticketStatusOptionFromZoho(status service.ZohoTicketStatus) gin.H {
	return gin.H{
		"value":       status.Value,
		"label":       status.Label,
		"status_type": status.StatusType,
		"stage":       stageFromZoho(status.StatusType, status.Value),
	}
}

func ticketDTOFromZoho(zt service.ZohoTicket, messages []models.TicketMessage) gin.H {
	contactName := strings.TrimSpace(zt.ContactName)
	contactPhone := firstNonEmptyLocal(zt.Phone, zt.ContactPhone)
	if looksLikePhoneLocal(contactName) {
		if contactPhone == "" {
			contactPhone = contactName
		}
		contactName = ""
	}
	contactEmail := firstNonEmptyLocal(zt.Email, zt.ContactEmail)

	return gin.H{
		"id":             hashStringToUint(zt.ID),
		"zoho_id":        zt.ID,
		"contact_id":     hashStringToUint(zt.ContactID),
		"ticket_number":  zt.TicketNumber,
		"title":          zt.Subject,
		"channel":        zt.Channel,
		"stage":          stageFromZoho(zt.StatusType, zt.Status),
		"status":         zt.Status,
		"priority":       zt.Priority,
		"category":       zt.Category,
		"description":    zt.Description,
		"sentiment":      zt.Sentiment,
		"customer_tone":  zt.CustomerTone,
		"is_escalated":   zt.IsEscalated,
		"web_url":        zt.WebURL,
		"assignee_id":    zt.AssigneeID,
		"assignee_name":  zt.AssigneeName,
		"assignee_email": zt.AssigneeEmail,
		"created_at":     zt.CreatedTime,
		"updated_at":     zt.ModifiedTime,
		"contact": gin.H{
			"id":    hashStringToUint(zt.ContactID),
			"name":  contactName,
			"phone": contactPhone,
			"email": contactEmail,
		},
		"messages": messages,
	}
}

func firstNonEmptyLocal(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func looksLikePhoneLocal(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	digits := 0
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits++
			continue
		}
		if !strings.ContainsRune("+ -().", r) {
			return false
		}
	}
	return digits >= 7
}

func linkedUserByPhone(db *gorm.DB, phone string) *models.User {
	clean := strings.NewReplacer("+", "", " ", "", "-", "", "(", "", ")", "").Replace(phone)
	if clean == "" {
		return nil
	}

	var user models.User
	if err := db.Where(
		"REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(phone_number, '+', ''), ' ', ''), '-', ''), '(', ''), ')', '') = ?",
		clean,
	).First(&user).Error; err != nil {
		return nil
	}
	return &user
}

func (h *TicketHandler) GetTickets(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	zohoTickets, err := h.zohoSvc.ListTickets()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch tickets from Zoho Desk: " + err.Error()})
		return
	}

	tickets := make([]gin.H, 0, len(zohoTickets))
	for _, zt := range zohoTickets {
		tickets = append(tickets, ticketDTOFromZoho(zt, nil))
	}

	c.JSON(http.StatusOK, tickets)
}

func (h *TicketHandler) GetTicketStatuses(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	zohoStatuses, err := h.zohoSvc.ListTicketStatuses()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket statuses from Zoho Desk: " + err.Error()})
		return
	}

	statuses := make([]gin.H, 0, len(zohoStatuses))
	for _, status := range zohoStatuses {
		statuses = append(statuses, ticketStatusOptionFromZoho(status))
	}
	c.JSON(http.StatusOK, statuses)
}

func (h *TicketHandler) GetTicket(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	ticketID := c.Param("id")
	zt, err := h.zohoSvc.GetTicketDetail(ticketID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket from Zoho Desk: " + err.Error()})
		return
	}

	threads, err := h.zohoSvc.GetTicketThreads(ticketID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket messages from Zoho Desk: " + err.Error()})
		return
	}

	messages := make([]models.TicketMessage, 0, len(threads))
	for _, thread := range threads {
		content := thread.Content
		if content == "" {
			content = thread.Summary
		}
		if content == "" {
			continue
		}

		senderType := models.SenderTypeContact
		switch strings.ToLower(thread.AuthorType) {
		case "agent":
			senderType = models.SenderTypeAgent
		case "system":
			senderType = models.SenderTypeSystem
		}

		channel := models.ChannelWhatsApp
		if strings.EqualFold(thread.Channel, "email") {
			channel = models.ChannelEmail
		}

		messages = append(messages, models.TicketMessage{
			ID:         hashStringToUint(thread.ID),
			TicketID:   hashStringToUint(ticketID),
			SenderType: senderType,
			Channel:    channel,
			Content:    stripHTML(content),
			CreatedAt:  thread.CreatedTime,
		})
	}

	phone := zt.Phone
	if phone == "" {
		phone = zt.ContactPhone
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket":      ticketDTOFromZoho(*zt, messages),
		"linked_user": linkedUserByPhone(h.db, phone),
		"zoho_id":     zt.ID,
	})
}

func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	var req struct {
		Stage      models.TicketStage `json:"stage"`
		Status     string             `json:"status"`
		AssigneeID string             `json:"assignee_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := req.Status
	if status == "" {
		status = statusFromStage(req.Stage)
	}
	if strings.TrimSpace(status) == "" && strings.TrimSpace(req.AssigneeID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No update fields provided"})
		return
	}

	if err := h.zohoSvc.UpdateTicketStatus(c.Param("id"), "", status, req.AssigneeID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to update ticket in Zoho Desk: " + err.Error()})
		return
	}

	zt, err := h.zohoSvc.GetTicketDetail(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"zoho_id": c.Param("id"), "stage": req.Stage, "status": status})
		return
	}

	c.JSON(http.StatusOK, ticketDTOFromZoho(*zt, nil))
}

func (h *TicketHandler) SendMessage(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	var req struct {
		Content string                `json:"content"`
		Channel models.MessageChannel `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var (
		thread *service.ZohoThread
		err    error
	)
	if req.Channel == models.ChannelWhatsApp {
		thread, err = h.zohoSvc.ReplyWhatsAppLiveChat(c.Param("id"), req.Content, c.GetString("email"))
	} else {
		thread, err = h.zohoSvc.ReplyTicket(c.Param("id"), req.Content, string(req.Channel))
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to send message through Zoho Desk: " + err.Error()})
		return
	}

	createdAt := time.Now()
	if thread != nil && !thread.CreatedTime.IsZero() {
		createdAt = thread.CreatedTime
	}

	c.JSON(http.StatusOK, models.TicketMessage{
		ID:         hashStringToUint(thread.ID),
		TicketID:   hashStringToUint(c.Param("id")),
		SenderType: models.SenderTypeAgent,
		Channel:    req.Channel,
		Content:    req.Content,
		CreatedAt:  createdAt,
	})
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")

	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
