package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type IncidentHandler struct {
	service service.IncidentService
}

func NewIncidentHandler(s service.IncidentService) *IncidentHandler {
	return &IncidentHandler{service: s}
}

func (h *IncidentHandler) guard(c *gin.Context) bool {
	if !middleware.IsSuperadmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin"})
		return false
	}
	return true
}

func (h *IncidentHandler) List(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	incidents, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch incidents"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"incidents": incidents})
}

func (h *IncidentHandler) Create(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Kind        string `json:"kind"`
		Country     string `json:"country"`
		State       string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	incident, err := h.service.Create(req.Title, req.Description, req.Kind, req.Country, req.State, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"incident": incident})
}

func (h *IncidentHandler) Get(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	incident, professionals, err := h.service.Detail(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incidente no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"incident": incident, "professionals": professionals})
}

func (h *IncidentHandler) Close(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	incident, err := h.service.Close(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incidente no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"incident": incident})
}

func (h *IncidentHandler) Broadcast(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	var req struct {
		Subject string `json:"subject" binding:"required"`
		Body    string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.Broadcast(uint(id), req.Subject, req.Body)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Incidente no encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sent": result.Sent, "failed": result.Failed})
}

func (h *IncidentHandler) UpsertResponse(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	userID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId inválido"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.UpsertResponse(uint(id), uint(userID), req.Status, req.Note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
