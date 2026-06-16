package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type UploadHandler struct {
	svc           service.UploadService
	uploadPath    string
	employmentSvc service.EmploymentService
}

func NewUploadHandler(svc service.UploadService, uploadPath string, employmentSvc service.EmploymentService) *UploadHandler {
	if uploadPath == "" {
		uploadPath = "./uploads"
	}
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		log.Printf("failed to create upload directory %q: %v", uploadPath, err)
	}
	return &UploadHandler{
		svc:           svc,
		uploadPath:    uploadPath,
		employmentSvc: employmentSvc,
	}
}

func (h *UploadHandler) UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	contentType := file.Header.Get("Content-Type")
	ext, err := h.svc.ValidateFile(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("%d_%d_%s%s", userID, file.Size, sanitizeFilename(file.Filename), ext)
	filePath := filepath.Join(h.uploadPath, filename)

	if err := os.MkdirAll(h.uploadPath, 0755); err != nil {
		log.Printf("failed to create upload directory %q: %v", h.uploadPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare upload directory"})
		return
	}

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		log.Printf("failed to save upload %q: %v", filePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	fileURL := fmt.Sprintf("/api/uploads/%s", filename)

	c.JSON(http.StatusOK, gin.H{
		"url":      fileURL,
		"filename": filename,
		"size":     file.Size,
		"type":     contentType,
	})
}

func (h *UploadHandler) GetFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
		return
	}
	// Reject path traversal: a valid upload name never contains a path separator or "..".
	if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") ||
		filename != filepath.Base(filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
		return
	}
	// Any authenticated tenant member may download.
	// Authentication is already enforced by the JWT middleware on the /api group.
	_ = middleware.GetUserID(c) // ensures the user is authenticated

	filePath := filepath.Join(h.uploadPath, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

// DownloadExpedienteDoc sirve un documento del expediente para la audiencia
// empresa (RR.HH. ve todo). La autorización de ruta (superadmin/CS) ya se aplica
// por el grupo /admin.
func (h *UploadHandler) DownloadExpedienteDoc(c *gin.Context) {
	h.downloadDoc(c, service.AudienceCompany)
}

// DownloadMyExpedienteDoc sirve un documento del propio expediente del
// profesional: solo los marcados como compartidos y de su propio empleo.
func (h *UploadHandler) DownloadMyExpedienteDoc(c *gin.Context) {
	h.downloadDoc(c, service.AudienceProfessional)
}

func (h *UploadHandler) downloadDoc(c *gin.Context, audience string) {
	docID, err := strconv.ParseUint(c.Param("docId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Documento inválido"})
		return
	}
	doc, err := h.employmentSvc.DocumentForDownload(uint(docID), audience, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	// doc.FileName es el nombre en disco; nunca confiamos en él para la ruta.
	filename := filepath.Base(doc.FileName)
	if filename == "" || filename == "." || strings.ContainsAny(filename, "/\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Archivo inválido"})
		return
	}
	filePath := filepath.Join(h.uploadPath, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Archivo no encontrado"})
		return
	}
	c.File(filePath)
}

func sanitizeFilename(name string) string {
	result := ""
	for i, r := range name {
		if i > 50 {
			break
		}
		if r != '.' && r != '/' && r != '\\' {
			result += string(r)
		}
	}
	return result
}
