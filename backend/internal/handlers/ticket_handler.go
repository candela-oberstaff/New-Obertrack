package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TicketHandler struct {
	DB       *gorm.DB
	zohoSvc  *service.ZohoService
	brevoSvc *service.BrevoService
}

func NewTicketHandler(db *gorm.DB, zohoSvc *service.ZohoService, brevoSvc *service.BrevoService) *TicketHandler {
	return &TicketHandler{
		DB:       db,
		zohoSvc:  zohoSvc,
		brevoSvc: brevoSvc,
	}
}

// GetTickets maps tickets directly from Zoho Desk
func (h *TicketHandler) GetTickets(c *gin.Context) {
	zohoTickets, err := h.zohoSvc.ListTickets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets from Zoho Desk: " + err.Error()})
		return
	}

	type TicketDTO struct {
		ZohoID        string             `json:"zoho_id"`
		ID            uint               `json:"id"`
		Title         string             `json:"title"`
		Channel       string             `json:"channel"`
		Stage         models.TicketStage `json:"stage"`
		Status        string             `json:"status"`
		CreatedAt     interface{}        `json:"created_at"`
		UpdatedAt     interface{}        `json:"updated_at"`
		AssigneeID    string             `json:"assignee_id,omitempty"`
		AssigneeName  string             `json:"assignee_name,omitempty"`
		AssigneeEmail string             `json:"assignee_email,omitempty"`
		Sentiment     string             `json:"sentiment,omitempty"`
	}

	var tickets []TicketDTO
	for _, zt := range zohoTickets {
		stage := models.StageNew
		switch strings.ToLower(zt.StatusType) {
		case "open":
			stage = models.StageInProgress
		case "closed":
			stage = models.StageClosed
		case "onhold":
			stage = models.StageWaiting
		}

		var assigneeName, assigneeEmail string
		if zt.AssigneeID != "" {
			if agentInfo, err := h.zohoSvc.GetAgent(zt.AssigneeID); err == nil {
				assigneeName = agentInfo.Name
				assigneeEmail = agentInfo.Email
			}
		}

		sentiment := zt.Sentiment
		var customerTone string
		_ = customerTone
		/* 🚀 Desactivado temporalmente para evitar 500 si Ollama falla
		if sentiment == "" || sentiment == "null" {
			sentiment, customerTone = h.analyzeZiaInsights(zt.Subject)
			_ = customerTone
		}
		*/

		t := TicketDTO{
			ZohoID:     zt.ID,
			ID:         uint(hashStringToUint(zt.ID)),
			Title:      zt.Subject,
			Channel:    zt.Channel,
			Stage:      stage,
			Status:     zt.Status,
			CreatedAt:  zt.CreatedTime,
			UpdatedAt:  zt.ModifiedTime,
			AssigneeID: zt.AssigneeID,
			AssigneeName: assigneeName,
			AssigneeEmail: assigneeEmail,
			Sentiment:    sentiment,
		}
		tickets = append(tickets, t)
	}

	c.JSON(http.StatusOK, tickets)
}

