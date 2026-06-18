package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/utils"
)

type EmailHandler struct {
	repo     repository.EmailRepository
	brevoSvc *service.BrevoService
}

func NewEmailHandler(repo repository.EmailRepository, brevoSvc *service.BrevoService) *EmailHandler {
	return &EmailHandler{repo: repo, brevoSvc: brevoSvc}
}

func (h *EmailHandler) CreateTemplate(c *gin.Context) {
	var template models.EmailTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(uint); ok {
		template.CreatedBy = uid
	}

	if err := h.repo.CreateTemplate(&template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

func (h *EmailHandler) GetTemplates(c *gin.Context) {
	templates, err := h.repo.GetTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

func (h *EmailHandler) UpdateTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var template models.EmailTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template.ID = uint(id)
	if err := h.repo.UpdateTemplate(&template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, template)
}

func (h *EmailHandler) DeleteTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	if err := h.repo.DeleteTemplate(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar plantilla"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plantilla eliminada"})
}

func (h *EmailHandler) CreateCampaign(c *gin.Context) {
	var campaign models.EmailCampaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(uint); ok {
		campaign.CreatedBy = uid
	}

	if err := h.repo.CreateCampaign(&campaign); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

func (h *EmailHandler) GetCampaigns(c *gin.Context) {
	campaigns, err := h.repo.GetCampaigns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, campaigns)
}

func (h *EmailHandler) UpdateCampaign(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var campaign models.EmailCampaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaign.ID = uint(id)
	if err := h.repo.UpdateCampaign(&campaign); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

func (h *EmailHandler) DeleteCampaign(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	if err := h.repo.DeleteCampaign(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar campaña"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaña eliminada"})
}

type ExpressContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type HybridRecipients struct {
	GroupIDs        []int            `json:"groupIds"`
	UserIDs         []int            `json:"userIds"`
	ExpressContacts []ExpressContact `json:"expressContacts"`
}

