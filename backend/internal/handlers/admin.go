package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type AdminHandler struct {
	service service.AdminService
}

func NewAdminHandler(s service.AdminService) *AdminHandler {
	return &AdminHandler{service: s}
}

func (h *AdminHandler) GetDashboard(c *gin.Context) {
	metrics, err := h.service.GetDashboardMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard metrics"})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func (h *AdminHandler) GetCompanies(c *gin.Context) {
	companies, err := h.service.GetCompanies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}
	c.JSON(http.StatusOK, companies)
}

func (h *AdminHandler) GetInactiveUsers(c *gin.Context) {
	days := c.DefaultQuery("days", "7")
	daysInt, _ := strconv.Atoi(days)

	users, err := h.service.GetInactiveUsers(daysInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inactive users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *AdminHandler) GetRecentActivity(c *gin.Context) {
	activities, err := h.service.GetRecentActivities()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent activities"})
		return
	}
	c.JSON(http.StatusOK, activities)
}

func (h *AdminHandler) GetAbsenceReport(c *gin.Context) {
	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))

	report, err := h.service.GetAbsenceReport(month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch absence report"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *AdminHandler) GetAllUsers(c *gin.Context) {
	userType := c.Query("user_type")
	isManager := c.Query("is_manager")
	isActive := c.Query("is_active")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	users, total, err := h.service.GetAllUsers(userType, isManager, isActive, offset, limit)
	if err != nil {
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

	payload := map[string]interface{}{
		"name":         req.Name,
		"email":        req.Email,
		"password":     req.Password,
		"user_type":    req.UserType,
		"company_name": req.CompanyName,
		"job_title":    req.JobTitle,
		"is_manager":   req.IsManager,
		"phone_number": req.PhoneNumber,
		"country":      req.Country,
		"city":         req.City,
	}
	if req.EmpleadorID != nil {
		payload["empleador_id"] = *req.EmpleadorID
	}
	if req.ManagerID != nil {
		payload["manager_id"] = *req.ManagerID
	}

	user, err := h.service.CreateUser(payload)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Invalid user type" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
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

	var req struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		JobTitle    string `json:"job_title"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		City        string `json:"city"`
		Location    string `json:"location"`
		CompanyName string `json:"company_name"`
		IsActive    *bool  `json:"is_active"`
		IsManager   *bool  `json:"is_manager"`
		UserType    string `json:"user_type"`
		EmpleadorID *uint  `json:"empleador_id"`
		ManagerID   *uint  `json:"manager_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Guard: an admin must not be able to deactivate their own account. Doing so
	// would lock them out the moment their session ends (login blocks inactive
	// users), with no way to recover if they are the last active superadmin.
	if req.IsActive != nil && !*req.IsActive && uint(id) == middleware.GetUserID(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No puedes desactivar tu propia cuenta."})
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
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.CompanyName != "" {
		updates["company_name"] = req.CompanyName
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsManager != nil {
		updates["is_manager"] = *req.IsManager
	}
	if req.UserType != "" {
		updates["user_type"] = req.UserType
	}
	if req.EmpleadorID != nil {
		updates["empleador_id"] = *req.EmpleadorID
	}
	if req.ManagerID != nil {
		updates["manager_id"] = *req.ManagerID
	}

	user, err := h.service.UpdateUser(uint(id), updates)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
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

	if err := h.service.DeleteUser(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
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

	if err := h.service.ResetPassword(uint(id), req.NewPassword); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *AdminHandler) GetTenants(c *gin.Context) {
	tenants, err := h.service.GetTenants()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenants"})
		return
	}
	c.JSON(http.StatusOK, tenants)
}

func (h *AdminHandler) GetTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	tenant, err := h.service.GetTenant(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

func (h *AdminHandler) GetTenantEmployees(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	employees, err := h.service.GetTenantEmployees(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant employees"})
		return
	}
	c.JSON(http.StatusOK, employees)
}

func (h *AdminHandler) GetEmployeeTracking(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid employee ID"})
		return
	}

	tracking, err := h.service.GetEmployeeTracking(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Employee not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tracking)
}

func (h *AdminHandler) GetTenantActivity(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	activities, err := h.service.GetTenantActivities(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant activity"})
		return
	}
	c.JSON(http.StatusOK, activities)
}

func (h *AdminHandler) SetTenantStatus(c *gin.Context, active bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	tenant, err := h.service.SetTenantStatus(uint(id), active)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Tenant not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

func (h *AdminHandler) SuspendTenant(c *gin.Context) {
	h.SetTenantStatus(c, false)
}

func (h *AdminHandler) ActivateTenant(c *gin.Context) {
	h.SetTenantStatus(c, true)
}

func (h *AdminHandler) CreateTenant(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		CompanyName string `json:"company_name" binding:"required"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		UserID      *uint  `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID != nil {
		tenant, err := h.service.AssignTenant(*req.UserID, req.CompanyName)
		if err != nil {
			status := http.StatusBadRequest
			if err.Error() == "Usuario no encontrado" {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, tenant)
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, email y password son obligatorios para crear una cuenta nueva"})
		return
	}

	tenant, err := h.service.CreateTenant(req.Name, req.CompanyName, req.Email, req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Email already registered" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tenant)
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) CreateSuperAdmin(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.CreateSuperAdmin(req.Name, req.Email, req.Password, false)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Superadmin already exists. Use /api/seed/reset-superadmin to recreate." {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
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

	user, err := h.service.ResetSuperAdmin(req.Name, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create superadmin: " + err.Error()})
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
	user, err := h.service.MakeSuperAdmin(email)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found with email" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
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

	user, err := h.service.CreateSuperAdmin(req.Name, req.Email, req.Password, true)
	if err != nil {
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
