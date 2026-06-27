package handlers

import (
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TicketHandler struct {
	db         *gorm.DB
	zohoSvc    *service.ZohoService
	ticketSvc  service.TicketService
	channelSvc service.ChannelService
}

func NewTicketHandler(db *gorm.DB, zohoSvc *service.ZohoService, ticketSvc service.TicketService, channelSvc service.ChannelService) *TicketHandler {
	return &TicketHandler{db: db, zohoSvc: zohoSvc, ticketSvc: ticketSvc, channelSvc: channelSvc}
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
	contactEmail := firstNonEmptyLocal(zt.Email, zt.ContactEmail)

	if zt.ContactInfo != nil {
		fullName := strings.TrimSpace(zt.ContactInfo.FirstName + " " + zt.ContactInfo.LastName)
		if fullName != "" {
			contactName = fullName
		}
		phoneVal := firstNonEmptyLocal(zt.ContactInfo.Phone, zt.ContactInfo.Mobile)
		if phoneVal != "" {
			contactPhone = phoneVal
		}
		if zt.ContactInfo.Email != "" {
			contactEmail = zt.ContactInfo.Email
		}
	}

	if looksLikePhoneLocal(contactName) {
		if contactPhone == "" {
			contactPhone = contactName
		}
		contactName = ""
	}

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
		"department_id":  zt.DepartmentID,
		"created_at":     zt.CreatedTime,
		"updated_at":     zt.ModifiedTime,
		"origin":         models.OriginZoho,
		"contact": gin.H{
			"id":    hashStringToUint(zt.ContactID),
			"name":  contactName,
			"phone": contactPhone,
			"email": contactEmail,
		},
		"messages": messages,
	}
}

// ticketDTOFromInternal maps a locally stored internal alert ticket to the same
// shape the board consumes for Zoho tickets, tagged with origin "internal".
func ticketDTOFromInternal(t models.Ticket) gin.H {
	messages := t.Messages
	if messages == nil {
		messages = []models.TicketMessage{}
	}
	assigneeName := ""
	assigneeEmail := ""
	if t.Assignee != nil {
		assigneeName = t.Assignee.Name
		assigneeEmail = t.Assignee.Email
	}
	return gin.H{
		"id":                 t.ID,
		"zoho_id":            "",
		"assigned_to":        t.AssignedTo,
		"assignee_name":      assigneeName,
		"assignee_email":     assigneeEmail,
		"title":              t.Title,
		"description":        t.Description,
		"stage":              t.Stage,
		"status":             t.Status,
		"origin":             models.OriginInternal,
		"user_id":            t.UserID,
		"professional_email": t.ProfessionalEmail,
		"professional_phone": t.ProfessionalPhone,
		"company_name":       t.CompanyName,
		"rejected_by_name":   t.RejectedByName,
		"reason":             t.Reason,
		"work_dates":         t.WorkDates,
		"created_at":         t.CreatedAt,
		"updated_at":         t.UpdatedAt,
		"contact":            nil,
		"messages":           messages,
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

// resolveZohoAgentID reads the current user's ZohoAgentID from the DB.
// It lazily fetches it from Zoho when missing, using the user's email.
func (h *TicketHandler) resolveZohoAgentID(c *gin.Context) (string, error) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return "", err
	}

	if user.ZohoAgentID != "" {
		return user.ZohoAgentID, nil
	}

	// Lazy-sync: match by email against Zoho Desk agents
	zohoID, _, err := h.zohoSvc.GetAgentByEmail(user.Email)
	if err != nil {
		log.Printf("[Tickets] Could not sync Zoho agent ID for user %d (%s): %v", userID, user.Email, err)
		return "", err
	}

	// Persist for subsequent requests
	h.db.Model(&user).Update("zoho_agent_id", zohoID)
	return zohoID, nil
}

// isTicketOwner checks that the ticket is assigned to the current user (or allows superadmins/customer success).
func (h *TicketHandler) isTicketOwner(c *gin.Context, zt *service.ZohoTicket) bool {
	if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess) {
		return true
	}
	agentID, err := h.resolveZohoAgentID(c)
	if err != nil {
		return false
	}
	systemAgentID := os.Getenv("ZOHO_SYSTEM_AGENT_ID")
	if systemAgentID != "" && zt.AssigneeID == systemAgentID {
		return true
	}
	return zt.AssigneeID == agentID
}