// SendCampaign renders the campaign's template blocks to HTML and dispatches
// via Brevo to each recipient. Accepts an optional JSON body with a
// "recipient_list" field to override the campaign's stored recipients.
func (h *EmailHandler) SendCampaign(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	campaign, err := h.repo.GetCampaignByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status == "sent" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign already sent"})
		return
	}

	// Accept optional recipient_list override from request body
	var body struct {
		RecipientList *string `json:"recipient_list"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.RecipientList != nil {
		campaign.RecipientList = *body.RecipientList
	}

	backendURL := resolveBackendURL(c)

	// Render blocks → HTML
	htmlContent, err := utils.RenderBlocksToHTML(campaign.Template.Content, backendURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render email content: " + err.Error()})
		return
	}

	uniqueRecipients := make(map[string]string) // email -> name

	if campaign.RecipientList != "" {
		var legacyIDs []int
		if err := json.Unmarshal([]byte(campaign.RecipientList), &legacyIDs); err == nil {
			// Resolve legacy IDs
			if len(legacyIDs) > 0 {
				type UserEmail struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				}
				var users []UserEmail
				placeholders := make([]string, len(legacyIDs))
				args := make([]interface{}, len(legacyIDs))
				for i, rid := range legacyIDs {
					placeholders[i] = "?"
					args[i] = rid
				}
				query := fmt.Sprintf("SELECT name, email FROM users WHERE id IN (%s) AND deleted_at IS NULL", strings.Join(placeholders, ","))
				h.repo.RawQuery(query, args, &users)
				for _, u := range users {
					if u.Email != "" {
						uniqueRecipients[strings.ToLower(u.Email)] = u.Name
					}
				}
			}
		} else {
			var hybrid HybridRecipients
			if err := json.Unmarshal([]byte(campaign.RecipientList), &hybrid); err == nil {
				// 1. Resolve individual UserIDs
				if len(hybrid.UserIDs) > 0 {
					type UserEmail struct {
						Name  string `json:"name"`
						Email string `json:"email"`
					}
					var users []UserEmail
					placeholders := make([]string, len(hybrid.UserIDs))
					args := make([]interface{}, len(hybrid.UserIDs))
					for i, rid := range hybrid.UserIDs {
						placeholders[i] = "?"
						args[i] = rid
					}
					query := fmt.Sprintf("SELECT name, email FROM users WHERE id IN (%s) AND deleted_at IS NULL", strings.Join(placeholders, ","))
					h.repo.RawQuery(query, args, &users)
					for _, u := range users {
						if u.Email != "" {
							uniqueRecipients[strings.ToLower(u.Email)] = u.Name
						}
					}
				}

				// 2. Resolve GroupIDs members
				if len(hybrid.GroupIDs) > 0 {
					type UserEmail struct {
						Name  string `json:"name"`
						Email string `json:"email"`
					}
					var users []UserEmail
					placeholders := make([]string, len(hybrid.GroupIDs))
					args := make([]interface{}, len(hybrid.GroupIDs))
					for i, gid := range hybrid.GroupIDs {
						placeholders[i] = "?"
						args[i] = gid
					}
					query := fmt.Sprintf("SELECT DISTINCT u.name, u.email FROM users u JOIN audience_group_members agm ON u.id = agm.user_id WHERE agm.audience_group_id IN (%s) AND u.deleted_at IS NULL", strings.Join(placeholders, ","))
					h.repo.RawQuery(query, args, &users)
					for _, u := range users {
						if u.Email != "" {
							uniqueRecipients[strings.ToLower(u.Email)] = u.Name
						}
					}
				}

				// 3. Resolve Express Contacts
				for _, ec := range hybrid.ExpressContacts {
					if ec.Email != "" {
						uniqueRecipients[strings.ToLower(ec.Email)] = ec.Name
					}
				}
			}
		}
	}

	subject := campaign.Subject
	if subject == "" {
		subject = campaign.Title
	}

	var sendErrors []string
	successCount := 0

	for email, name := range uniqueRecipients {
		if err := h.brevoSvc.SendEmail(email, name, subject, htmlContent); err != nil {
			sendErrors = append(sendErrors, fmt.Sprintf("%s: %s", email, err.Error()))
		} else {
			successCount++
		}
	}

	// Mark campaign as sent regardless of partial failures
	now := time.Now()
	campaign.Status = "sent"
	campaign.SentAt = &now
	campaign.Recipients = successCount

	if err := h.repo.UpdateCampaign(campaign); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update campaign status"})
		return
	}

	response := gin.H{
		"message":  "Campaign dispatched",
		"sent":     successCount,
		"total":    len(uniqueRecipients),
		"campaign": campaign,
	}
	if len(sendErrors) > 0 {
		response["errors"] = sendErrors
	}

	c.JSON(http.StatusOK, response)
}

// HandleBrevoWebhook receives and persists events from Brevo (opens, clicks, etc.)
func (h *EmailHandler) HandleBrevoWebhook(c *gin.Context) {
	var payload struct {
		Event      string `json:"event"`
		Email      string `json:"email"`
		CampaignID uint   `json:"campaign_id"`
		MessageID  string `json:"message-id"`
		IP         string `json:"ip"`
		UserAgent  string `json:"user-agent"`
		Date       string `json:"date"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		// Log error but return 200 to Brevo to avoid retries if format is slightly off
		fmt.Printf("Webhook bind error: %v\n", err)
		c.Status(http.StatusOK)
		return
	}

	timestamp := time.Now()
	if payload.Date != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", payload.Date); err == nil {
			timestamp = t
		}
	}

	event := &models.EmailEvent{
		CampaignID: payload.CampaignID,
		Email:      payload.Email,
		Event:      payload.Event,
		IP:         payload.IP,
		UserAgent:  payload.UserAgent,
		Timestamp:  timestamp,
	}

	if err := h.repo.CreateEvent(event); err != nil {
		fmt.Printf("Failed to save email event: %v\n", err)
	}

	// Always return 200 to Brevo
	c.Status(http.StatusOK)
}

