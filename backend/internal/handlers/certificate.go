package handlers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

// CertificateHandler expone las plantillas de certificado (panel), la
// descarga y el listado para quien puede ver a la persona, y la verificación
// pública por código.
type CertificateHandler struct {
	svc   service.CertificateService
	users service.UserService
}

func NewCertificateHandler(svc service.CertificateService, users service.UserService) *CertificateHandler {
	return &CertificateHandler{svc: svc, users: users}
}

// --- Plantillas ---

func (h *CertificateHandler) ListTemplates(c *gin.Context) {
	templates, err := h.svc.ListTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func (h *CertificateHandler) CreateTemplate(c *gin.Context) {
	var in service.TemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	t, err := h.svc.CreateTemplate(middleware.GetUserID(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *CertificateHandler) UpdateTemplate(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plantilla inválida"})
		return
	}
	var in service.TemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	t, err := h.svc.UpdateTemplate(id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *CertificateHandler) DeleteTemplate(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plantilla inválida"})
		return
	}
	if err := h.svc.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plantilla eliminada"})
}

// PreviewTemplate renderiza la plantilla del cuerpo (guardada o no) con datos
// de ejemplo y devuelve el PDF, para verla antes de guardar.
func (h *CertificateHandler) PreviewTemplate(c *gin.Context) {
	var in service.TemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	pdf, err := h.svc.Preview(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", "inline; filename=\"vista-previa-certificado.pdf\"")
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// --- Certificados ---

func (h *CertificateHandler) Mine(c *gin.Context) {
	certs, err := h.svc.ListForUser(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los certificados"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": certs})
}

// canSeeUser reusa la visibilidad del detalle de usuario: superadmin y
// soporte ven a cualquiera, la empresa a los suyos, cada quien a sí mismo.
func (h *CertificateHandler) canSeeUser(c *gin.Context, userID uint) (int, error) {
	if middleware.GetUserID(c) == userID {
		return 0, nil
	}
	if _, err := h.users.GetByID(userID, middleware.GetUserID(c), middleware.GetTenantID(c), middleware.GetUserRole(c), middleware.IsSuperadmin(c)); err != nil {
		if err.Error() == "Access denied" {
			return http.StatusForbidden, err
		}
		return http.StatusNotFound, err
	}
	return 0, nil
}

func (h *CertificateHandler) ForUser(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	if status, err := h.canSeeUser(c, id); err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	certs, err := h.svc.ListForUser(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los certificados"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": certs})
}

// Download sirve el PDF a quien puede ver a la persona.
func (h *CertificateHandler) Download(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Certificado inválido"})
		return
	}
	cert, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if status, err := h.canSeeUser(c, cert.UserID); err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.serve(c, cert.Code, h.svc.FilePath(cert))
}

func (h *CertificateHandler) serve(c *gin.Context, code, path string) {
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "El archivo del certificado no está disponible"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"certificado-%s.pdf\"", code))
	c.File(path)
}

// ForProgram devuelve cuántos certificados se emitieron en un programa y los
// más recientes, para el editor del programa.
func (h *CertificateHandler) ForProgram(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Programa inválido"})
		return
	}
	view, err := h.svc.ListForProgram(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los certificados"})
		return
	}
	c.JSON(http.StatusOK, view)
}

// IssueForInvite emite (Soporte) el certificado de una capacitación aprobada
// que no lo tiene.
func (h *CertificateHandler) IssueForInvite(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Capacitación inválida"})
		return
	}
	cert, err := h.svc.IssueForInvite(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cert)
}

func (h *CertificateHandler) Reissue(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Certificado inválido"})
		return
	}
	cert, err := h.svc.Reissue(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cert)
}

// --- Público ---

// Verify responde si un código corresponde a un certificado emitido.
func (h *CertificateHandler) Verify(c *gin.Context) {
	view, err := h.svc.Verify(c.Param("code"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo verificar"})
		return
	}
	c.JSON(http.StatusOK, view)
}

// PublicPDF sirve el PDF por su código: quien tiene el código (impreso en el
// propio certificado) puede descargarlo, que es lo que hace verificable el
// documento fuera de Obertrack.
func (h *CertificateHandler) PublicPDF(c *gin.Context) {
	cert, err := h.svc.GetByCode(c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Certificado no encontrado"})
		return
	}
	h.serve(c, cert.Code, h.svc.FilePath(cert))
}
