package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// InductionHandler expone la inducción del profesional recién contratado. Tiene
// dos superficies muy distintas:
//   - Landing / Submit: PÚBLICAS (el profesional aún no tiene cuenta activa;
//     su única credencial es el token del enlace que recibió por correo).
//   - Config / Status / Reset: internas, para configuración y para Soporte.
type InductionHandler struct {
	svc service.InductionService
}

func NewInductionHandler(svc service.InductionService) *InductionHandler {
	return &InductionHandler{svc: svc}
}

// --- Público (sin sesión, autenticado por el token del enlace) ---

// Landing devuelve el video y las preguntas de la invitación. Nunca incluye las
// respuestas correctas.
func (h *InductionHandler) Landing(c *gin.Context) {
	view, err := h.svc.Landing(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

type submitPayload struct {
	Answers []service.SubmittedAnswer `json:"answers"`
}

// Submit califica el intento y aplica la decisión de acceso.
func (h *InductionHandler) Submit(c *gin.Context) {
	var req submitPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Respuestas inválidas"})
		return
	}
	result, err := h.svc.Submit(c.Param("token"), req.Answers)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// --- Interno (configuración y Soporte) ---

func (h *InductionHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type configPayload struct {
	TutorialID    *uint `json:"tutorial_id"`
	SurveyID      *uint `json:"survey_id"`
	PassingScore  int   `json:"passing_score"`
	MaxAttempts   int   `json:"max_attempts"`
	InviteTTLDays int   `json:"invite_ttl_days"`
	IsActive      bool  `json:"is_active"`
}

func (h *InductionHandler) SaveConfig(c *gin.Context) {
	var req configPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfg := &models.InductionConfig{
		TutorialID:    req.TutorialID,
		SurveyID:      req.SurveyID,
		PassingScore:  req.PassingScore,
		MaxAttempts:   req.MaxAttempts,
		InviteTTLDays: req.InviteTTLDays,
		IsActive:      req.IsActive,
	}
	if err := h.svc.SaveConfig(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// Status devuelve el detalle de la inducción de un profesional (intentos,
// puntajes) para que Soporte tenga contexto antes de contactarlo.
func (h *InductionHandler) Status(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	view, err := h.svc.Status(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

// Reset desbloquea al profesional, le devuelve sus intentos y le reenvía el
// enlace. Es la acción que ejecuta Soporte tras contactarlo.
func (h *InductionHandler) Reset(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	if err := h.svc.Reset(uint(userID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Inducción reiniciada. Se reenvió el enlace al profesional."})
}
