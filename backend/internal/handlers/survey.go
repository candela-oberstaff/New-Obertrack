package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

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

	surveyURL := fmt.Sprintf("%s/survey/%d", surveyFrontendURL(), survey.ID)
	// Los botones de un clic apuntan al backend (son ellos los que registran la
	// respuesta), no al sitio: por eso hace falta también su dominio público.
	backendURL := resolveBackendURL(c)

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
			// Aquí se bifurca: una cuenta EMPRESA recibe un correo que se
			// responde SIN iniciar sesión —enlace firmado y, si la encuesta abre
			// con una valoración, los botones de un clic—. Cualquier otro rol
			// sigue recibiendo el de siempre, que entra por la aplicación. La
			// bifurcación vive aquí y no dentro de la plantilla para que se vea
			// de un vistazo a quién se le está levantando la restricción.
			var htmlContent string
			if user.UserType == models.UserTypeEmployer {
				token := newSurveyLinkToken(survey.ID, user.ID, time.Now().Add(surveyLinkTTL))
				question, options := surveyQuickOptions(survey, backendURL, token)
				htmlContent = service.BuildSurveyCompanyInviteHTML(
					user.Name, survey.Title, survey.Description,
					surveyPublicLink(token), question, options, int(surveyLinkTTL.Hours()/24),
				)
			} else {
				htmlContent = service.BuildSurveyInviteHTML(user.Name, survey.Title, survey.Description, surveyURL)
			}

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
		// Quitar una pregunta ya contestada no es una avería: es una decisión que le
		// toca a quien edita, y el mensaje le dice qué puede hacer en su lugar.
		var respondida repository.ErrPreguntaRespondida
		if errors.As(err, &respondida) {
			c.JSON(http.StatusConflict, gin.H{"error": respondida.Error()})
			return
		}
		// Cualquier otro fallo se cuenta tal cual y se deja en el registro. Antes se
		// contestaba "Failed to update survey" y el motivo real no aparecía en
		// ningún sitio: ni en pantalla ni en los registros del servidor.
		log.Printf("survey %d: no se pudo guardar: %v", survey.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No se pudo guardar la encuesta: " + err.Error(),
		})
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
