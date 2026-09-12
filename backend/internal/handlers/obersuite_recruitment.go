package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// Reclutamiento escribe en el Expediente desde Obersuite. Es la PRIMERA ruta de
// escritura de la integración: hasta aquí todo eran GET y el contrato decía
// "el espejo va en un sentido". Ahora va en los dos, pero solo para esta
// categoría, y con el mismo lenguaje de códigos que /hire para que su lógica de
// reintentos no tenga que aprender nada nuevo.

// recruitmentNoteRequest es el cuerpo del POST, en los términos del formulario
// de Obersuite (vía en español). Todo lo que se valida de verdad se valida en
// el servicio; aquí solo se lee.
type recruitmentNoteRequest struct {
	ExternalID string  `json:"external_id"`
	Category   string  `json:"category"`
	Via        string  `json:"via"`
	Content    string  `json:"content"`
	PersonIDs  []uint  `json:"person_ids"`
	AuthorName string  `json:"author_name"`
	CreatedAt  *string `json:"created_at"`
}

// CreateRecruitmentNote es POST .../companies/:id/timeline.
func (h *ObersuiteCompanyHandler) CreateRecruitmentNote(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}

	var req recruitmentNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el cuerpo no es JSON válido: " + err.Error()})
		return
	}
	// category es obligatoria y hoy solo admite un valor. Se exige igualmente
	// para que el día que haya una segunda categoría escribible el cliente ya
	// la esté mandando, y no haya que adivinar de qué era cada nota vieja.
	if strings.TrimSpace(req.Category) != "recruitment" {
		c.JSON(http.StatusBadRequest, gin.H{"error": `category es obligatoria y en esta versión solo admite "recruitment"`})
		return
	}

	in := service.RecruitmentNoteInput{
		ExternalID: req.ExternalID,
		Via:        req.Via,
		Content:    req.Content,
		PersonIDs:  req.PersonIDs,
		AuthorName: req.AuthorName,
	}
	if req.CreatedAt != nil && strings.TrimSpace(*req.CreatedAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.CreatedAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "created_at tiene que ser una fecha-hora ISO 8601 (p. ej. 2026-09-11T22:10:00Z)"})
			return
		}
		in.CreatedAt = &t
	}

	result, err := h.admin.AddRecruitmentNote(tenant.ID, in)
	if err != nil {
		h.recruitmentError(c, "crear", tenant.ID, req.ExternalID, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// DeleteRecruitmentNote es DELETE .../companies/:id/timeline/:external_id.
// Borra en firme. El segundo DELETE del mismo id es 404 (4xx: no se reintenta).
func (h *ObersuiteCompanyHandler) DeleteRecruitmentNote(c *gin.Context) {
	tenant, ok := h.company(c)
	if !ok {
		return
	}
	externalID := strings.TrimSpace(c.Param("external_id"))
	if externalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "falta el external_id de la nota"})
		return
	}
	if err := h.admin.DeleteRecruitmentNote(tenant.ID, externalID); err != nil {
		h.recruitmentError(c, "borrar", tenant.ID, externalID, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// recruitmentError traduce el error a la MISMA tabla de códigos que /hire: los
// 4xx llevan el texto tal cual (Obersuite se lo enseña al reclutador), los 500
// llevan un texto genérico y un request_id que también va al log.
func (h *ObersuiteCompanyHandler) recruitmentError(c *gin.Context, accion string, companyID uint, externalID string, err error) {
	status, msg := hireStatus(err)
	if status >= 500 {
		rid := hireRequestID()
		log.Printf("[Obersuite] %s nota de reclutamiento falló (empresa %d, external_id=%q) request_id=%s: %v",
			accion, companyID, externalID, rid, err)
		c.JSON(status, gin.H{
			"error":      "no se pudo " + accion + " la nota por un problema en Obertrack. Vuelve a intentarlo en un momento.",
			"request_id": rid,
		})
		return
	}
	c.JSON(status, gin.H{"error": msg})
}
