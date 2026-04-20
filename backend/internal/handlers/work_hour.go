package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type WorkHourHandler struct {
	svc service.WorkHourService
}

func NewWorkHourHandler(svc service.WorkHourService) *WorkHourHandler {
	return &WorkHourHandler{svc: svc}
}

func (h *WorkHourHandler) GetAll(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	userIDFilter := c.Query("user_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	workHours, total, err := h.svc.GetAll(userID, role, isSuperadmin, userIDFilter, startDate, endDate, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch work hours"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  workHours,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *WorkHourHandler) Create(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	workHour, err := h.svc.Create(userID, req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Invalid date format" || err.Error() == "No puedes registrar horas en fechas futuras" {
			status = http.StatusBadRequest
		} else if err.Error() == "Ya existe un registro para esta fecha. Solo puedes registrar un máximo de una jornada por día." {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, workHour)
}

func (h *WorkHourHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid work hour ID"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workHour, err := h.svc.Update(uint(id), req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Work hour not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, workHour)
}

func (h *WorkHourHandler) Approve(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)

	err := h.svc.Approve(req.IDs, userID, role, isSuperadmin, isManager)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "No work hours found" {
			status = http.StatusBadRequest
		} else if err.Error() == "Not authorized to approve work hours for user" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Work hours approved"})
}

func (h *WorkHourHandler) GetSummary(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	summary, err := h.svc.GetSummary(userID, role, isSuperadmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *WorkHourHandler) GetPending(c *gin.Context) {
	empleadorID := middleware.GetEmpleadorID(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	pending, err := h.svc.GetPending(empleadorID, userID, role, isSuperadmin)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Only employers can access this resource" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pending)
}