// GetTicket detail loads a detailed ticket along with its threads (chat conversations) from Zoho Desk
func (h *TicketHandler) GetTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")

	zohoTicketID := getZohoIDFromParam(ticketIDStr, h.DB)
	if zohoTicketID == "" {
		zohoTicketID = ticketIDStr
	}

	zt, err := h.zohoSvc.GetTicketDetail(zohoTicketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found in Zoho Desk: " + err.Error()})
		return
	}

	threads, err := h.zohoSvc.GetTicketThreads(zohoTicketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chat logs: " + err.Error()})
		return
	}

	h.saveIDMapping(zt.ID, hashStringToUint(zt.ID))
	if zt.ContactID != "" {
		h.saveIDMapping(zt.ContactID, hashStringToUint(zt.ContactID))
	}

	var linkedUser *models.User
	cleanPhone := zt.Phone
	if cleanPhone != "" {
		var u models.User
		if err := h.DB.Where(
			"REPLACE(REPLACE(phone_number, '+', ''), ' ', '') = ?", cleanPhone,
		).First(&u).Error; err == nil {
			linkedUser = &u
		}
	}

	stage := models.StageNew
	switch strings.ToLower(zt.StatusType) {
	case "open":
		stage = models.StageInProgress
	case "closed":
		stage = models.StageClosed
	case "onhold":
		stage = models.StageWaiting
	}

	var assigneeName, assigneeEmail string
	if zt.AssigneeID != "" {
		if agentInfo, err := h.zohoSvc.GetAgent(zt.AssigneeID); err == nil {
			assigneeName = agentInfo.Name
			assigneeEmail = agentInfo.Email
		}
	}

	var messages []models.TicketMessage
	for _, thread := range threads {
		channel := models.ChannelWhatsApp
		if strings.EqualFold(thread.Channel, "email") {
			channel = models.ChannelEmail
		}

		subMsgs, err := h.zohoSvc.GetThreadMessages(thread.ID)
		if err == nil && len(subMsgs) > 0 {
			for _, sub := range subMsgs {
				if sub.Summary == "" && strings.ToLower(sub.Type) != "text" {
					continue
				}

				subSenderType := models.SenderTypeContact
				if sub.Author != nil {
					if strings.ToLower(sub.Author.Type) == "agent" {
						subSenderType = models.SenderTypeAgent
					}
				} else {
					if strings.ToLower(sub.Direction) == "out" {
						subSenderType = models.SenderTypeAgent
					}
				}

				cleanContent := stripHTML(sub.Summary)

				messages = append(messages, models.TicketMessage{
					ID:         uint(hashStringToUint(sub.ID)),
					TicketID:   uint(hashStringToUint(zt.ID)),
					SenderType: subSenderType,
					Channel:    channel,
					Content:    cleanContent,
					CreatedAt:  sub.CreatedTime,
				})
			}
		} else {
			senderType := models.SenderTypeContact
			if strings.ToLower(thread.AuthorType) == "agent" {
				senderType = models.SenderTypeAgent
			} else if strings.ToLower(thread.AuthorType) == "system" {
				senderType = models.SenderTypeSystem
			}

			msgContent := thread.Summary
			if thread.Content != "" {
				msgContent = thread.Content
			}

			messages = append(messages, models.TicketMessage{
				ID:         uint(hashStringToUint(thread.ID)),
				TicketID:   uint(hashStringToUint(zt.ID)),
				SenderType: senderType,
				Channel:    channel,
				Content:    stripHTML(msgContent),
				CreatedAt:  thread.CreatedTime,
			})
		}
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	type TicketDetailDTO struct {
		ID            uint                   `json:"id"`
		ZohoID        string                 `json:"zoho_id"`
		TicketNumber  string                 `json:"ticket_number"`
		Title         string                 `json:"title"`
		Channel       string                 `json:"channel,omitempty"`
		Stage         models.TicketStage     `json:"stage"`
		Status        string                 `json:"status"`
		Priority      string                 `json:"priority,omitempty"`
		Category      string                 `json:"category,omitempty"`
		Description   string                 `json:"description,omitempty"`
		Contact       models.Contact         `json:"contact,omitempty"`
		Messages      []models.TicketMessage `json:"messages,omitempty"`
		CreatedAt     interface{}            `json:"created_at"`
		UpdatedAt     interface{}            `json:"updated_at"`
		Sentiment     string                 `json:"sentiment,omitempty"`
		CustomerTone  string                 `json:"customer_tone,omitempty"`
		IsEscalated   bool                   `json:"is_escalated,omitempty"`
		WebURL        string                 `json:"web_url,omitempty"`
		AssigneeID    string                 `json:"assignee_id,omitempty"`
		AssigneeName  string                 `json:"assignee_name,omitempty"`
		AssigneeEmail string                 `json:"assignee_email,omitempty"`
	}

	sentiment := zt.Sentiment
	customerTone := zt.CustomerTone

	/* 🚀 Desactivado temporalmente para evitar 500 si Ollama falla
	if sentiment == "" || sentiment == "null" || customerTone == "" || customerTone == "null" {
		contextText := zt.Subject
		if zt.Description != "" {
			contextText = contextText + " - " + zt.Description
		}
		sentiment, customerTone = h.analyzeZiaInsights(contextText)
	}
	*/

	dto := TicketDetailDTO{
		ID:           uint(hashStringToUint(zt.ID)),
		ZohoID:       zt.ID,
		TicketNumber: zt.TicketNumber,
		Title:        zt.Subject,
		Channel:      zt.Channel,
		Stage:        stage,
		Status:       zt.Status,
		Priority:     zt.Priority,
		Category:     zt.Category,
		Description:  zt.Description,
		Contact: models.Contact{
			ID:    uint(hashStringToUint(zt.ContactID)),
			Name:  zt.ContactName,
			Phone: zt.Phone,
			Email: zt.Email,
		},
		Messages:      messages,
		CreatedAt:     zt.CreatedTime,
		UpdatedAt:     zt.ModifiedTime,
		Sentiment:     sentiment,
		CustomerTone:  customerTone,
		IsEscalated:   zt.IsEscalated,
		WebURL:        zt.WebURL,
		AssigneeID:    zt.AssigneeID,
		AssigneeName:  assigneeName,
		AssigneeEmail: assigneeEmail,
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket":      dto,
		"linked_user": linkedUser,
		"zoho_id":     zt.ID,
	})
}

