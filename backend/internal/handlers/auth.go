package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

const (
	accessCookieMaxAge  = 2 * 60 * 60      // 2h, mirrors access token TTL
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // 7d, mirrors refresh token TTL
	refreshCookiePath   = "/api/auth/refresh"
)

// setAuthCookies writes the access and refresh tokens as httpOnly cookies
// (audit findings A-03/A-04). Secure is enabled in production (GIN_MODE=release).
func setAuthCookies(c *gin.Context, access, refresh string) {
	secure := os.Getenv("GIN_MODE") == "release"
	// For production behind proxy, we might need Secure: false if SSL terminates at the load balancer
	// but Coolify usually handles this. Let's make it configurable or safer.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", access, accessCookieMaxAge, "/", "", secure, true)
	c.SetCookie("refresh_token", refresh, refreshCookieMaxAge, "/", "", secure, true)
}

func clearAuthCookies(c *gin.Context) {
	secure := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_token", "", -1, "/", "", secure, true)
}

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

type RegisterRequest struct {
	Name        string `json:"name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	UserType    string `json:"user_type" binding:"required"`
	CompanyName string `json:"company_name"`
	EmpleadorID *uint  `json:"empleador_id"`
	PhoneNumber string `json:"phone_number"`
	Location    string `json:"location"`
	JobTitle    string `json:"job_title"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	User models.User `json:"user"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate based on UserType
	switch req.UserType {
	case "profesional", "empleado":
		if req.PhoneNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El teléfono es obligatorio para profesionales"})
			return
		}
		if req.Location == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La ubicación es obligatoria para profesionales"})
			return
		}
		if req.JobTitle == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El rol (cargo) es obligatorio para profesionales"})
			return
		}
		if req.EmpleadorID == nil || *req.EmpleadorID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La empresa es obligatoria para profesionales"})
			return
		}
	case "empleador", "empresa":
		if req.CompanyName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El nombre de la empresa es obligatorio"})
			return
		}
		if req.PhoneNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El teléfono es obligatorio para empresas"})
			return
		}
		if req.Location == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La ubicación es obligatoria para empresas"})
			return
		}
	case "superadmin", "customer_success":
		// Allowed types with no additional field validation for now
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipo de usuario no válido"})
		return
	}

	user, access, refresh, err := h.authService.Register(req.Name, req.Email, req.Password, req.UserType, req.CompanyName, req.EmpleadorID, req.PhoneNumber, req.Location, req.JobTitle)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Email already registered" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	setAuthCookies(c, access, refresh)
	c.JSON(http.StatusCreated, AuthResponse{User: *user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, access, refresh, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Invalid credentials" {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	setAuthCookies(c, access, refresh)
	c.JSON(http.StatusOK, AuthResponse{User: *user})
}

// Refresh issues a new token pair from a valid refresh cookie.
func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing refresh token"})
		return
	}

	user, access, refresh, err := h.authService.Refresh(refreshToken)
	if err != nil {
		clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	setAuthCookies(c, access, refresh)
	c.JSON(http.StatusOK, AuthResponse{User: *user})
}

// Logout clears the auth cookies.
func (h *AuthHandler) Logout(c *gin.Context) {
	clearAuthCookies(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}


func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)

	user, err := h.authService.GetUserDetails(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) GetCompanies(c *gin.Context) {
	companies, err := h.authService.GetPublicCompanies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, companies)
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.ForgotPassword(req.Email); err != nil {
		// Still return 200 to not reveal if email exists
		c.JSON(http.StatusOK, gin.H{"message": "Si el correo está registrado, recibirás un enlace para restablecer tu contraseña."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Si el correo está registrado, recibirás un enlace para restablecer tu contraseña."})
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.ResetPassword(req.Token, req.NewPassword); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "invalid or expired reset token" || err.Error() == "reset token has expired" {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contraseña actualizada exitosamente."})
}
