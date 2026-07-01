package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type UserHandler struct {
	service service.UserService
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func NewUserHandler(s service.UserService) *UserHandler {
	return &UserHandler{service: s}
}

type UpdateUserRequest struct {
	Name             string `json:"name"`
	Email            string `json:"email"`
	Avatar           string `json:"avatar"`
	JobTitle         string `json:"job_title"`
	PhoneNumber      string `json:"phone_number"`
	Country          string `json:"country"`
	State            string `json:"state"`
	City             string `json:"city"`
	Location         string `json:"location"`
	IdentityDocument string `json:"identity_document"`
}

func (h *UserHandler) GetAll(c *gin.Context) {
	role := c.Query("role")
	isManager := c.Query("is_manager")
	search := c.Query("q")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var companyID uint
	if middleware.IsSuperadmin(c) {
		// Superadmin may scope the search to a company via ?company_id=.
		if v, err := strconv.ParseUint(c.Query("company_id"), 10, 32); err == nil {
			companyID = uint(v)
		}
	} else {
		companyID = middleware.GetTenantID(c)
	}

	users, total, err := h.service.GetAll(role, isManager, search, companyID, offset, limit)
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

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	user, err := h.service.GetByID(uint(id), requesterID, tenantID, isSuperadmin)
	if err != nil {
		status := http.StatusNotFound
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Email       string `json:"email" binding:"required,email"`
		Password    string `json:"password" binding:"required"`
		UserType    string `json:"user_type"`
		CompanyName string `json:"company_name"`
		JobTitle    string `json:"job_title"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := map[string]interface{}{
		"name":      req.Name,
		"email":     req.Email,
		"password":  req.Password,
		"user_type": req.UserType,
	}
	if req.CompanyName != "" {
		payload["company_name"] = req.CompanyName
	}
	if req.JobTitle != "" {
		payload["job_title"] = req.JobTitle
	}

	user, err := h.service.Create(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
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
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
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
	if req.State != "" {
		updates["state"] = req.State
	}
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.IdentityDocument != "" {
		updates["identity_document"] = req.IdentityDocument
	}

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	user, err := h.service.Update(uint(id), requesterID, tenantID, role, isManager, isSuperadmin, updates)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	if err := h.service.Delete(uint(id), requesterID, tenantID, role, isManager, isSuperadmin); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "a su cargo") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *UserHandler) ToggleStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	user, err := h.service.ToggleStatus(uint(id), requesterID, tenantID, role, isManager, isSuperadmin)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "a su cargo") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User status toggled", "is_active": user.IsActive})
}

func (h *UserHandler) PromoteToManager(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Optional body: { "is_manager": true|false }. If absent, req.IsManager
	// stays nil and the service falls back to the legacy toggle behavior.
	var req struct {
		IsManager *bool `json:"is_manager"`
	}
	_ = c.ShouldBindJSON(&req) // body is optional; ignore bind errors (empty body)

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	user, err := h.service.PromoteToManager(uint(id), requesterID, tenantID, role, isManager, isSuperadmin, req.IsManager)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "a su cargo") {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ReassignTeam mueve todos los reportes activos del manager actual (:id) al
// nuevo manager indicado en el body (new_manager_id), o los desasigna si es null.
func (h *UserHandler) ReassignTeam(c *gin.Context) {
	oldManagerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		NewManagerID *uint `json:"new_manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	n, err := h.service.ReassignTeam(uint(oldManagerID), req.NewManagerID, requesterID, tenantID, role, isManager, isSuperadmin)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reassigned": n})
}

func (h *UserHandler) GetEmployees(c *gin.Context) {
	userID := middleware.GetUserID(c)

	employees, err := h.service.GetEmployees(userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, employees)
}

func (h *UserHandler) AssignToManager(c *gin.Context) {
	professionalID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid professional ID"})
		return
	}

	var req struct {
		ManagerID uint `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	requesterID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	professional, err := h.service.AssignToManager(uint(professionalID), req.ManagerID, requesterID, tenantID, role, isManager, isSuperadmin)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Professional not found" || err.Error() == "Manager not found" {
			status = http.StatusNotFound
		} else if err.Error() == "User is not a manager" || err.Error() == "Un profesional no puede ser su propio manager" {
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, professional)
}

func (h *UserHandler) GetMyTeam(c *gin.Context) {
	userID := middleware.GetUserID(c)

	team, err := h.service.GetMyTeam(userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, team)
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if uint(id) != userID && !middleware.IsSuperadmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Can only change your own password"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ChangePassword(uint(id), req.CurrentPassword, req.NewPassword); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Current password is incorrect" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
