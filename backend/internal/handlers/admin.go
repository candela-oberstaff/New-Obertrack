package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
