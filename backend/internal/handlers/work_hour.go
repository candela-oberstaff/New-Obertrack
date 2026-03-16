package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

type WorkHourHandler struct {
	db *gorm.DB
}

func NewWorkHourHandler(db *gorm.DB) *WorkHourHandler {
	return &WorkHourHandler{db: db}
}

type CreateWorkHourRequest struct {
	WorkDate   string `json:"work_date" binding:"required"`
	WorkType   string `json:"work_type" binding:"required"`
	Activities string `json:"activities"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Comments   string `json:"comments"`
}

type UpdateWorkHourRequest struct {
	WorkDate   string `json:"work_date"`
	WorkType   string `json:"work_type"`
	Activities string `json:"activities"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Comments   string `json:"comments"`
}

func (h *WorkHourHandler) GetAll(c *gin.Context) {
	var workHours []models.WorkHour
	query := h.db.Model(&models.WorkHour{})

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := middleware.GetEmpleadorID(c)

	// Scope por empresa
	if role == string(models.UserTypeEmployee) && !isSuperadmin {
		if empleadorID > 0 {
			subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
			query = query.Where("user_id IN (?) OR user_id = ?", subquery, userID)
		} else {
			query = query.Where("user_id = ?", userID)
		}
	}

	userIDFilter := c.Query("user_id")
	if userIDFilter != "" && (isSuperadmin || role == string(models.UserTypeEmployer)) {
		query = query.Where("user_id = ?", userIDFilter)
	}

	startDate := c.Query("start_date")
	if startDate != "" {
		t, _ := time.Parse("2006-01-02", startDate)
		query = query.Where("work_date >= ?", t)
	}

	endDate := c.Query("end_date")
	if endDate != "" {
		t, _ := time.Parse("2006-01-02", endDate)
		query = query.Where("work_date <= ?", t)
	}

	var total int64
	query.Count(&total)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	if err := query.Preload("User").Preload("ApprovedByUser").Offset(offset).Limit(limit).Order("work_date DESC").Find(&workHours).Error; err != nil {
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
	var req CreateWorkHourRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	workDate, err := time.Parse("2006-01-02", req.WorkDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
		return
	}

	var existingWorkHour models.WorkHour
	if err := h.db.Where("user_id = ? AND work_date = ?", userID, workDate).First(&existingWorkHour).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Ya existe un registro para esta fecha. Solo puedes registrar un máximo de una jornada por día."})
		return
	}

	hoursWorked := 8.0
	workType := models.WorkTypeComplete
	if req.WorkType == "absence" {
		hoursWorked = 0
		workType = models.WorkTypeAbsence
	}

	workHour := models.WorkHour{
		UserID:      userID,
		WorkDate:    workDate,
		WorkType:    workType,
		HoursWorked: hoursWorked,
		Activities:  req.Activities,
		Comments:    req.Comments,
	}

	if req.StartTime != "" {
		if t, err := time.Parse("15:04", req.StartTime); err == nil {
			workHour.StartTime = &t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse("15:04", req.EndTime); err == nil {
			workHour.EndTime = &t
		}
	}

	if err := h.db.Create(&workHour).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create work hour"})
		return
	}

	h.db.Preload("User").First(&workHour, workHour.ID)

	c.JSON(http.StatusCreated, workHour)
}

func (h *WorkHourHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid work hour ID"})
		return
	}

	var workHour models.WorkHour
	if err := h.db.First(&workHour, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work hour not found"})
		return
	}

	var req UpdateWorkHourRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.WorkDate != "" {
		if t, err := time.Parse("2006-01-02", req.WorkDate); err == nil {
			updates["work_date"] = t
		}
	}
	if req.WorkType != "" {
		updates["work_type"] = req.WorkType
		if req.WorkType == "absence" {
			updates["hours_worked"] = 0
		} else {
			updates["hours_worked"] = 8
		}
	}
	if req.Activities != "" {
		updates["activities"] = req.Activities
	}
	if req.Comments != "" {
		updates["comments"] = req.Comments
	}

	if err := h.db.Model(&workHour).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update work hour"})
		return
	}

	h.db.Preload("User").First(&workHour, workHour.ID)

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
	now := time.Now()

	if err := h.db.Model(&models.WorkHour{}).
		Where("id IN ?", req.IDs).
		Updates(map[string]interface{}{
			"approved":    true,
			"approved_by": userID,
			"approved_at": now,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve work hours"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Work hours approved"})
}

func (h *WorkHourHandler) GetSummary(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := c.GetUint("empleador_id")

	query := h.db.Model(&models.WorkHour{})

	// Scope por empresa
	if role == string(models.UserTypeEmployee) && !isSuperadmin {
		if empleadorID > 0 {
			subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
			query = query.Where("user_id IN (?) OR user_id = ?", subquery, userID)
		} else {
			query = query.Where("user_id = ?", userID)
		}
	}

	var totalHours float64
	var approvedHours float64

	query.Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours)
	query.Where("approved = true").Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours)

	c.JSON(http.StatusOK, gin.H{
		"total_hours":    totalHours,
		"approved_hours": approvedHours,
		"pending_hours":  totalHours - approvedHours,
	})
}

func (h *WorkHourHandler) GetPending(c *gin.Context) {
	empleadorID := middleware.GetEmpleadorID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	if empleadorID == 0 && !isSuperadmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only employers can access this resource"})
		return
	}

	var workHours []models.WorkHour
	query := h.db.Model(&models.WorkHour{}).Where("approved = false")

	if !isSuperadmin && empleadorID > 0 {
		subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
		query = query.Where("user_id IN (?)", subquery)
	}

	if err := query.Preload("User").Order("work_date DESC").Find(&workHours).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending work hours"})
		return
	}

	c.JSON(http.StatusOK, workHours)
}
