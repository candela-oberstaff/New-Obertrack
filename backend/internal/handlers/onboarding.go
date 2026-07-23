package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// OnboardingHandler expone el puente Obersuite → Obertrack. Sus rutas viven
// bajo /api/integrations/obersuite y se protegen con un token de servicio
// estático (middleware.SharedSecretAuth), no con sesión de usuario.
type OnboardingHandler struct {
	svc service.OnboardingService
}

func NewOnboardingHandler(svc service.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{svc: svc}
}

// ListCompanies devuelve las empresas [{id,name}] para alimentar el dropdown de
// contratación en Obersuite. El reclutador elige una y nos envía su id.
func (h *OnboardingHandler) ListCompanies(c *gin.Context) {
	companies, err := h.svc.ListCompanies()
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
