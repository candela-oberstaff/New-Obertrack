package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
)

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

type DashboardMetrics struct {
	TotalCompanies     int     `json:"total_companies"`
	TotalProfessionals int     `json:"total_professionals"`
	TotalManagers      int     `json:"total_managers"`
	TotalHoursWorked   float64 `json:"total_hours_worked"`
	ApprovedHours      float64 `json:"approved_hours"`
	PendingHours       float64 `json:"pending_hours"`
	TotalTasks         int     `json:"total_tasks"`
	CompletedTasks     int     `json:"completed_tasks"`
	PendingTasks       int     `json:"pending_tasks"`
	ActiveToday        int     `json:"active_today"`
	InactiveWarning    int     `json:"inactive_warning"`
}

type CompanyMetric struct {
	ID             uint    `json:"id"`
	Name           string  `json:"name"`
	Professionals  int     `json:"professionals"`
	HoursThisMonth float64 `json:"hours_this_month"`
	TasksCompleted int     `json:"tasks_completed"`
	ActiveUsers    int     `json:"active_users"`
}

type InactiveUser struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Company      string    `json:"company"`
	LastActive   time.Time `json:"last_active"`
	DaysInactive int       `json:"days_inactive"`
}

func (h *AdminHandler) GetDashboard(c *gin.Context) {
	var metrics DashboardMetrics

	var totalCompanies, totalProfessionals, totalManagers, totalTasks, completedTasks, activeToday, inactiveWarning int64

	h.db.Model(&models.User{}).Where("user_type = ?", "empleador").Count(&totalCompanies)
	metrics.TotalCompanies = int(totalCompanies)

	h.db.Model(&models.User{}).Where("user_type = ?", "profesional").Count(&totalProfessionals)
	metrics.TotalProfessionals = int(totalProfessionals)

	h.db.Model(&models.User{}).Where("is_manager = ?", true).Count(&totalManagers)
	metrics.TotalManagers = int(totalManagers)

	var totalHours float64
	h.db.Model(&models.WorkHour{}).Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours)
	metrics.TotalHoursWorked = totalHours

	var approvedHours float64
	h.db.Model(&models.WorkHour{}).Where("approved = ?", true).Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours)
	metrics.ApprovedHours = approvedHours
	metrics.PendingHours = totalHours - approvedHours

	h.db.Model(&models.Task{}).Count(&totalTasks)
	metrics.TotalTasks = int(totalTasks)

	h.db.Model(&models.Task{}).Where("completed = ?", true).Count(&completedTasks)
	metrics.CompletedTasks = int(completedTasks)
	metrics.PendingTasks = metrics.TotalTasks - metrics.CompletedTasks

	today := time.Now().Truncate(24 * time.Hour)
	h.db.Model(&models.WorkHour{}).Where("work_date >= ?", today).Distinct("user_id").Count(&activeToday)
	metrics.ActiveToday = int(activeToday)

	threeDaysAgo := time.Now().AddDate(0, 0, -3)
	h.db.Model(&models.User{}).
		Where("user_type = ? AND id NOT IN (SELECT DISTINCT user_id FROM work_hours WHERE work_date >= ?)", "profesional", threeDaysAgo).
		Count(&inactiveWarning)
	metrics.InactiveWarning = int(inactiveWarning)

	c.JSON(http.StatusOK, metrics)
}

func (h *AdminHandler) GetCompanies(c *gin.Context) {
	var companies []CompanyMetric

	rows, err := h.db.Raw(`
		SELECT 
			u.id,
			u.company_name as name,
			COUNT(DISTINCT p.id) as professionals,
			COALESCE(SUM(wh.hours_worked), 0) as hours_this_month,
			COUNT(DISTINCT CASE WHEN t.completed = true THEN t.id END) as tasks_completed,
			COUNT(DISTINCT CASE WHEN wh.work_date >= CURRENT_DATE - INTERVAL '7 days' THEN wh.user_id END) as active_users
		FROM users u
		LEFT JOIN users p ON p.empleador_id = u.id AND p.user_type = 'profesional'
		LEFT JOIN work_hours wh ON wh.user_id = p.id AND wh.work_date >= date_trunc('month', CURRENT_DATE)
		LEFT JOIN tasks t ON t.created_by = p.id AND t.completed = true
		WHERE u.user_type = 'empleador'
		GROUP BY u.id, u.company_name
		ORDER BY hours_this_month DESC
	`).Rows()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var cm CompanyMetric
		var companyName interface{}
		rows.Scan(&cm.ID, &companyName, &cm.Professionals, &cm.HoursThisMonth, &cm.TasksCompleted, &cm.ActiveUsers)
		switch v := companyName.(type) {
		case []byte:
			cm.Name = string(v)
		case string:
			cm.Name = v
		default:
			cm.Name = ""
		}
		companies = append(companies, cm)
	}

	c.JSON(http.StatusOK, companies)
}

