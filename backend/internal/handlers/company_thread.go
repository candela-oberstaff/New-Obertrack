package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

// CompanyThreadHandler expone el hilo del expediente de empresa: comentarios y
// archivos colgados de cada entrada.
type CompanyThreadHandler struct {
	svc        service.CompanyThreadService
	uploadPath string
}

func NewCompanyThreadHandler(svc service.CompanyThreadService, uploadPath string) *CompanyThreadHandler {
	return &CompanyThreadHandler{svc: svc, uploadPath: uploadPath}
}

// tenantAndEvent saca empresa y entrada de la ruta. Devuelve ok=false cuando ya
// se escribió la respuesta de error.
func tenantAndEvent(c *gin.Context) (tenantID, eventID uint, ok bool) {
	t, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return 0, 0, false
	}
	e, err := strconv.ParseUint(c.Param("eventId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return 0, 0, false
	}
	return uint(t), uint(e), true
}

func tenantAndParam(c *gin.Context, param, label string) (tenantID, other uint, ok bool) {
	t, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return 0, 0, false
	}
	o, err := strconv.ParseUint(c.Param(param), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid " + label + " ID"})
		return 0, 0, false
	}
	return uint(t), uint(o), true
}

// threadError traduce el error del servicio al código correcto. Un evento de
// otra empresa responde 404, igual que uno inexistente: decir "existe pero no
// es tuyo" ya es contar algo.
func threadError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, service.ErrEventNotInCompany):
		status = http.StatusNotFound
	case err.Error() == "Comentario no encontrado", err.Error() == "Archivo no encontrado":
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func (h *CompanyThreadHandler) AddComment(c *gin.Context) {
	tenantID, eventID, ok := tenantAndEvent(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	comment, err := h.svc.AddComment(tenantID, eventID, middleware.GetUserID(c), req.Content)
	if err != nil {
		threadError(c, err)
		return
	}
	c.JSON(http.StatusCreated, comment)
}

func (h *CompanyThreadHandler) UpdateComment(c *gin.Context) {
	tenantID, commentID, ok := tenantAndParam(c, "commentId", "comment")
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.UpdateComment(tenantID, commentID, req.Content); err != nil {
		threadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Comentario actualizado"})
}

func (h *CompanyThreadHandler) DeleteComment(c *gin.Context) {
	tenantID, commentID, ok := tenantAndParam(c, "commentId", "comment")
	if !ok {
		return
	}
	if err := h.svc.DeleteComment(tenantID, commentID); err != nil {
		threadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Comentario eliminado"})
}

// AddAttachment registra un archivo ya subido por /api/uploads. El cuerpo trae
// el nombre con el que quedó en disco (`stored_name`, el campo `filename` que
// devolvió la subida) y el original para enseñarlo.
func (h *CompanyThreadHandler) AddAttachment(c *gin.Context) {
	tenantID, eventID, ok := tenantAndEvent(c)
	if !ok {
		return
	}
	var req struct {
		StoredName string `json:"stored_name"`
		FileName   string `json:"file_name"`
		FileSize   int64  `json:"file_size"`
		MimeType   string `json:"mime_type"`
		CommentID  *uint  `json:"comment_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	att, err := h.svc.AddAttachment(tenantID, eventID, middleware.GetUserID(c), req.CommentID,
		req.FileName, req.StoredName, req.FileSize, req.MimeType)
	if err != nil {
		threadError(c, err)
		return
	}
	c.JSON(http.StatusCreated, att)
}

func (h *CompanyThreadHandler) DeleteAttachment(c *gin.Context) {
	tenantID, attID, ok := tenantAndParam(c, "attId", "attachment")
	if !ok {
		return
	}
	if err := h.svc.DeleteAttachment(tenantID, attID); err != nil {
		threadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Archivo eliminado"})
}

// DownloadAttachment sirve el archivo comprobando antes que pertenece a esta
// empresa.
//
// Existe en vez de reutilizar /api/uploads/:filename porque esa ruta solo pide
// estar autenticado: cualquier profesional de cualquier empresa que acierte el
// nombre se lleva el archivo. Aquí hay capturas de incidencias y documentos
// internos que no puede ver ni la propia empresa, así que la autorización tiene
// que estar en la ruta y no en lo difícil que sea adivinar un nombre.
func (h *CompanyThreadHandler) DownloadAttachment(c *gin.Context) {
	tenantID, attID, ok := tenantAndParam(c, "attId", "attachment")
	if !ok {
		return
	}
	att, err := h.svc.AttachmentForDownload(tenantID, attID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Archivo no encontrado"})
		return
	}

	// filepath.Base otra vez aunque el servicio ya lo aplicara al guardar: es la
	// última línea antes de tocar el disco y no cuesta nada.
	path := filepath.Join(h.uploadPath, filepath.Base(att.StoredName))
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "El archivo ya no está disponible"})
		return
	}

	// Las imágenes se muestran en línea (van como miniatura en el expediente);
	// el resto se descarga con su nombre original.
	if att.IsImage() {
		c.Header("Content-Disposition", "inline")
		if att.MimeType != "" {
			c.Header("Content-Type", att.MimeType)
		}
		c.File(path)
		return
	}
	c.FileAttachment(path, att.FileName)
}