func (h *EmailHandler) GetCampaignEvents(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	events, err := h.repo.GetEventsByCampaign(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

// SendQuickEmail sends a one-off transactional email directly via Brevo
// without requiring a persisted template or campaign. Useful for ad-hoc
// communications from the tenant/employee detail views.
func (h *EmailHandler) SendQuickEmail(c *gin.Context) {
	var req struct {
		ToEmail     string `json:"to_email" binding:"required,email"`
		ToName      string `json:"to_name"`
		Subject     string `json:"subject" binding:"required"`
		HTMLContent string `json:"html_content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	html := rewriteImageURLs(req.HTMLContent, resolveBackendURL(c))

	if err := h.brevoSvc.SendEmail(req.ToEmail, req.ToName, req.Subject, html); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al enviar el email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email enviado correctamente", "to": req.ToEmail})
}

// SendQuickEmailBulk sends the same email to multiple recipients at once.
// The body accepts an array of contacts in {to_email, to_name, subject, html_content} form.
func (h *EmailHandler) SendQuickEmailBulk(c *gin.Context) {
	var req struct {
		Recipients  []service.BrevoContact `json:"recipients" binding:"required,min=1"`
		Subject     string                 `json:"subject" binding:"required"`
		HTMLContent string                 `json:"html_content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	html := rewriteImageURLs(req.HTMLContent, resolveBackendURL(c))
	errs := h.brevoSvc.SendBulk(req.Recipients, req.Subject, html)

	sent := len(req.Recipients) - len(errs)
	resp := gin.H{
		"message": fmt.Sprintf("Enviado a %d de %d destinatarios", sent, len(req.Recipients)),
		"sent":    sent,
		"total":   len(req.Recipients),
	}
	if len(errs) > 0 {
		errStrs := make([]string, len(errs))
		for i, e := range errs {
			errStrs[i] = e.Error()
		}
		resp["errors"] = errStrs
	}

	c.JSON(http.StatusOK, resp)
}

// SendTemplate renders a gestor template's blocks to HTML and dispatches
// via Brevo to a resolved recipient list (hybrid: userIds / groupIds / expressContacts).
func (h *EmailHandler) SendTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	template, err := h.repo.GetTemplateByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	var body struct {
		RecipientList string `json:"recipient_list" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recipient_list is required"})
		return
	}

	backendURL := resolveBackendURL(c)

	htmlContent, err := utils.RenderBlocksToHTML(template.Content, backendURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template: " + err.Error()})
		return
	}

	// Resolve recipients — same hybrid logic as SendCampaign
	uniqueRecipients := make(map[string]string)

	var hybrid HybridRecipients
	if err := json.Unmarshal([]byte(body.RecipientList), &hybrid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipient_list format"})
		return
	}

	if len(hybrid.UserIDs) > 0 {
		type UserEmail struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		var users []UserEmail
		placeholders := make([]string, len(hybrid.UserIDs))
		args := make([]interface{}, len(hybrid.UserIDs))
		for i, rid := range hybrid.UserIDs {
			placeholders[i] = "?"
			args[i] = rid
		}
		query := fmt.Sprintf("SELECT name, email FROM users WHERE id IN (%s) AND deleted_at IS NULL", strings.Join(placeholders, ","))
		h.repo.RawQuery(query, args, &users)
		for _, u := range users {
			if u.Email != "" {
				uniqueRecipients[strings.ToLower(u.Email)] = u.Name
			}
		}
	}

	if len(hybrid.GroupIDs) > 0 {
		type UserEmail struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		var users []UserEmail
		placeholders := make([]string, len(hybrid.GroupIDs))
		args := make([]interface{}, len(hybrid.GroupIDs))
		for i, gid := range hybrid.GroupIDs {
			placeholders[i] = "?"
			args[i] = gid
		}
		query := fmt.Sprintf("SELECT DISTINCT u.name, u.email FROM users u JOIN audience_group_members agm ON u.id = agm.user_id WHERE agm.audience_group_id IN (%s) AND u.deleted_at IS NULL", strings.Join(placeholders, ","))
		h.repo.RawQuery(query, args, &users)
		for _, u := range users {
			if u.Email != "" {
				uniqueRecipients[strings.ToLower(u.Email)] = u.Name
			}
		}
	}

	for _, ec := range hybrid.ExpressContacts {
		if ec.Email != "" {
			uniqueRecipients[strings.ToLower(ec.Email)] = ec.Name
		}
	}

	if len(uniqueRecipients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No recipients resolved"})
		return
	}

	subject := template.Subject
	if subject == "" {
		subject = template.Title
	}

	var sendErrors []string
	successCount := 0
	for email, name := range uniqueRecipients {
		if err := h.brevoSvc.SendEmail(email, name, subject, htmlContent); err != nil {
			sendErrors = append(sendErrors, fmt.Sprintf("%s: %s", email, err.Error()))
		} else {
			successCount++
		}
	}

	resp := gin.H{
		"message": "Template dispatched",
		"sent":    successCount,
		"total":   len(uniqueRecipients),
	}
	if len(sendErrors) > 0 {
		resp["errors"] = sendErrors
	}
	c.JSON(http.StatusOK, resp)
}

// resolveBackendURL returns the backend's public URL for building absolute
// links in sent emails. It first checks the SERVICE_URL_BACKEND env var; if
// unset it derives the URL from the incoming request.
func resolveBackendURL(c *gin.Context) string {
	backendURL := os.Getenv("SERVICE_URL_BACKEND")
	if backendURL != "" {
		return backendURL
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if fwd := c.GetHeader("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// rewriteImageURLs replaces relative /api/uploads/ paths in pre-compiled HTML
// with absolute URLs pointing to the public file-serving endpoint.
func rewriteImageURLs(html, backendURL string) string {
	return strings.ReplaceAll(html, "/api/uploads/", strings.TrimRight(backendURL, "/")+"/api/public/uploads/")
}
