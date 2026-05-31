package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// SendCampaign renders the campaign's template blocks to HTML and dispatches
// via Brevo to each recipient stored in RecipientList.
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

	// Render blocks → HTML
	htmlContent, err := utils.RenderBlocksToHTML(campaign.Template.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render email content: " + err.Error()})
		return
	}

	// Parse recipient IDs
	var recipientIDs []int
	if campaign.RecipientList != "" {
		if err := json.Unmarshal([]byte(campaign.RecipientList), &recipientIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse recipient list"})
			return
		}
	}

	// Fetch recipients from user repo (injected via gin context or simple DB query)
	// For now we send to each ID's email using the recipient list.
	// The actual user lookup is handled by the repository layer.
	// We collect recipients from the DB using a raw approach here.
	type UserEmail struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	var users []UserEmail
	if len(recipientIDs) > 0 {
		// Build placeholder string for SQL IN clause
		placeholders := make([]string, len(recipientIDs))
		args := make([]interface{}, len(recipientIDs))
		for i, rid := range recipientIDs {
			placeholders[i] = "?"
			args[i] = rid
		}
		query := fmt.Sprintf("SELECT id, name, email FROM users WHERE id IN (%s) AND deleted_at IS NULL", strings.Join(placeholders, ","))
		h.repo.RawQuery(query, args, &users)

	}

	subject := campaign.Subject
	if subject == "" {
		subject = campaign.Title
	}

	var sendErrors []string
	successCount := 0

	for _, user := range users {
		if err := h.brevoSvc.SendEmail(user.Email, user.Name, subject, htmlContent); err != nil {
			sendErrors = append(sendErrors, fmt.Sprintf("%s: %s", user.Email, err.Error()))
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
		"message":       "Campaign dispatched",
		"sent":          successCount,
		"total":         len(users),
		"campaign":      campaign,
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

