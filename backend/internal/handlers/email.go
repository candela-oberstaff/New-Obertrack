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
	"github.com/obertrack/backend/internal/middleware"
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

// recipientColumns son los campos del usuario que alimentan las variables de
// personalización ({{nombre}}, {{empresa}}, ...). Ver utils.EmailRecipient.
const recipientColumns = "u.name, u.email, u.job_title, u.company_name, u.industry, u.phone_number, u.country, u.state, u.city"

// resolveRecipients expande un recipient_list al conjunto final de
// destinatarios, deduplicado por email en minúsculas. Acepta el formato
// híbrido ({groupIds, userIds, expressContacts}) y el legacy (array plano de
// IDs de usuario).
func (h *EmailHandler) resolveRecipients(recipientList string) map[string]utils.EmailRecipient {
	out := make(map[string]utils.EmailRecipient)
	if strings.TrimSpace(recipientList) == "" {
		return out
	}

	add := func(rs []utils.EmailRecipient) {
		for _, r := range rs {
			if r.Email == "" {
				continue
			}
			out[strings.ToLower(r.Email)] = r
		}
	}

	var legacyIDs []int
	if err := json.Unmarshal([]byte(recipientList), &legacyIDs); err == nil {
		add(h.usersByIDs(legacyIDs))
		return out
	}

	var hybrid HybridRecipients
	if err := json.Unmarshal([]byte(recipientList), &hybrid); err != nil {
		return out
	}

	add(h.usersByIDs(hybrid.UserIDs))
	add(h.usersByGroups(hybrid.GroupIDs))
	for _, ec := range hybrid.ExpressContacts {
		key := strings.ToLower(ec.Email)
		if ec.Email == "" {
			continue
		}
		// Un contacto express solo tiene nombre y correo, así que no debe pisar
		// al mismo correo ya resuelto como usuario: ese trae todos los campos
		// que alimentan las variables.
		if _, alreadyResolved := out[key]; alreadyResolved {
			continue
		}
		out[key] = utils.EmailRecipient{Name: ec.Name, Email: ec.Email}
	}
	return out
}

func (h *EmailHandler) usersByIDs(ids []int) []utils.EmailRecipient {
	if len(ids) == 0 {
		return nil
	}
	placeholders, args := sqlPlaceholders(ids)
	query := fmt.Sprintf(
		"SELECT %s FROM users u WHERE u.id IN (%s) AND u.deleted_at IS NULL",
		recipientColumns, placeholders,
	)
	var users []utils.EmailRecipient
	h.repo.RawQuery(query, args, &users)
	return users
}

func (h *EmailHandler) usersByGroups(groupIDs []int) []utils.EmailRecipient {
	if len(groupIDs) == 0 {
		return nil
	}
	placeholders, args := sqlPlaceholders(groupIDs)
	query := fmt.Sprintf(
		"SELECT DISTINCT %s FROM users u JOIN audience_group_members agm ON u.id = agm.user_id WHERE agm.audience_group_id IN (%s) AND u.deleted_at IS NULL",
		recipientColumns, placeholders,
	)
	var users []utils.EmailRecipient
	h.repo.RawQuery(query, args, &users)
	return users
}

func sqlPlaceholders(ids []int) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return strings.Join(placeholders, ","), args
}

// dispatchPersonalized envía el correo resolviendo las variables {{...}} con
// los datos de cada destinatario. El HTML se renderiza una sola vez antes de
// llamar aquí; lo único que se repite por persona es la sustitución de tokens,
// y ni eso cuando la plantilla no declara ninguna variable.
func (h *EmailHandler) dispatchPersonalized(recipients map[string]utils.EmailRecipient, subject, htmlContent string) (int, []string) {
	personalize := utils.HasEmailVariables(htmlContent) || utils.HasEmailVariables(subject)

	sent := 0
	var sendErrors []string
	for _, r := range recipients {
		body, subj := htmlContent, subject
		if personalize {
			data := r.VariableData()
			body = utils.RenderVariablesHTML(htmlContent, data)
			subj = utils.RenderVariablesText(subject, data)
		}
		if err := h.brevoSvc.SendEmailKind(service.EmailKindCampaign, r.Email, r.Name, subj, body); err != nil {
			sendErrors = append(sendErrors, fmt.Sprintf("%s: %s", r.Email, err.Error()))
			continue
		}
		sent++
	}
	return sent, sendErrors
}

