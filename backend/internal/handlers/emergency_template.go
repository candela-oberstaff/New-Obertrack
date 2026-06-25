package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type EmergencyTemplateHandler struct {
	service service.EmergencyTemplateService
}

func NewEmergencyTemplateHandler(s service.EmergencyTemplateService) *EmergencyTemplateHandler {
	return &EmergencyTemplateHandler{service: s}
}

type emergencyTemplateDTO struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func toEmergencyTemplateDTO(t *models.EmergencyTemplate) emergencyTemplateDTO {
	return emergencyTemplateDTO{
		ID:        t.ID,
		Title:     t.Title,
		Subject:   t.Subject,
		Body:      t.Body,
		CreatedAt: t.CreatedAt,
	}
}

func (h *EmergencyTemplateHandler) guard(c *gin.Context) bool {
	if !middleware.IsSuperadmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin"})
		return false
	}
	return true
}

func (h *EmergencyTemplateHandler) List(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	templates, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates"})
		return
	}
	out := make([]emergencyTemplateDTO, 0, len(templates))
	for i := range templates {
		out = append(out, toEmergencyTemplateDTO(&templates[i]))
	}
	c.JSON(http.StatusOK, gin.H{"templates": out})
}

func (h *EmergencyTemplateHandler) Create(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		Title   string `json:"title"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	template, err := h.service.Create(req.Title, req.Subject, req.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"template": toEmergencyTemplateDTO(template)})
}

func (h *EmergencyTemplateHandler) Update(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	var req struct {
		Title   string `json:"title"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	template, err := h.service.Update(uint(id), req.Title, req.Subject, req.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": toEmergencyTemplateDTO(template)})
}

func (h *EmergencyTemplateHandler) Delete(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	if err := h.service.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