// UpdateTicket pushes status changes back to Zoho Desk
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	zohoTicketID := getZohoIDFromParam(ticketIDStr, h.DB)
	if zohoTicketID == "" {
		zohoTicketID = ticketIDStr
	}

	var updateData struct {
		Stage      models.TicketStage `json:"stage"`
		Status     string             `json:"status"`
		AssignedTo *uint              `json:"assigned_to"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zohoStatus := updateData.Status
	zohoStage := ""
	switch updateData.Stage {
	case models.StageNew:
		zohoStage = "New"
		if zohoStatus == "" {
			zohoStatus = "Open"
		}
	case models.StageInProgress:
		zohoStage = "In Progress"
		if zohoStatus == "" {
			zohoStatus = "Open"
		}
	case models.StageWaiting:
		zohoStage = "On Hold"
	case models.StageClosed:
		zohoStage = "Completed"
		zohoStatus = "Closed"
	}

	err := h.zohoSvc.UpdateTicketStatus(zohoTicketID, zohoStage, zohoStatus, "")
	if err != nil {
		h.DB.Logger.Warn(c, "Failed to update ticket in Zoho: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket in Zoho: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

type SendMessageRequest struct {
	Content string                `json:"content"`
	Channel models.MessageChannel `json:"channel"`
}

// SendMessage sends response thread directly via Zoho Desk official APIs
func (h *TicketHandler) SendMessage(c *gin.Context) {
	ticketIDStr := c.Param("id")
	zohoTicketID := getZohoIDFromParam(ticketIDStr, h.DB)
	if zohoTicketID == "" {
		zohoTicketID = ticketIDStr
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// 🚀 SI EL CANAL ES WHATSAPP:
	if req.Channel == models.ChannelWhatsApp {
		agentEmail := c.GetString("email")
		thread, err := h.zohoSvc.ReplyWhatsAppLiveChat(zohoTicketID, req.Content, agentEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send WhatsApp via Zoho: " + err.Error()})
			return
		}

		msg := models.TicketMessage{
			ID:         uint(hashStringToUint(thread.ID)),
			TicketID:   uint(hashStringToUint(zohoTicketID)),
			SenderType: models.SenderTypeAgent,
			Channel:    models.ChannelWhatsApp,
			Content:    req.Content,
			CreatedAt:  thread.CreatedTime,
		}
		c.JSON(http.StatusOK, msg)
		return
	}

	// ✉️ FALLBACK TRADICIONAL PARA EMAIL
	zohoChannel := "email"
	thread, err := h.zohoSvc.ReplyTicket(zohoTicketID, req.Content, zohoChannel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dispatch email to Zoho Desk: " + err.Error()})
		return
	}

	msg := models.TicketMessage{
		ID:         uint(hashStringToUint(thread.ID)),
		TicketID:   uint(hashStringToUint(zohoTicketID)),
		SenderType: models.SenderTypeAgent,
		Channel:    req.Channel,
		Content:    req.Content,
		CreatedAt:  thread.CreatedTime,
	}
	c.JSON(http.StatusOK, msg)
}

// SendReply sends a reply specifically for chat section, targeting WhatsApp channel
func (h *TicketHandler) SendReply(c *gin.Context) {
	ticketIDStr := c.Param("id")
	zohoTicketID := getZohoIDFromParam(ticketIDStr, h.DB)
	if zohoTicketID == "" {
		zohoTicketID = ticketIDStr
	}

	var req struct {
		Content string                `json:"content" binding:"required"`
		Channel models.MessageChannel `json:"channel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content and channel are required"})
		return
	}

	// Verify that the channel is WhatsApp as required for chat section replies
	if req.Channel != models.ChannelWhatsApp {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel must be 'whatsapp' for chat section replies"})
		return
	}

	// Send via WhatsApp channel using Zoho Desk
	agentEmail := c.GetString("email")
	thread, err := h.zohoSvc.ReplyWhatsAppLiveChat(zohoTicketID, req.Content, agentEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send WhatsApp reply: " + err.Error()})
		return
	}

	msg := models.TicketMessage{
		ID:         uint(hashStringToUint(thread.ID)),
		TicketID:   uint(hashStringToUint(zohoTicketID)),
		SenderType: models.SenderTypeAgent,
		Channel:    req.Channel,
		Content:    req.Content,
		CreatedAt:  thread.CreatedTime,
	}
	c.JSON(http.StatusOK, msg)
}

