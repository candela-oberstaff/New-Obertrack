package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// Empresas escritas desde Obersuite. Misma tabla de códigos que /hire; los
// 4xx antes de tocar nada; cada operación deja línea en el log.

type obersuiteCompanyCreateRequest struct {
	ExternalID        string `json:"external_id"`
	Name              string `json:"name"`
	ResponsibleName   string `json:"responsible_name"`
	ResponsibleEmail  string `json:"responsible_email"`
	PhoneNumber       string `json:"phone_number"`
	Industry          string `json:"industry"`
	Country           string `json:"country"`
	State             string `json:"state"`
	City              string `json:"city"`
	Address           string `json:"address"`
	CustomerSuccessID *uint  `json:"customer_success_id"`
}

// Create es POST /companies.
func (h *ObersuiteCompanyHandler) Create(c *gin.Context) {
	var req obersuiteCompanyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el cuerpo no es JSON válido: " + err.Error()})
		return
	}
	res, err := h.admin.CreateCompanyFromObersuite(service.ObersuiteCompanyCreate{
		ExternalID: req.ExternalID, Name: req.Name,
		ResponsibleName: req.ResponsibleName, ResponsibleEmail: req.ResponsibleEmail,
		PhoneNumber: req.PhoneNumber, Industry: req.Industry,
		Country: req.Country, State: req.State, City: req.City, Address: req.Address,
		CustomerSuccessID: req.CustomerSuccessID,
	})
	if err != nil {
		h.recruitmentError(c, "crear la empresa", 0, req.ExternalID, err)
		return
	}
	log.Printf("[Obersuite] empresa external_id=%q → %s (id=%d)", req.ExternalID, res.Status, res.ID)
	c.JSON(http.StatusOK, res)
}

// obersuiteCompanyPatchRequest: punteros para distinguir "no viajó" de "vacío".
type obersuiteCompanyPatchRequest struct {
	Name             *string `json:"name"`
	ResponsibleName  *string `json:"responsible_name"`
	ResponsibleEmail *string `json:"responsible_email"`
	PhoneNumber      *string `json:"phone_number"`
	Industry         *string `json:"industry"`
	Country          *string `json:"country"`
	State            *string `json:"state"`
	City             *string `json:"city"`
	Address          *string `json:"address"`
	ExternalID       *string `json:"external_id"`
}

// Update es PUT /companies/:id. Parcial: lo que no viaja no se toca.
func (h *ObersuiteCompanyHandler) Update(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	var req obersuiteCompanyPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el cuerpo no es JSON válido: " + err.Error()})
		return
	}
	// Los dos campos que NO se editan por aquí se rechazan, no se ignoran:
	// ignorarlos haría creer que el cambio se aplicó.
	if req.ResponsibleEmail != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "responsible_email no se edita por la integración: es el correo con el que la empresa entra en Obertrack. Se cambia desde la ficha en Obertrack."})
		return
	}
	if req.ExternalID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "external_id no se puede cambiar: es la identidad de la empresa en la integración"})
		return
	}
	changed, err := h.admin.UpdateCompanyFromObersuite(tenant.ID, service.ObersuiteCompanyPatch{
		Name: req.Name, ResponsibleName: req.ResponsibleName, PhoneNumber: req.PhoneNumber,
		Industry: req.Industry, Country: req.Country, State: req.State, City: req.City, Address: req.Address,
	})
	if err != nil {
		h.recruitmentError(c, "editar la empresa", tenant.ID, "", err)
		return
	}
	log.Printf("[Obersuite] empresa %d editada: %s", tenant.ID, strings.Join(changed, ","))
	c.JSON(http.StatusOK, gin.H{"id": tenant.ID, "updated_fields": changed})
}

// Industries es GET /industries: la lista cerrada de rubros.
func (h *ObersuiteCompanyHandler) Industries(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"industries": models.Industries})
}

// Analysts es GET /analysts: a quién se puede asignar.
func (h *ObersuiteCompanyHandler) Analysts(c *gin.Context) {
	list, err := h.admin.ListAssignableAnalysts()
	if err != nil {
		h.recruitmentError(c, "listar los analistas", 0, "", err)
		return
	}
	if list == nil {
		list = []service.ObersuiteAnalyst{}
	}
	c.JSON(http.StatusOK, gin.H{"analysts": list})
}

// SetAnalyst es PUT /companies/:id/customer-success. analyst_id nulo o 0 quita.
func (h *ObersuiteCompanyHandler) SetAnalyst(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	var req struct {
		AnalystID *uint `json:"analyst_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el cuerpo no es JSON válido: " + err.Error()})
		return
	}
	if err := h.admin.SetCompanyAnalystFromObersuite(tenant.ID, req.AnalystID); err != nil {
		h.recruitmentError(c, "asignar el analista de", tenant.ID, "", err)
		return
	}
	status := "cleared"
	if req.AnalystID != nil && *req.AnalystID != 0 {
		status = "assigned"
	}
	log.Printf("[Obersuite] empresa %d analista → %s", tenant.ID, status)
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// Suspend es POST /companies/:id/suspend.
func (h *ObersuiteCompanyHandler) Suspend(c *gin.Context) {
	h.setStatus(c, false)
}

// Reactivate es POST /companies/:id/reactivate.
func (h *ObersuiteCompanyHandler) Reactivate(c *gin.Context) {
	h.setStatus(c, true)
}

func (h *ObersuiteCompanyHandler) setStatus(c *gin.Context, active bool) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	// El cuerpo es opcional (reactivar no lleva nada).
	_ = c.ShouldBindJSON(&req)
	if err := h.admin.SetCompanyStatusFromObersuite(tenant.ID, active, req.Reason); err != nil {
		h.recruitmentError(c, "cambiar el estado de", tenant.ID, "", err)
		return
	}
	status := "suspended"
	if active {
		status = "active"
	}
	log.Printf("[Obersuite] empresa %d → %s (motivo=%q)", tenant.ID, status, req.Reason)
	c.JSON(http.StatusOK, gin.H{"status": status})
}
