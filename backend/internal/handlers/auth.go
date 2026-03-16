package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

type AuthHandler struct {
	db        *gorm.DB
	jwtSecret string
	jwtExpiry time.Duration
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:        db,
		jwtSecret: cfg.JWTSecret,
		jwtExpiry: 24 * time.Hour,
	}
}

type RegisterRequest struct {
	Name        string `json:"name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	UserType    string `json:"user_type"`
	CompanyName string `json:"company_name"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	User         models.User `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existingUser := models.User{}
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	userType := models.UserTypeEmployee
	if req.UserType == "empleador" {
		userType = models.UserTypeEmployer
	}

	user := models.User{
		Name:        req.Name,
		Email:       req.Email,
		Password:    string(hashedPassword),
		UserType:    userType,
		CompanyName: req.CompanyName,
	}

	log.Printf("Creating user: %+v", user)

	if err := h.db.Create(&user).Error; err != nil {
		log.Printf("Error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user", "details": err.Error()})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		User:        user,
		AccessToken: token,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User:        user,
		AccessToken: token,
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) generateToken(user *models.User) (string, error) {
	claims := middleware.Claims{
		UserID:       user.ID,
		Email:        user.Email,
		Role:         string(user.UserType),
		IsManager:    user.IsManager,
		IsSuperadmin: user.IsSuperadmin,
		EmpleadorID:  user.EmpleadorID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(h.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