// SendTestEmail manda una única copia de prueba a quien la pide.
//
// Existe porque la vista previa del editor NO es lo que llega al buzón: la
// compila el navegador, mientras que el envío real la arma aquí y la pasa por
// premailer. Esta prueba recorre exactamente el mismo camino que un envío de
// verdad, así que es la única forma de ver el resultado final.
//
// El destinatario por defecto es quien la pide, y se puede cambiar a otra
// dirección (la corporativa, la de un compañero que revisa el diseño). Lo que
// NO cambia es que va a UNA sola dirección por petición y que el asunto sale
// marcado con [PRUEBA]: eso es lo que impide que esto se convierta en un envío
// masivo por descuido, que era el motivo original de atarlo a uno mismo.
//
// La ruta ya está limitada a superadmin y customer success, que pueden lanzar
// campañas reales de todas formas; obligarles a usar su correo personal para
// ver cómo queda una plantilla no protegía de nada.
func (h *EmailHandler) SendTestEmail(c *gin.Context) {
	var req struct {
		Subject string `json:"subject" binding:"required"`
		// ToEmail cambia el destinatario de la prueba. Vacío = uno mismo.
		ToEmail string `json:"to_email"`
		// Origen del contenido, por orden de preferencia. Los dos primeros
		// pasan por el renderizador real; html_content es para el texto suelto.
		TemplateID  uint   `json:"template_id"`
		Blocks      string `json:"blocks"`
		HTMLContent string `json:"html_content"`
		// Usuario cuyos datos resuelven las variables. Sin él se usan los
		// valores de ejemplo del catálogo.
		AsUserID uint `json:"as_user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	me := h.usersByIDs([]int{int(middleware.GetUserID(c))})
	if len(me) == 0 || me[0].Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tu usuario no tiene un correo donde recibir la prueba"})
		return
	}

	// Destinatario: el propio salvo que se indique otro. Para una dirección
	// ajena no se manda nombre —no se sabe de quién es el buzón— y BrevoService
	// se encarga de poner uno a partir de la propia dirección. El saludo del
	// correo no depende de esto: lo resuelven las variables {{nombre}}.
	toEmail, toName := me[0].Email, me[0].Name
	if custom := strings.TrimSpace(req.ToEmail); custom != "" && !strings.EqualFold(custom, me[0].Email) {
		if !validEmail(custom) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La dirección de prueba no es válida"})
			return
		}
		toEmail, toName = custom, ""
	}

	backendURL := resolveBackendURL(c)
	var html string
	var err error

	switch {
	case req.TemplateID > 0:
		tpl, tplErr := h.repo.GetTemplateByID(req.TemplateID)
		if tplErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plantilla no encontrada"})
			return
		}
		html, err = utils.RenderBlocksToHTML(tpl.Content, backendURL)
	case strings.TrimSpace(req.Blocks) != "":
		html, err = utils.RenderBlocksToHTML(req.Blocks, backendURL)
	case strings.TrimSpace(req.HTMLContent) != "":
		html = rewriteImageURLs(req.HTMLContent, backendURL)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay contenido que probar"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo componer el correo: " + err.Error()})
		return
	}

	// Con datos reales la prueba también valida que los campos existan y se
	// lean bien, no solo que el diseño aguante.
	data := utils.ExampleVariableData()
	viewedAs := "datos de ejemplo"
	if req.AsUserID > 0 {
		if rs := h.usersByIDs([]int{int(req.AsUserID)}); len(rs) > 0 {
			data = rs[0].VariableData()
			viewedAs = rs[0].Name
		}
	}

	subject := "[PRUEBA] " + utils.RenderVariablesText(req.Subject, data)
	body := utils.RenderVariablesHTML(html, data)

	if sendErr := h.brevoSvc.SendEmailKind(service.EmailKindCampaign, toEmail, toName, subject, body); sendErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo enviar la prueba: " + sendErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"to": toEmail, "viewed_as": viewedAs})
}

// GetVariables expone el catálogo de variables de personalización. El editor
// lo consume para ofrecer los mismos tokens en modo visual y en modo código.
func (h *EmailHandler) GetVariables(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"variables": utils.EmailVariables()})
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

	uniqueRecipients := h.resolveRecipients(campaign.RecipientList)

	subject := campaign.Subject
	if subject == "" {
		subject = campaign.Title
	}

	successCount, sendErrors := h.dispatchPersonalized(uniqueRecipients, subject, htmlContent)

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

	// Un envío suelto no pasa por la base de datos, así que solo se pueden
	// resolver las variables que vienen en el propio payload; el resto cae a
	// su valor por defecto.
	data := utils.EmailRecipient{Name: req.ToName, Email: req.ToEmail}.VariableData()
	html = utils.RenderVariablesHTML(html, data)
	subject := utils.RenderVariablesText(req.Subject, data)

	if err := h.brevoSvc.SendEmailKind(service.EmailKindManualComposer, req.ToEmail, req.ToName, subject, html); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al enviar el email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email enviado correctamente", "to": req.ToEmail})
}

// SendQuickEmailBulk sends the same email to multiple recipients at once.
// Los destinatarios llegan de una de dos formas:
//
//   - recipient_list: el formato híbrido ({userIds, groupIds, expressContacts}),
//     que se resuelve contra la base de datos y por tanto permite personalizar
//     con TODAS las variables ({{empresa}}, {{cargo}}, ...).
//   - recipients: una lista suelta de {name, email} para destinatarios que no
//     son usuarios; ahí solo se resuelven nombre y correo.
func (h *EmailHandler) SendQuickEmailBulk(c *gin.Context) {
	var req struct {
		Recipients    []service.BrevoContact `json:"recipients"`
		RecipientList string                 `json:"recipient_list"`
		Subject       string                 `json:"subject" binding:"required"`
		HTMLContent   string                 `json:"html_content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	html := rewriteImageURLs(req.HTMLContent, resolveBackendURL(c))

	recipients := h.resolveRecipients(req.RecipientList)
	for _, r := range req.Recipients {
		key := strings.ToLower(r.Email)
		if r.Email == "" {
			continue
		}
		// No pisa a un usuario ya resuelto: ese trae todos los campos.
		if _, alreadyResolved := recipients[key]; alreadyResolved {
			continue
		}
		recipients[key] = utils.EmailRecipient{Name: r.Name, Email: r.Email}
	}

	if len(recipients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay destinatarios"})
		return
	}

	sent, errStrs := h.dispatchPersonalized(recipients, req.Subject, html)

	resp := gin.H{
		"message": fmt.Sprintf("Enviado a %d de %d destinatarios", sent, len(recipients)),
		"sent":    sent,
		"total":   len(recipients),
	}
	if len(errStrs) > 0 {
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
	uniqueRecipients := h.resolveRecipients(body.RecipientList)
	if len(uniqueRecipients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No recipients resolved"})
		return
	}

	subject := template.Subject
	if subject == "" {
		subject = template.Title
	}

	successCount, sendErrors := h.dispatchPersonalized(uniqueRecipients, subject, htmlContent)

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