func (h *TicketHandler) GetTickets(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	// Superadmins see all tickets; customer_success only see their own
	var assigneeID string
	if !middleware.IsSuperadmin(c) {
		var err error
		assigneeID, err = h.resolveZohoAgentID(c)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "No se pudo obtener tu ID de agente en Zoho. Asegurate de que tu correo esté registrado allí.",
			})
			return
		}
	}

	tickets := make([]gin.H, 0)

	// Internal Obertrack alerts (e.g. work-hour rejections) come first and are
	// visible to all support users. A Zoho outage must not hide them.
	if h.ticketSvc != nil {
		internal, ierr := h.ticketSvc.ListInternal()
		if ierr != nil {
			log.Printf("[Tickets] failed to list internal alerts: %v", ierr)
		} else {
			for _, t := range internal {
				tickets = append(tickets, ticketDTOFromInternal(t))
			}
		}
	}

	// Solicitudes de soporte por chat (origen "support"): se muestran junto a las
	// demás en el tablero. Un fallo aquí no debe ocultar las otras.
	if h.channelSvc != nil {
		support, serr := h.channelSvc.ListSupportTicketsForBoard()
		if serr != nil {
			log.Printf("[Tickets] failed to list support tickets: %v", serr)
		} else {
			for _, st := range support {
				tickets = append(tickets, ticketDTOFromSupport(st))
			}
		}
	}

	zohoTickets, err := h.zohoSvc.ListTickets(assigneeID)
	if err != nil {
		// Degrade gracefully: still return internal alerts if Zoho is unavailable.
		log.Printf("[Tickets] failed to fetch from Zoho Desk: %v", err)
	} else {
		for _, zt := range zohoTickets {
			tickets = append(tickets, ticketDTOFromZoho(zt, nil))
		}
	}

	c.JSON(http.StatusOK, tickets)
}

// ticketDTOFromSupport mapea un ticket de soporte por chat al DTO del tablero.
// Estado → columna: open→Nuevo, assigned→En Progreso, resolved→Cerrado.
func ticketDTOFromSupport(t models.SupportTicket) gin.H {
	stage := models.StageNew
	status := "open"
	switch t.Status {
	case models.SupportStatusAssigned:
		stage = models.StageInProgress
	case models.SupportStatusResolved:
		stage = models.StageClosed
		status = "closed"
	}

	requesterName, requesterEmail, companyName := "", "", ""
	if t.Requester != nil {
		requesterName = t.Requester.Name
		requesterEmail = t.Requester.Email
		companyName = t.Requester.CompanyName
	}
	assigneeName, assigneeEmail := "", ""
	if t.Assignee != nil {
		assigneeName = t.Assignee.Name
		assigneeEmail = t.Assignee.Email
	}
	title := "Soporte"
	if requesterName != "" {
		title = "Soporte · " + requesterName
	}

	return gin.H{
		"id":                 t.ID,
		"zoho_id":            "",
		"channel_id":         t.ChannelID,
		"assigned_to":        t.AssignedTo,
		"assignee_name":      assigneeName,
		"assignee_email":     assigneeEmail,
		"title":              title,
		"description":        "Solicitud de soporte por chat",
		"stage":              stage,
		"status":             status,
		"origin":             "support",
		"user_id":            t.RequesterID,
		"professional_email": requesterEmail,
		"professional_phone": "",
		"company_name":       companyName,
		"created_at":         t.CreatedAt,
		"updated_at":         t.UpdatedAt,
		"contact":            nil,
		"messages":           []models.TicketMessage{},
	}
}

