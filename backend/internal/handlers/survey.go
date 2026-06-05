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
func surveyHMACSecret() string {
	secret := os.Getenv("SURVEY_TOKEN_SECRET")
	if secret == "" {
		secret = "obertrack-survey-default-secret"
	}
	return secret
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
	repo         repository.SurveyRepository
	userRepo     repository.UserRepository
	brevoSvc     *service.BrevoService
	notifSvc     service.NotificationService
}

func NewSurveyHandler(
	repo repository.SurveyRepository,
	userRepo repository.UserRepository,
	brevoSvc *service.BrevoService,
	notifSvc service.NotificationService,
) *SurveyHandler {
	return &SurveyHandler{
		repo:      repo,
		userRepo:  userRepo,
		brevoSvc:  brevoSvc,
		notifSvc:  notifSvc,
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

	if survey.Status == models.SurveyStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Survey is already sent/active"})
		return
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "No recipients specified"})
		return
	}

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
			var actionHtml string

			// Check for rating question to create quick-response buttons
			var ratingQuestion *models.SurveyQuestion
			for _, q := range survey.Questions {
				if q.Type == models.QuestionTypeRating {
					ratingQuestion = &q
					break
				}
			}

			if ratingQuestion != nil {
				actionHtml = fmt.Sprintf(`
					<h3 style="text-align: center; margin-top: 30px; color: #1e293b;">%s</h3>
					<div class="rating-container">
				`, ratingQuestion.Text)

				for i := 1; i <= 5; i++ {
					token := generateSurveyToken(survey.ID, user.ID)
					quickLink := fmt.Sprintf("%s/api/surveys/%d/quick-response?user_id=%d&q_id=%d&score=%d&t=%s", frontendURL, survey.ID, user.ID, ratingQuestion.ID, i, token)
					actionHtml += fmt.Sprintf(`<a href="%s" class="rating-btn" style="text-align:center; text-decoration:none; display:inline-block; width:45px; height:45px; line-height:45px; background-color:#f1f5f9; color:#cc33cc; border-radius:50%%; margin:0 5px; font-weight:bold; font-size:18px;">%d</a>`, quickLink, i)
				}
				
				actionHtml += `
					</div>
					<div style="text-align: center; margin-top: 20px;">
						<a href="`+surveyURL+`" style="color: #cc33cc; text-decoration: underline; font-size: 14px;">Ir a la encuesta completa</a>
					</div>
				`
			} else {
				actionHtml = fmt.Sprintf(`
					<div style="text-align: center; margin-top: 30px;">
						<a href="%s" class="btn-primary" style="display:inline-block; padding:12px 24px; background-color:#cc33cc; color:#ffffff; text-decoration:none; border-radius:8px; font-weight:600;">Responder Encuesta</a>
					</div>
				`, surveyURL)
			}

			rawContent := fmt.Sprintf(`
				<h2 style="margin-top: 0; color: #1e293b;">Hola %s,</h2>
				<p style="color: #334155;">Tienes una nueva encuesta para responder en Obertrack: <strong>%s</strong></p>
				<p style="color: #475569;">%s</p>
				%s
			`, user.Name, survey.Title, survey.Description, actionHtml)

			htmlContent := utils.WrapInPremiumTemplate("Nueva Encuesta: "+survey.Title, rawContent)

			if err := h.brevoSvc.SendEmail(user.Email, user.Name, "Nueva Encuesta: "+survey.Title, htmlContent); err != nil {
				errors = append(errors, fmt.Sprintf("Email fail for %s: %s", user.Email, err.Error()))
			} else {
				userSuccess = true
			}
		}

		if userSuccess {
			successCount++
		}
	}

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
