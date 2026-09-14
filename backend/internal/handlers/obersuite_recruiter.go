package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// El reclutador de Obersuite que lleva la empresa: PUT para asignar o
// reemplazar, DELETE para quitar. Misma tabla de códigos que /hire.

type recruiterRequest struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
}

// SetRecruiter es PUT .../companies/:id/recruiter.
func (h *ObersuiteCompanyHandler) SetRecruiter(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	var req recruiterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el cuerpo no es JSON válido: " + err.Error()})
		return
	}
	cr, err := h.admin.SetTenantRecruiter(tenant.ID, service.RecruiterInput{
		ExternalID: req.ExternalID, Name: req.Name, Email: req.Email,
	})
	if err != nil {
		h.recruitmentError(c, "asignar el reclutador de", tenant.ID, req.ExternalID, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "assigned", "recruiter": recruiterContact(cr)})
}

// ClearRecruiter es DELETE .../companies/:id/recruiter.
func (h *ObersuiteCompanyHandler) ClearRecruiter(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	if err := h.admin.ClearTenantRecruiter(tenant.ID); err != nil {
		h.recruitmentError(c, "quitar el reclutador de", tenant.ID, "", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}

// recruiterContact es el reclutador en la forma común de la integración
// (ObersuiteContact), o nil. El id es el SUYO (external_id): es lo que usan
// para filtrar.
func recruiterContact(cr *models.CompanyRecruiter) gin.H {
	if cr == nil {
		return nil
	}
	return gin.H{
		"id":          cr.ExternalID,
		"name":        cr.Name,
		"email":       cr.Email,
		"assigned_at": cr.AssignedAt,
	}
}
