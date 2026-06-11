package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TutorialHandler struct {
	service service.TutorialService
}

func NewTutorialHandler(service service.TutorialService) *TutorialHandler {
	return &TutorialHandler{service: service}
}

type CreateTutorialRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	GoogleDriveURL string `json:"google_drive_url"`
	IconName       string `json:"icon_name"`
	Category       string `json:"category"`
	Audience       string `json:"audience"`
	DurationMin    int    `json:"duration_min"`
	OrderIndex     int    `json:"order_index"`
	IsActive       *bool  `json:"is_active"`
}

type UpdateTutorialRequest struct {
	Title          *string `json:"title"`
	Description    *string `json:"description"`
	GoogleDriveURL *string `json:"google_drive_url"`
	IconName       *string `json:"icon_name"`
	Category       *string `json:"category"`
	Audience       *string `json:"audience"`
	DurationMin    *int    `json:"duration_min"`
	OrderIndex     *int    `json:"order_index"`
	IsActive       *bool   `json:"is_active"`
}

type ReorderTutorialsRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// audienceForRequest maps the authenticated user's type to the tutorial audience
// they're allowed to see. Empty string means no filter: superadmins and platform
// staff (customer_success) see tutorials for every audience.
func audienceForRequest(c *gin.Context) string {
	if middleware.IsSuperadmin(c) {
		return ""
	}
	switch middleware.GetUserRole(c) {
	case string(models.UserTypeEmployer):
		return models.TutorialAudienceEmployer
	case string(models.UserTypeProfessional):
		return models.TutorialAudienceProfessional
	}
	return ""
}

func (h *TutorialHandler) GetAll(c *gin.Context) {
	onlyActive := !middleware.IsSuperadmin(c)

	tutorials, err := h.service.GetAll(onlyActive, audienceForRequest(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tutorials", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tutorials})
}

func (h *TutorialHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	tutorial, err := h.service.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tutorial no encontrado"})
		return
	}

	if audience := audienceForRequest(c); audience != "" &&
		tutorial.Audience != models.TutorialAudienceAll && tutorial.Audience != audience {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tutorial no encontrado"})
		return
	}

	c.JSON(http.StatusOK, tutorial)
}

func (h *TutorialHandler) Create(c *gin.Context) {
	var req CreateTutorialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	tutorial, err := h.service.Create(userID, req.Title, req.Description, req.GoogleDriveURL, req.IconName, req.Category, req.Audience, req.DurationMin, req.OrderIndex, isActive)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tutorial)
}

func (h *TutorialHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	var req UpdateTutorialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.GoogleDriveURL != nil {
		updates["google_drive_url"] = *req.GoogleDriveURL
	}
	if req.IconName != nil {
		updates["icon_name"] = *req.IconName
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Audience != nil {
		updates["audience"] = *req.Audience
	}
	if req.DurationMin != nil {
		updates["duration_min"] = *req.DurationMin
	}
	if req.OrderIndex != nil {
		updates["order_index"] = *req.OrderIndex
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	tutorial, err := h.service.Update(uint(id), updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tutorial)
}

func (h *TutorialHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tutorial"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tutorial eliminado"})
}

func (h *TutorialHandler) Reorder(c *gin.Context) {
	var req ReorderTutorialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Reorder(req.IDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Orden actualizado"})
}

func (h *TutorialHandler) RecordView(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.service.RecordView(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vista registrada"})
}

func (h *TutorialHandler) GetMyViews(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ids, err := h.service.GetUserViewedIDs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ids == nil {
		ids = []uint{}
	}
	c.JSON(http.StatusOK, gin.H{"data": ids})
}