func (h *AdminHandler) GetInactiveUsers(c *gin.Context) {
	days := c.DefaultQuery("days", "7")
	daysInt, _ := strconv.Atoi(days)

	since := time.Now().AddDate(0, 0, -daysInt)

	var users []InactiveUser

	h.db.Raw(`
		SELECT 
			u.id,
			u.name,
			u.email,
			COALESCE(e.company_name, '-') as company,
			COALESCE(MAX(wh.work_date), u.created_at) as last_active,
			EXTRACT(DAY FROM CURRENT_DATE - COALESCE(MAX(wh.work_date), u.created_at)) as days_inactive
		FROM users u
		LEFT JOIN work_hours wh ON wh.user_id = u.id
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE u.user_type = 'profesional'
		GROUP BY u.id, u.name, u.email, e.company_name
		HAVING MAX(wh.work_date) IS NULL OR MAX(wh.work_date) < ?
		ORDER BY days_inactive DESC
		LIMIT 50
	`, since).Scan(&users)

	c.JSON(http.StatusOK, users)
}

func (h *AdminHandler) GetRecentActivity(c *gin.Context) {
	type Activity struct {
		Type      string    `json:"type"`
		User      string    `json:"user"`
		Company   string    `json:"company"`
		Details   string    `json:"details"`
		Timestamp time.Time `json:"timestamp"`
	}

	var activities []Activity

	h.db.Raw(`
		SELECT 
			'work_hour' as type,
			u.name as user,
			COALESCE(e.company_name, '-') as company,
			CASE 
				WHEN wh.work_type = 'complete' THEN 'Registró jornada completa'
				ELSE 'Registró ausencia'
			END as details,
			wh.created_at as timestamp
		FROM work_hours wh
		JOIN users u ON u.id = wh.user_id
		LEFT JOIN users e ON e.id = u.empleador_id
		ORDER BY wh.created_at DESC
		LIMIT 20
	`).Scan(&activities)

	c.JSON(http.StatusOK, activities)
}

