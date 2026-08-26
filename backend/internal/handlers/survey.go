package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/utils"
)

// surveyHMACSecret returns the secret key used for signing survey quick-response tokens.
// Prefers a dedicated SURVEY_TOKEN_SECRET, but falls back to JWT_SECRET (validated
// to be strong at startup) instead of a public hardcoded default that would make
// quick-response tokens forgeable.
func surveyHMACSecret() string {
	if secret := os.Getenv("SURVEY_TOKEN_SECRET"); secret != "" {
		return secret
	}
	return os.Getenv("JWT_SECRET")
}

// generateSurveyToken creates an HMAC-SHA256 token for a survey+user combination.
func generateSurveyToken(surveyID, userID uint) string {
	mac := hmac.New(sha256.New, []byte(surveyHMACSecret()))
	mac.Write([]byte(fmt.Sprintf("%d:%d", surveyID, userID)))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifySurveyToken checks that the provided token matches the expected HMAC for survey+user.
func verifySurveyToken(surveyID, userID uint, token string) bool {
	expected := generateSurveyToken(surveyID, userID)
	return hmac.Equal([]byte(expected), []byte(token))
}

type SurveyHandler struct {
	repo     repository.SurveyRepository
	userRepo repository.UserRepository
	brevoSvc *service.BrevoService
	notifSvc service.NotificationService
}

func NewSurveyHandler(
	repo repository.SurveyRepository,
	userRepo repository.UserRepository,
	brevoSvc *service.BrevoService,
	notifSvc service.NotificationService,
) *SurveyHandler {
	return &SurveyHandler{
		repo:     repo,
		userRepo: userRepo,
		brevoSvc: brevoSvc,
		notifSvc: notifSvc,
	}
}

func (h *SurveyHandler) CreateSurvey(c *gin.Context) {
	var survey models.Survey
	if err := c.ShouldBindJSON(&survey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(uint); ok {
		survey.CreatedBy = uid
	}

	if err := h.repo.CreateSurvey(&survey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create survey"})
		return
	}

	c.JSON(http.StatusCreated, survey)
}

func (h *SurveyHandler) GetSurveys(c *gin.Context) {
	surveys, err := h.repo.GetSurveys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch surveys"})
		return
	}
	c.JSON(http.StatusOK, surveys)
}

func surveyHasRecipient(recipientList string, userID uint) bool {
	if recipientList == "" {
		return false
	}
	var ids []int
	if err := json.Unmarshal([]byte(recipientList), &ids); err != nil {
		return false
	}
	for _, id := range ids {
		if uint(id) == userID {
			return true
		}
	}
	return false
}

func (h *SurveyHandler) GetSurvey(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	survey, err := h.repo.GetSurveyByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Survey not found"})
		return
	}

	if !middleware.IsSuperadmin(c) {
		if !surveyHasRecipient(survey.RecipientList, middleware.GetUserID(c)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		survey.Responses = nil
		// Quien responde nunca debe ver la respuesta esperada ni la ponderación
		// de un cuestionario calificado (inducción).
		for i := range survey.Questions {
			survey.Questions[i].CorrectAnswer = ""
		}
	}

	c.JSON(http.StatusOK, survey)
}

func (h *SurveyHandler) SubmitResponse(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var response models.SurveyResponse
	if err := c.ShouldBindJSON(&response); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	if !middleware.IsSuperadmin(c) {
		survey, err := h.repo.GetSurveyByID(uint(id))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Survey not found"})
			return
		}
		if !surveyHasRecipient(survey.RecipientList, userID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	response.SurveyID = uint(id)
	response.UserID = userID
	now := time.Now()
	response.CompletedAt = &now

	if err := h.repo.CreateResponse(&response); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit response"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// SendSurvey dispatches the survey to the specified recipients via Email and/or In-App Notification
func (h *SurveyHandler) SendSurvey(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	survey, err := h.repo.GetSurveyByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Survey not found"})
		return
	}

	// El cuerpo puede traer una lista de destinatarios que reemplaza a la
	// guardada en la encuesta (p. ej. el envío masivo desde el panel de
	// usuarios). No se persiste: solo aplica a este envío.
	var body struct {
		RecipientList *string `json:"recipient_list"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.RecipientList != nil {
		survey.RecipientList = *body.RecipientList
	}

	// Parse recipient IDs
	var recipientIDs []int
	if survey.RecipientList != "" {
		if err := json.Unmarshal([]byte(survey.RecipientList), &recipientIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse recipient list"})
			return
		}
	}

	if len(recipientIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No elegiste destinatarios. Abre la encuesta, entra en Configuración y marca a quién va dirigida.",
		})
		return
	}
	// Nota: más abajo se descartan los destinatarios de otra empresa (salvo para un
	// superadmin). Si se quedan TODOS fuera, `users` viene vacío y la respuesta lo
	// dice explícitamente en vez de contestar "enviada a 0".

	// Fetch users (respect tenant unless superadmin)
	var users []models.User
	tenantID := middleware.GetTenantID(c)
	isSuper := middleware.IsSuperadmin(c)
	for _, rid := range recipientIDs {
		if user, err := h.userRepo.GetByID(uint(rid)); err == nil {
			if !isSuper {
				if models.TenantForUser(user) != tenantID {
					continue
				}
			}
			users = append(users, *user)
		}
	}

	// Se eligieron destinatarios pero no quedó ninguno: o ya no existen, o son de otra
	// empresa y el filtro de arriba los descartó. Sin esto la respuesta era "enviada"
	// con sent=0, que es la peor forma de fallar: la encuesta se marcaba activa y
	// nadie la había recibido.
	if len(users) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ninguno de los destinatarios elegidos puede recibirla: o ya no están, o pertenecen a otra empresa. Vuelve a elegirlos en Configuración.",
		})
		return
	}

	successCount := 0
	var errors []string

	frontendURL := os.Getenv("SERVICE_URL_FRONTEND")
	if frontendURL == "" {
		frontendURL = "https://obertrack.com"
	}

	surveyURL := fmt.Sprintf("%s/survey/%d", frontendURL, survey.ID)

	for _, user := range users {
		userSuccess := false

		// 1. Send In-App Notification
		if survey.SendByInApp {
			errNotif := h.notifSvc.CreateNotification(
				user.ID,
				"survey",
				"Nueva Encuesta: "+survey.Title,
				"Tienes una nueva encuesta disponible para responder.",
				map[string]interface{}{"link": fmt.Sprintf("/survey/%d", survey.ID)},
			)
			if errNotif != nil {
				errors = append(errors, fmt.Sprintf("Notif fail for %d: %s", user.ID, errNotif.Error()))
			} else {
				userSuccess = true
			}
		}

		// 2. Send Email
		if survey.SendByEmail {
			actionHtml := fmt.Sprintf(`
				<div style="text-align: center; margin-top: 30px;">
					<a href="%s" class="btn-primary" style="display:inline-block; padding:12px 24px; background-color:#cc33cc; color:#ffffff; text-decoration:none; border-radius:8px; font-weight:600;">Responder Encuesta</a>
				</div>
			`, surveyURL)

			rawContent := fmt.Sprintf(`
				<h2 style="margin-top: 0; color: #1e293b;">Hola %s,</h2>
				<p style="color: #334155;">Tienes una nueva encuesta para responder en Obertrack: <strong>%s</strong></p>
				<p style="color: #475569;">%s</p>
				%s
			`, user.Name, survey.Title, survey.Description, actionHtml)

			htmlContent := utils.WrapInPremiumTemplate("Nueva Encuesta: "+survey.Title, rawContent)

			if err := h.brevoSvc.SendEmailKind(service.EmailKindSurveyInvite, user.Email, user.Name, "Nueva Encuesta: "+survey.Title, htmlContent); err != nil {
				errors = append(errors, fmt.Sprintf("Email fail for %s: %s", user.Email, err.Error()))
			} else {
				userSuccess = true
			}
		}

		if userSuccess {
			successCount++
		}
	}

	// Ni una sola vía funcionó. El motivo más común no es un fallo técnico: es que la
	// encuesta va sólo por correo y ese tipo de correo está apagado en Configuración →
	// Correos. Decirlo aquí ahorra el viaje de ir a mirar los registros del servidor.
	if successCount == 0 && len(users) > 0 {
		motivo := "No se pudo enviar la encuesta a ningún destinatario."
		if survey.SendByEmail && !survey.SendByInApp && !h.brevoSvc.AllowsKind(service.EmailKindSurveyInvite) {
			motivo = "La encuesta va sólo por correo y los correos de encuesta están apagados en Configuración → Correos. Enciéndelos, o marca también el aviso dentro de la aplicación."
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": motivo, "errors": errors})
		return
	}

	// Salió para unos y no para otros: la respuesta ya lleva `sent` y `errors`, y la
	// pantalla avisa de quién se quedó fuera en vez de cantar un éxito completo.

	survey.Status = models.SurveyStatusActive
	h.repo.UpdateSurvey(survey)

	c.JSON(http.StatusOK, gin.H{
		"message": "Survey dispatched",
		"sent":    successCount,
		"errors":  errors,
	})
}
func (h *SurveyHandler) UpdateSurvey(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var survey models.Survey
	if err := c.ShouldBindJSON(&survey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	survey.ID = uint(id)
	if err := h.repo.UpdateSurvey(&survey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survey"})
		return
	}

	c.JSON(http.StatusOK, survey)
}

func (h *SurveyHandler) DeleteSurvey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	if err := h.repo.DeleteSurvey(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar encuesta"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Encuesta eliminada"})
}

// QuickResponse handles one-click submissions from email
func (h *SurveyHandler) QuickResponse(c *gin.Context) {
	surveyIDStr := c.Param("id")
	userIDStr := c.Query("user_id")
	questionIDStr := c.Query("q_id")
	scoreStr := c.Query("score")
	token := c.Query("t")

	surveyID, err1 := strconv.ParseUint(surveyIDStr, 10, 32)
	userID, err2 := strconv.ParseUint(userIDStr, 10, 32)
	questionID, err3 := strconv.ParseUint(questionIDStr, 10, 32)
	score, err4 := strconv.Atoi(scoreStr)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		c.String(http.StatusBadRequest, "Parámetros inválidos.")
		return
	}

	// Verify HMAC token to prevent impersonation
	if token == "" || !verifySurveyToken(uint(surveyID), uint(userID), token) {
		c.String(http.StatusForbidden, "Token de validación inválido o expirado.")
		return
	}

	// Validate survey exists and is active
	survey, err := h.repo.GetSurveyByID(uint(surveyID))
	if err != nil || survey == nil {
		c.String(http.StatusNotFound, "Encuesta no encontrada.")
		return
	}
	if survey.Status != models.SurveyStatusActive {
		c.String(http.StatusBadRequest, "Esta encuesta ya no está activa.")
		return
	}

	// Validate question belongs to this survey
	questionValid := false
	for _, q := range survey.Questions {
		if q.ID == uint(questionID) {
			questionValid = true
			break
		}
	}
	if !questionValid {
		c.String(http.StatusBadRequest, "Pregunta no válida para esta encuesta.")
		return
	}

	// Validate score range
	if score < 1 || score > 10 {
		c.String(http.StatusBadRequest, "El puntaje debe estar entre 1 y 10.")
		return
	}

	// Save the response
	now := time.Now()
	response := models.SurveyResponse{
		SurveyID:    uint(surveyID),
		UserID:      uint(userID),
		CompletedAt: &now,
		Answers: []models.SurveyAnswer{
			{
				QuestionID:  uint(questionID),
				NumberValue: score,
			},
		},
	}

	// We ignore duplicates here for simplicity.
	// In production, we might check if response already exists and update.
	_ = h.repo.CreateResponse(&response)

	frontendURL := os.Getenv("SERVICE_URL_FRONTEND")
	if frontendURL == "" {
		frontendURL = "https://obertrack.com"
	}

	// Return a premium HTML thank you page
	rawHTML := fmt.Sprintf(`
		<div style="text-align: center; padding: 40px 20px;">
			<div style="background-color: #dcfce7; color: #15803d; width: 64px; height: 64px; border-radius: 50%%; display: flex; align-items: center; justify-content: center; font-size: 32px; margin: 0 auto 20px;">&#10003;</div>
			<h2 style="margin-top: 0; color: #1e293b;">¡Gracias por tu respuesta!</h2>
			<p style="color: #64748b; font-size: 16px;">Tu valoración de <strong>%d</strong> ha sido registrada exitosamente.</p>
			<div style="margin-top: 30px;">
				<a href="%s/survey/%d" class="btn-primary" style="display:inline-block; padding:12px 24px; background-color:#cc33cc; color:#ffffff; text-decoration:none; border-radius:8px; font-weight:600;">Ver encuesta completa</a>
			</div>
		</div>
	`, score, frontendURL, surveyID)

	finalHTML := utils.WrapInPremiumTemplate("Respuesta Registrada", rawHTML)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, finalHTML)
}