// UpdateInternalTicket changes the stage/status of an internal alert ticket
// (e.g. marking a work-hour rejection alert as resolved). It never touches Zoho.
func (h *TicketHandler) UpdateInternalTicket(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req struct {
		Stage  string `json:"stage"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket, err := h.ticketSvc.UpdateInternal(uint(id), models.TicketStage(req.Stage), req.Status)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal ticket not found"})
		return
	}
	c.JSON(http.StatusOK, ticketDTOFromInternal(*ticket))
}

// GetInternalTicket returns a single internal alert ticket (for its detail page).
func (h *TicketHandler) GetInternalTicket(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}
	ticket, err := h.ticketSvc.GetInternal(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal ticket not found"})
		return
	}
	c.JSON(http.StatusOK, ticketDTOFromInternal(*ticket))
}

// AddInternalNote appends a follow-up note to an internal alert ticket.
func (h *TicketHandler) AddInternalNote(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := h.ticketSvc.AddInternalNote(uint(id), middleware.GetUserID(c), req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudo agregar la nota"})
		return
	}
	c.JSON(http.StatusCreated, msg)
}

// GetRejectionReport returns internal work-hour-rejection alerts for a month,
// for the rejections report table/export.
func (h *TicketHandler) GetRejectionReport(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	now := time.Now()
	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))
	if month < 1 || month > 12 {
		month = int(now.Month())
	}
	if year < 2000 {
		year = now.Year()
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)

	tickets, err := h.ticketSvc.ListInternalReport(start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build rejection report"})
		return
	}

	items := make([]gin.H, 0, len(tickets))
	for _, t := range tickets {
		items = append(items, ticketDTOFromInternal(t))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items), "month": month, "year": year})
}

// GetSupportAgents lists active customer_success users (transfer targets).
func (h *TicketHandler) GetSupportAgents(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	agents, err := h.ticketSvc.ListSupportAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents"})
		return
	}
	out := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		out = append(out, gin.H{"id": a.ID, "name": a.Name, "email": a.Email, "zoho_agent_id": a.ZohoAgentID})
	}
	c.JSON(http.StatusOK, out)
}

// GetTicketTransfers returns the transfer history for a ticket.
func (h *TicketHandler) GetTicketTransfers(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	transfers, err := h.ticketSvc.ListTransfers(c.Query("origin"), c.Query("ref"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list transfers"})
		return
	}
	c.JSON(http.StatusOK, transfers)
}

// TransferInternalTicket reassigns an internal alert ticket to another agent.
func (h *TicketHandler) TransferInternalTicket(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}
	var req struct {
		ToUserID uint   `json:"to_user_id" binding:"required"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ticket, err := h.ticketSvc.TransferInternal(uint(id), req.ToUserID, middleware.GetUserID(c), middleware.IsSuperadmin(c), req.Reason)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": "No se pudo traspasar el ticket"})
		return
	}
	c.JSON(http.StatusOK, ticketDTOFromInternal(*ticket))
}

// TransferZohoTicket reassigns a Zoho Desk ticket to another support agent.
func (h *TicketHandler) TransferZohoTicket(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	ticketID := c.Param("id")
	var req struct {
		ToAgentID string `json:"to_agent_id" binding:"required"` // Zoho agent id
		Reason    string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zt, err := h.zohoSvc.GetTicketDetail(ticketID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket from Zoho Desk: " + err.Error()})
		return
	}
	if !h.isTicketOwner(c, zt) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo el responsable actual o un superadmin pueden traspasar este ticket"})
		return
	}

	// Resolve the target Zoho agent (name/email).
	agent, err := h.zohoSvc.GetAgent(req.ToAgentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agente de Zoho no encontrado"})
		return
	}

	if err := h.zohoSvc.AssignTicket(ticketID, req.ToAgentID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "No se pudo reasignar en Zoho Desk: " + err.Error()})
		return
	}

	// Cross-reference Zoho agents with local users (by email) for notifications.
	var toUserID *uint
	if agent.Email != "" {
		var u models.User
		if err := h.db.Where("email = ?", agent.Email).First(&u).Error; err == nil {
			toUserID = &u.ID
		}
	}
	var fromUserID *uint
	if zt.AssigneeEmail != "" {
		var prev models.User
		if err := h.db.Where("email = ?", zt.AssigneeEmail).First(&prev).Error; err == nil {
			fromUserID = &prev.ID
		}
	}
	var byName string
	if by, err := h.ticketSvc.GetUserName(middleware.GetUserID(c)); err == nil {
		byName = by
	}
	_ = h.ticketSvc.RecordTransfer(service.TransferInput{
		Origin:      models.OriginZoho,
		TicketRef:   ticketID,
		TicketTitle: zt.Subject,
		FromUserID:  fromUserID,
		FromName:    zt.AssigneeName,
		ToUserID:    toUserID,
		ToName:      agent.Name,
		ByUserID:    middleware.GetUserID(c),
		ByName:      byName,
		Reason:      req.Reason,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Ticket traspasado", "assignee_id": req.ToAgentID})
}

// GetZohoAgents lists Zoho Desk agents (transfer targets for Zoho tickets),
// cross-referenced with local users by email when a match exists.
func (h *TicketHandler) GetZohoAgents(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}
	agents, err := h.zohoSvc.ListAgents()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "No se pudieron obtener los agentes de Zoho: " + err.Error()})
		return
	}
	out := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		var uid *uint
		if a.Email != "" {
			var u models.User
			if err := h.db.Where("email = ?", a.Email).First(&u).Error; err == nil {
				uid = &u.ID
			}
		}
		out = append(out, gin.H{"zoho_agent_id": a.ID, "name": a.Name, "email": a.Email, "user_id": uid})
	}
	c.JSON(http.StatusOK, out)
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

	// Verify the ticket is assigned to the current agent
	if !h.isTicketOwner(c, zt) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: ticket not assigned to you"})
		return
	}

	threads, err := h.zohoSvc.GetTicketThreads(ticketID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket messages from Zoho Desk: " + err.Error()})
		return
	}

	messages := make([]models.TicketMessage, 0, len(threads))
	for _, thread := range threads {
		// Try to get individual sub‑messages for Instant Messaging (e.g. WhatsApp)
		subMsgs, err := h.zohoSvc.GetThreadMessages(thread.ID)
		if err == nil && len(subMsgs) > 0 {
			for _, sub := range subMsgs {
				if sub.Summary == "" {
					continue
				}

				senderType := models.SenderTypeContact
				if sub.Author != nil && strings.EqualFold(sub.Author.Type, "AGENT") {
					senderType = models.SenderTypeAgent
				} else if strings.EqualFold(sub.Direction, "OUT") {
					senderType = models.SenderTypeAgent
				}

				channel := models.ChannelWhatsApp
				if strings.EqualFold(thread.Channel, "email") {
					channel = models.ChannelEmail
				}

				content := stripHTML(sub.Summary)
				if content == "This message has been sent from other source and cannot be viewed" || content == "" {
					var dbMsg models.TicketMessage
					if h.db.Where("external_id = ?", sub.ID).First(&dbMsg).Error == nil {
						content = dbMsg.Content
					}
				}

				messages = append(messages, models.TicketMessage{
					ID:         hashStringToUint(sub.ID),
					TicketID:   hashStringToUint(ticketID),
					SenderType: senderType,
					Channel:    channel,
					Content:    content,
					CreatedAt:  sub.CreatedTime,
				})
			}
		} else {
			content := thread.Content
			if content == "" {
				content = thread.Summary
			}
			if content == "This message has been sent from other source and cannot be viewed" || content == "" {
				var dbMsg models.TicketMessage
				if h.db.Where("external_id = ?", thread.ID).First(&dbMsg).Error == nil {
					content = dbMsg.Content
				}
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

	// Verify ownership before updating
	zt, err := h.zohoSvc.GetTicketDetail(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket from Zoho Desk: " + err.Error()})
		return
	}
	if !h.isTicketOwner(c, zt) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: ticket not assigned to you"})
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

	updated, err := h.zohoSvc.GetTicketDetail(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"zoho_id": c.Param("id"), "stage": req.Stage, "status": status})
		return
	}

	c.JSON(http.StatusOK, ticketDTOFromZoho(*updated, nil))
}