// 🚀 ==========================================
// 🚀 ZOHO DESK INCOMING WEBHOOK HANDLER
// 🚀 ==========================================

type ZohoWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		TicketID    string    `json:"ticketId"`
		ThreadID    string    `json:"id"`
		Channel     string    `json:"channel"`
		Summary     string    `json:"summary"`
		CreatedTime time.Time `json:"createdTime"`
	} `json:"data"`
}

func (h *TicketHandler) HandleZohoWebhook(c *gin.Context) {
	var payload ZohoWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook structural payload"})
		return
	}

	if payload.Event == "Incoming Threads" || strings.Contains(payload.Event, "Thread") {
		ticketID := payload.Data.TicketID
		content := stripHTML(payload.Data.Summary)
		channelName := strings.ToLower(payload.Data.Channel)

		targetChannel := models.ChannelEmail
		if channelName == "whatsapp" {
			targetChannel = models.ChannelWhatsApp
		}

		newMessage := models.TicketMessage{
			ID:         uint(hashStringToUint(payload.Data.ThreadID)),
			TicketID:   uint(hashStringToUint(ticketID)),
			SenderType: models.SenderTypeContact,
			Channel:    targetChannel,
			Content:    content,
			CreatedAt:  payload.Data.CreatedTime,
		}

		// 🚀 _ = newMessage le dice al compilador explícitamente que la variable está en uso latente
		_ = newMessage 

		fmt.Printf("[Zoho Webhook OpenTrack] Mensaje entrante procesado para Ticket %s vía %s\n", ticketID, channelName)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// GetContacts stub compatibility layer
func (h *TicketHandler) GetContacts(c *gin.Context) {
	var contacts []models.Contact
	if err := h.DB.Find(&contacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contacts"})
		return
	}
	c.JSON(http.StatusOK, contacts)
}

// UpdateContact stub compatibility layer
func (h *TicketHandler) UpdateContact(c *gin.Context) {
	id := c.Param("contactId")
	var contact models.Contact
	if err := h.DB.First(&contact, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}

	var req struct {
		Name *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != nil && *req.Name != "" {
		contact.Name = *req.Name
	}
	h.DB.Save(&contact)
	c.JSON(http.StatusOK, contact)
}

func hashStringToUint(s string) uint32 {
	var hash uint32 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint32(s[i])
	}
	return hash
}

func (h *TicketHandler) saveIDMapping(zohoID string, numericID uint32) {
	type ZohoMapping struct {
		ZohoID    string `gorm:"primaryKey"`
		NumericID uint32 `gorm:"uniqueIndex"`
	}

	h.DB.AutoMigrate(&ZohoMapping{})
	h.DB.Save(&ZohoMapping{ZohoID: zohoID, NumericID: numericID})
}

func getZohoIDFromParam(param string, db *gorm.DB) string {
	type ZohoMapping struct {
		ZohoID    string
		NumericID uint32
	}

	num, err := strconv.ParseUint(param, 10, 32)
	if err != nil {
		return ""
	}

	var m ZohoMapping
	if err := db.AutoMigrate(&ZohoMapping{}); err == nil {
		if err := db.Where("numeric_id = ?", uint32(num)).First(&m).Error; err == nil {
			return m.ZohoID
		}
	}
	return ""
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	
	var builder strings.Builder
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
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// 🚀 ==========================================
// 🚀 OLLAMA CLUIT SENTIMENT & TONE INTEGRATION
// 🚀 ==========================================

type OllamaReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResp struct {
	Response string `json:"response"`
}

type ZiaAnalysisResult struct {
	Sentiment string `json:"sentiment"`
	Tone      string `json:"tone"`
}

func (h *TicketHandler) analyzeZiaInsights(text string) (string, string) {
	defaultSentiment := "NEUTRAL"
	defaultTone := "Informativo"

	if text == "" {
		return defaultSentiment, defaultTone
	}

	apiURL := os.Getenv("OLLAMA_API_URL")
	apiKey := os.Getenv("OLLAMA_API_KEY")
	model := os.Getenv("OLLAMA_MODEL")

	if apiURL == "" {
		apiURL = "https://api.ollama.com"
	}
	if model == "" {
		model = "llama3"
	}

	prompt := fmt.Sprintf(
		"Analiza el siguiente texto de un ticket de soporte. "+
		"Debes responder EXCLUSIVAMENTE con un objeto JSON válido que contenga dos campos:\n"+
		"1. \"sentiment\": Debe ser estrictamente uno de estos valores: POSITIVO, NEUTRAL, NEGATIVO o URGENTE.\n"+
		"2. \"tone\": Describe brevemente el tono emocional del cliente en una o dos palabras en español (ejemplos: Frustrado, Amable, Profesional, Preocupado, Enojado, Informativo).\n\n"+
		"No agregues texto antes ni después del JSON. Si no puedes determinarlo, usa valores neutrales.\n"+
		"Texto a analizar: \"%s\"",
		text,
	)

	bodyBytes, _ := json.Marshal(OllamaReq{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})

	req, err := http.NewRequest("POST", apiURL+"/api/generate", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return defaultSentiment, defaultTone
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return defaultSentiment, defaultTone
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return defaultSentiment, defaultTone
	}

	var res OllamaResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return defaultSentiment, defaultTone
	}

	cleanJSON := strings.TrimSpace(res.Response)
	
	var analysis ZiaAnalysisResult
	if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
		fmt.Println("[Ollama Parsing Error]: Falló al decodificar el JSON de respuesta:", cleanJSON)
		return defaultSentiment, defaultTone
	}

	return strings.ToUpper(analysis.Sentiment), strings.Title(strings.ToLower(analysis.Tone))
}