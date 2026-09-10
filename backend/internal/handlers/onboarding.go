package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// OnboardingHandler expone el puente con Obersuite en los dos sentidos:
// entrada (webhook de contratación) y salida (padrón de empresas). Sus rutas viven
// bajo /api/integrations/obersuite y se protegen con un token de servicio
// estático (middleware.SharedSecretAuth), no con sesión de usuario.
type OnboardingHandler struct {
	svc service.OnboardingService
}

func NewOnboardingHandler(svc service.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{svc: svc}
}

// ListCompanies devuelve el padrón completo de empresas con su ficha para la
// integración con Obersuite: identidad, ubicación, alta, contadores, operación
// (horas, pendientes, tickets) y las dos señales —último contacto nuestro y
// última actividad suya—.
//
// ?updated_since=<RFC3339> acota a lo que cambió después de ese instante, para
// sincronizar en incremental. Sin el parámetro devuelve todas, que es como
// funcionaba antes: Obersuite ya llama a este endpoint y no se le puede cambiar
// el comportamiento por defecto.
//
// Una fecha que no se entiende se rechaza en vez de ignorarse: tratarla como
// "sin filtro" devolvería el padrón entero y el que llama creería estar
// recibiendo solo lo nuevo.
func (h *OnboardingHandler) ListCompanies(c *gin.Context) {
	var updatedSince *time.Time
	if raw := c.Query("updated_since"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "updated_since debe ser una fecha RFC3339, por ejemplo 2026-09-09T13:00:00Z",
			})
			return
		}
		updatedSince = &parsed
	}

	companies, err := h.svc.ListCompanies(updatedSince)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}

// hireCVPayload es el CV embebido en base64 (ver HireCV en el servicio).
type hireCVPayload struct {
	FileName      string `json:"file_name"`
	MimeType      string `json:"mime_type"`
	ContentBase64 string `json:"content_base64"`
}

// hirePayload es el cuerpo del webhook de contratación de Obersuite.
type hirePayload struct {
	ExternalID       string         `json:"external_id"`
	Email            string         `json:"email" binding:"required,email"`
	Name             string         `json:"name" binding:"required"`
	IdentityDocument string         `json:"identity_document"`
	PhoneNumber      string         `json:"phone_number"`
	Country          string         `json:"country"`
	State            string         `json:"state"`
	City             string         `json:"city"`
	Address          string         `json:"address"`
	JobTitle         string         `json:"job_title"`
	CompanyID        uint           `json:"company_id" binding:"required"`
	StartedAt        string         `json:"started_at"` // YYYY-MM-DD (opcional)
	CV               *hireCVPayload `json:"cv"`
}

// Hire recibe la contratación desde Obersuite y materializa al profesional con
// su empleo activo. Idempotente por email (ver OnboardingService.Hire).
func (h *OnboardingHandler) Hire(c *gin.Context) {
	var req hirePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	in := service.HireRequest{
		ExternalID:       req.ExternalID,
		Email:            req.Email,
		Name:             req.Name,
		IdentityDocument: req.IdentityDocument,
		PhoneNumber:      req.PhoneNumber,
		Country:          req.Country,
		State:            req.State,
		City:             req.City,
		Address:          req.Address,
		JobTitle:         req.JobTitle,
		CompanyID:        req.CompanyID,
		StartedAt:        parseDatePtr(req.StartedAt),
	}
	if req.CV != nil {
		in.CV = &service.HireCV{
			FileName:      req.CV.FileName,
			MimeType:      req.CV.MimeType,
			ContentBase64: req.CV.ContentBase64,
		}
	}

	result, err := h.svc.Hire(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