func (h *AdminHandler) GetAllUsers(c *gin.Context) {
	var users []models.User
	query := h.db.Model(&models.User{})

	userType := c.Query("user_type")
	if userType != "" {
		query = query.Where("user_type = ?", userType)
	}

	isManager := c.Query("is_manager")
	if isManager != "" {
		query = query.Where("is_manager = ?", isManager == "true")
	}

	isActive := c.Query("is_active")
	if isActive != "" {
		query = query.Where("is_active = ?", isActive == "true")
	}

	var total int64
	query.Count(&total)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Email       string `json:"email" binding:"required,email"`
		Password    string `json:"password" binding:"required"`
		UserType    string `json:"user_type" binding:"required"`
		CompanyName string `json:"company_name"`
		JobTitle    string `json:"job_title"`
		EmpleadorID *uint  `json:"empleador_id"`
		ManagerID   *uint  `json:"manager_id"`
		IsManager   bool   `json:"is_manager"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		City        string `json:"city"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserType != "empleador" && req.UserType != "profesional" && req.UserType != "superadmin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user type"})
		return
	}

	user := models.User{
		Name:        req.Name,
		Email:       req.Email,
		Password:    req.Password,
		UserType:    models.UserType(req.UserType),
		CompanyName: req.CompanyName,
		JobTitle:    req.JobTitle,
		IsManager:   req.IsManager,
		PhoneNumber: req.PhoneNumber,
		Country:     req.Country,
		City:        req.City,
	}

	if req.EmpleadorID != nil {
		user.EmpleadorID = req.EmpleadorID
	}

	if req.ManagerID != nil {
		user.ManagerID = req.ManagerID
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = string(hashedPassword)

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		JobTitle    string `json:"job_title"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		City        string `json:"city"`
		IsActive    *bool  `json:"is_active"`
		IsManager   *bool  `json:"is_manager"`
		EmpleadorID *uint  `json:"empleador_id"`
		ManagerID   *uint  `json:"manager_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.JobTitle != "" {
		updates["job_title"] = req.JobTitle
	}
	if req.PhoneNumber != "" {
		updates["phone_number"] = req.PhoneNumber
	}
	if req.Country != "" {
		updates["country"] = req.Country
	}
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsManager != nil {
		updates["is_manager"] = *req.IsManager
	}
	if req.EmpleadorID != nil {
		updates["empleador_id"] = *req.EmpleadorID
	}
	if req.ManagerID != nil {
		updates["manager_id"] = *req.ManagerID
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	tx := h.db.Begin()

	if err := tx.Where("user_id = ?", id).Delete(&models.WorkHour{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user work hours"})
		return
	}

	if err := tx.Where("approved_by = ?", id).Update("approved_by", nil).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update approved work hours"})
		return
	}

	if err := tx.Unscoped().Delete(&models.User{}, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Resetting password for user ID: %d", id)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	if err := h.db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		log.Printf("Error updating password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	type Stats struct {
		TotalUsers      int64   `json:"total_users"`
		ActiveUsers     int64   `json:"active_users"`
		TotalCompanies  int64   `json:"total_companies"`
		TotalHoursMonth float64 `json:"total_hours_month"`
		CompletionRate  float64 `json:"completion_rate"`
	}

	var stats Stats

	h.db.Model(&models.User{}).Count(&stats.TotalUsers)
	h.db.Model(&models.User{}).Where("is_active = ?", true).Count(&stats.ActiveUsers)
	h.db.Model(&models.User{}).Where("user_type = ?", "empleador").Count(&stats.TotalCompanies)

	h.db.Model(&models.WorkHour{}).
		Where("work_date >= date_trunc('month', CURRENT_DATE)").
		Select("COALESCE(SUM(hours_worked), 0)").Scan(&stats.TotalHoursMonth)

	var total, completed int64
	h.db.Model(&models.Task{}).Count(&total)
	h.db.Model(&models.Task{}).Where("completed = ?", true).Count(&completed)
	if total > 0 {
		stats.CompletionRate = float64(completed) / float64(total) * 100
	}

	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) CreateSuperAdmin(c *gin.Context) {
	var count int64
	h.db.Model(&models.User{}).Where("user_type = ?", "superadmin").Count(&count)
	if count > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Superadmin already exists. Use /api/seed/reset-superadmin to recreate."})
		return
	}

	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	log.Printf("Creating superadmin with email: %s", req.Email)

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		Password:     string(hashedPassword),
		UserType:     models.UserType("superadmin"),
		IsSuperadmin: true,
	}

	log.Printf("Creating superadmin: %+v", user)

	if err := h.db.Create(&user).Error; err != nil {
		log.Printf("ERROR creating superadmin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create superadmin: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Superadmin created successfully",
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"user_type": user.UserType,
		},
	})
}

func (h *AdminHandler) ResetSuperAdmin(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Where("user_type = ?", "superadmin").Delete(&models.User{})

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		Password:     string(hashedPassword),
		UserType:     models.UserType("superadmin"),
		IsSuperadmin: true,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create superadmin"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Superadmin reset successfully",
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"user_type": user.UserType,
		},
	})
}

func (h *AdminHandler) MakeSuperAdmin(c *gin.Context) {
	email := c.Param("email")

	var user models.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found with email: " + email})
		return
	}

	user.IsSuperadmin = true
	user.UserType = models.UserType("superadmin")

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User is now a superadmin",
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"user_type":     user.UserType,
			"is_superadmin": user.IsSuperadmin,
		},
	})
}

func (h *AdminHandler) CreateSuperAdminForced(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		Password:     string(hashedPassword),
		UserType:     models.UserType("superadmin"),
		IsSuperadmin: true,
		IsActive:     true,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create superadmin: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Superadmin created successfully",
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"user_type": user.UserType,
		},
	})
}
