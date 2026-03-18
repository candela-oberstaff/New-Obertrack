package handlers

import (
	"fmt"
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
	WorkDate      string  `json:"work_date" binding:"required"`
	WorkType      string  `json:"work_type" binding:"required"`
	Activities    string  `json:"activities"`
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	Comments      string  `json:"comments"`
	HoursWorked   float64 `json:"hours_worked"`
	AbsenceReason string  `json:"absence_reason"`
	AbsenceHours  float64 `json:"absence_hours"`
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

	// Si es superadmin, ve todo
	// Si es empresa (empleador), ve las horas de sus empleados
	if isSuperadmin {
		// Ve todo
	} else if role == string(models.UserTypeEmployer) || role == "empleador" {
		subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", userID).Select("id")
		query = query.Where("user_id IN (?)", subquery)
	} else if role == string(models.UserTypeProfessional) || role == "profesional" {
		// Si es profesional, solo ve SUS PROPIAS horas
		query = query.Where("user_id = ?", userID)
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

	// Validar que no sea fecha futura
	today := time.Now().Truncate(24 * time.Hour)
	if workDate.After(today) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No puedes registrar horas en fechas futuras"})
		return
	}

	var existingWorkHour models.WorkHour
	if err := h.db.Where("user_id = ? AND work_date = ?", userID, workDate).First(&existingWorkHour).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Ya existe un registro para esta fecha. Solo puedes registrar un máximo de una jornada por día."})
		return
	}

	// Si el frontend envía las horas calculadas, usarlas; si no, calcularlas
	hoursWorked := req.HoursWorked
	workType := models.WorkTypeComplete

	if req.WorkType == "absence" {
		workType = models.WorkTypeAbsence
		// Si no se envió hours_worked, calcular: 8 - absence_hours
		if hoursWorked == 0 {
			hoursWorked = 8 - req.AbsenceHours
			if hoursWorked < 0 {
				hoursWorked = 0
			}
		}
	} else if hoursWorked == 0 {
		// Si es jornada completa y no se enviaron horas, usar 8
		hoursWorked = 8
	}

	workHour := models.WorkHour{
		UserID:        userID,
		WorkDate:      workDate,
		WorkType:      workType,
		HoursWorked:   hoursWorked,
		Activities:    req.Activities,
		Comments:      req.Comments,
		AbsenceReason: req.AbsenceReason,
		AbsenceHours:  req.AbsenceHours,
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

	var totalHours float64
	var approvedHours float64

	if isSuperadmin {
		h.db.Model(&models.WorkHour{}).Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours)
		h.db.Model(&models.WorkHour{}).Where("approved = true").Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours)
	} else if role == string(models.UserTypeEmployer) || role == "empleador" {
		subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", userID).Select("id")
		h.db.Model(&models.WorkHour{}).Where("user_id IN (?)", subquery).Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours)
		h.db.Model(&models.WorkHour{}).Where("user_id IN (?) AND approved = true", subquery).Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours)
	} else {
		h.db.Model(&models.WorkHour{}).Where("user_id = ?", userID).Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours)
		h.db.Model(&models.WorkHour{}).Where("user_id = ? AND approved = true", userID).Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours)
	}

	fmt.Printf("[DEBUG] GetSummary - userID: %d, role: %s, totalHours: %f, approvedHours: %f\n", userID, role, totalHours, approvedHours)

	c.JSON(http.StatusOK, gin.H{
		"total_hours":    totalHours,
		"approved_hours": approvedHours,
		"pending_hours":  totalHours - approvedHours,
	})
}

func (h *WorkHourHandler) GetPending(c *gin.Context) {
	empleadorID := middleware.GetEmpleadorID(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	// Si es superadmin, puede ver todas las horas pendientes
	if isSuperadmin {
		var workHours []models.WorkHour
		query := h.db.Model(&models.WorkHour{}).Where("approved = false")
		if err := query.Preload("User").Order("work_date DESC").Find(&workHours).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending work hours"})
			return
		}
		c.JSON(http.StatusOK, workHours)
		return
	}

	// Si es empresa (empleador), usa su propio ID
	if role == "empresa" || role == "empleador" {
		empleadorID = userID
	}

	if empleadorID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only employers can access this resource"})
		return
	}

	var workHours []models.WorkHour
	query := h.db.Model(&models.WorkHour{}).Where("approved = false")

	subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
	query = query.Where("user_id IN (?)", subquery)

	if err := query.Preload("User").Order("work_date DESC").Find(&workHours).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending work hours"})
		return
	}

	c.JSON(http.StatusOK, workHours)
}
