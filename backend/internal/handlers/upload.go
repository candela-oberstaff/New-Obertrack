package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type UploadHandler struct {
	svc       service.UploadService
	uploadPath string
}

func NewUploadHandler(svc service.UploadService, uploadPath string) *UploadHandler {
	if uploadPath == "" {
		uploadPath = "./uploads"
	}
	os.MkdirAll(uploadPath, 0755)
	return &UploadHandler{
		svc:        svc,
		uploadPath: uploadPath,
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

	if err := c.SaveUploadedFile(file, filePath); err != nil {
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