func (h *TicketHandler) SendMessage(c *gin.Context) {
	if !canUseSupportInbox(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access restricted to support users"})
		return
	}

	ticketID := c.Param("id")
	// Verify ownership before sending
	zt, err := h.zohoSvc.GetTicketDetail(ticketID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch ticket from Zoho Desk: " + err.Error()})
		return
	}
	if !h.isTicketOwner(c, zt) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: ticket not assigned to you"})
		return
	}

	var req struct {
		Content    string                `json:"content"`
		Channel    models.MessageChannel `json:"channel"`
		TemplateID string                `json:"template_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (strings.TrimSpace(req.Content) == "" && req.TemplateID == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var (
		messageID string
		sendErr   error
	)

	if req.Channel == models.ChannelWhatsApp {
		thread, err := h.zohoSvc.ReplyWhatsAppLiveChat(ticketID, req.Content, c.GetString("email"), req.TemplateID)
		if err != nil {
			sendErr = err
		} else if thread != nil {
			messageID = thread.ID
		}
	} else {
		thread, err := h.zohoSvc.ReplyTicket(ticketID, req.Content, string(req.Channel))
		if err != nil {
			sendErr = err
		} else if thread != nil {
			messageID = thread.ID
		}
	}

	if sendErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to send message through Zoho Desk: " + sendErr.Error()})
		return
	}

	// Save the outgoing message to the local database to bypass Zoho's security masking
	if messageID != "" {
		dbMsg := models.TicketMessage{
			ID:         hashStringToUint(messageID),
			TicketID:   hashStringToUint(ticketID),
			SenderType: models.SenderTypeAgent,
			Channel:    req.Channel,
			Content:    req.Content,
			ExternalID: messageID,
			CreatedAt:  time.Now(),
		}
		if err := h.db.Create(&dbMsg).Error; err != nil {
			log.Printf("[Tickets] Failed to save outgoing message to local cache: %v", err)
		}
	}

	c.JSON(http.StatusOK, models.TicketMessage{
		ID:         hashStringToUint(messageID),
		TicketID:   hashStringToUint(ticketID),
		SenderType: models.SenderTypeAgent,
		Channel:    req.Channel,
		Content:    req.Content,
		CreatedAt:  time.Now(),
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
