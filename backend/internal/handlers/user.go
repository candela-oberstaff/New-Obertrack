package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

type UpdateUserRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	JobTitle    string `json:"job_title"`
	PhoneNumber string `json:"phone_number"`
	Country     string `json:"country"`
	City        string `json:"city"`
	Location    string `json:"location"`
}

func (h *UserHandler) GetAll(c *gin.Context) {
	var users []models.User
	query := h.db.Model(&models.User{})

	role := c.Query("role")
	if role != "" {
		query = query.Where("user_type = ?", role)
	}

	isManager := c.Query("is_manager")
	if isManager != "" {
		query = query.Where("is_manager = ?", isManager == "true")
	}

	var total int64
	query.Count(&total)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
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

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
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

	user := models.User{
		Name:        req.Name,
		Email:       req.Email,
		Password:    req.Password,
		UserType:    models.UserType(req.UserType),
		CompanyName: req.CompanyName,
		JobTitle:    req.JobTitle,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
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

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
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
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
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

	if err := h.db.Delete(&models.User{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
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

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := h.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User status toggled"})
}

func (h *UserHandler) PromoteToManager(c *gin.Context) {
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

	if err := h.db.Model(&user).Update("is_manager", !user.IsManager).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to promote user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetEmployees(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var employees []models.User
	if err := h.db.Where("empleador_id = ?", userID).Find(&employees).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employees"})
		return
	}

	c.JSON(http.StatusOK, employees)
}
