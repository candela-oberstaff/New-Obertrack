package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// MeetingHandler expone el módulo de Sesiones (reuniones con Google Meet).
// Ninguna ruta recibe un organizer_id: quien convoca es siempre el usuario de la
// sesión, así que nadie puede crear reuniones en nombre de otro.
type MeetingHandler struct {
	service service.MeetingService
}

func NewMeetingHandler(s service.MeetingService) *MeetingHandler {
	return &MeetingHandler{service: s}
}

// meetingRequest es el cuerpo de creación y edición. Las horas llegan en RFC3339
// (con offset) desde el navegador; TimeZone es la zona IANA con la que el
// organizador convocó, que es lo que necesita Google para las series.
type meetingRequest struct {
	Title           string    `json:"title" binding:"required"`
	Description     string    `json:"description"`
	StartAt         time.Time `json:"start_at" binding:"required"`
	EndAt           time.Time `json:"end_at" binding:"required"`
	TimeZone        string    `json:"time_zone" binding:"required"`
	AttendeeUserIDs []uint    `json:"attendee_user_ids"`
	AttendeeEmails  []string  `json:"attendee_emails"`
	TaskID          *uint     `json:"task_id"`
	RecurrenceRule  string    `json:"recurrence_rule"`
}

func (r meetingRequest) toInput() service.MeetingInput {
	return service.MeetingInput{
		Title:           r.Title,
		Description:     r.Description,
		StartAt:         r.StartAt,
		EndAt:           r.EndAt,
		TimeZone:        r.TimeZone,
		AttendeeUserIDs: r.AttendeeUserIDs,
		AttendeeEmails:  r.AttendeeEmails,
		TaskID:          r.TaskID,
		RecurrenceRule:  r.RecurrenceRule,
	}
}

func (h *MeetingHandler) List(c *gin.Context) {
	past := c.Query("past") == "true"
	taskID, _ := strconv.ParseUint(c.Query("task_id"), 10, 64)

	sessions, err := h.service.List(
		middleware.GetTenantID(c), middleware.GetUserID(c), past, uint(taskID),
	)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meetings": sessions})
}

// Upcoming alimenta el widget del Dashboard: las siguientes sesiones del usuario.
func (h *MeetingHandler) Upcoming(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "3"))

	sessions, err := h.service.Upcoming(middleware.GetTenantID(c), middleware.GetUserID(c), limit)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meetings": sessions})
}

func (h *MeetingHandler) Get(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	session, err := h.service.Get(id, middleware.GetUserID(c))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meeting": session})
}

// Presence devuelve quién está conectado ahora a la sala. Lo consulta el
// frontend en bucle mientras la sesión está en curso, así que responde 200 con
// live:false en los casos "sala vacía" en vez de error: un contador no debe
// llenar la pantalla de avisos rojos.
func (h *MeetingHandler) Presence(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	presence, err := h.service.Presence(id, middleware.GetUserID(c))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, presence)
}

func (h *MeetingHandler) Create(c *gin.Context) {
	var req meetingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.service.Create(middleware.GetUserID(c), middleware.GetTenantID(c), req.toInput())
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"meeting": session})
}

func (h *MeetingHandler) Update(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	var req meetingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.service.Update(id, middleware.GetUserID(c), req.toInput())
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meeting": session})
}

func (h *MeetingHandler) Cancel(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	if err := h.service.Cancel(id, middleware.GetUserID(c)); err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cancelled": true})
}

func (h *MeetingHandler) parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Identificador de sesión inválido"})
		return 0, false
	}
	return uint(id), true
}

// respondError traduce los errores del servicio a códigos HTTP. Los dos que
// importan de verdad son los de Google: 'no conectado' y 'needs_reauth' NO son
// fallos del sistema sino estados que el usuario resuelve él mismo, así que el
// frontend los distingue por su código para mostrar el botón de conectar en vez
// de un error genérico.
func (h *MeetingHandler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMeetingValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrMeetingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "La sesión no existe"})
	case errors.Is(err, service.ErrMeetingForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrGoogleNotConnected):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "google_not_connected": true})
	case errors.Is(err, service.ErrNeedsReauth):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "needs_reauth": true})
	// El token se emitió antes de que pidiéramos meetings.space.readonly. Se
	// distingue de needs_reauth porque la cuenta sigue sirviendo para todo lo
	// demás: solo falta el permiso del contador de asistentes.
	case errors.Is(err, service.ErrMeetScopeMissing):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "meet_scope_missing": true})
	case errors.Is(err, service.ErrGoogleDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "La integración con Google no está disponible"})
	case errors.Is(err, service.ErrGooglePermanent):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		log.Printf("[meetings] error inesperado: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo completar la operación"})
	}
}
