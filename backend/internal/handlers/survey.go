package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

type SurveyHandler struct {
	repo         repository.SurveyRepository
	userRepo     repository.UserRepository
	brevoSvc     *service.BrevoService
	notifRepo    repository.NotificationRepository
}

func NewSurveyHandler(
	repo repository.SurveyRepository,
	userRepo repository.UserRepository,
	brevoSvc *service.BrevoService,
	notifRepo repository.NotificationRepository,
) *SurveyHandler {
	return &SurveyHandler{
		repo:      repo,
		userRepo:  userRepo,
		brevoSvc:  brevoSvc,
		notifRepo: notifRepo,
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

func (h *SurveyHandler) GetSurvey(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	survey, err := h.repo.GetSurveyByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Survey not found"})
		return
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

	response.SurveyID = uint(id)
	userID, _ := c.Get("user_id")
	if uid, ok := userID.(uint); ok {
		response.UserID = uid
	}
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

	// Fetch users
	var users []models.User
	for _, rid := range recipientIDs {
		if user, err := h.userRepo.GetByID(uint(rid)); err == nil {
			users = append(users, *user)
		}
	}

	successCount := 0
	var errors []string

	surveyURL := fmt.Sprintf("https://nuevo.obertrack.com/survey/%d", survey.ID) // Update with actual domain if dynamic

	for _, user := range users {
		userSuccess := false

		// 1. Send In-App Notification
		if survey.SendByInApp {
			notif := &models.Notification{
				UserID:  user.ID,
				Title:   "Nueva Encuesta: " + survey.Title,
				Message: "Tienes una nueva encuesta disponible para responder.",
				Type:    "survey",
				Data:    fmt.Sprintf(`{"link": "/survey/%d"}`, survey.ID),
			}
			if err := h.notifRepo.Create(notif); err != nil {
				errors = append(errors, fmt.Sprintf("Notif fail for %d: %s", user.ID, err.Error()))
			} else {
				userSuccess = true
			}
		}

		// 2. Send Email
		if survey.SendByEmail {
			htmlContent := fmt.Sprintf(`
				<h2>Hola %s,</h2>
				<p>Tienes una nueva encuesta para responder en Obertrack: <strong>%s</strong></p>
				<p>%s</p>
				<br/>
				<a href="%s" style="padding:10px 20px;background:#8b5cf6;color:white;text-decoration:none;border-radius:5px;">Responder Encuesta</a>
			`, user.Name, survey.Title, survey.Description, surveyURL)

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
