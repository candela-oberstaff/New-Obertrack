package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

const (
	accessCookieMaxAge  = 2 * 60 * 60      // 2h, mirrors access token TTL
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // 7d, mirrors refresh token TTL
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
	authService   service.AuthService
	auditSvc      service.AuditService
	rbacSvc       service.RBACService
	employmentSvc service.EmploymentService
}

func NewAuthHandler(authService service.AuthService, auditSvc service.AuditService, rbacSvc service.RBACService, employmentSvc service.EmploymentService) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		auditSvc:      auditSvc,
		rbacSvc:       rbacSvc,
		employmentSvc: employmentSvc,
	}
}

// attachContext agrega al usuario sus permisos efectivos (del tenant activo) y
// sus empresas activas (switcher multi-empresa), para que el frontend ajuste la
// UI sin llamadas extra.
func (h *AuthHandler) attachContext(user *models.User) {
	if user == nil {
		return
	}
	if perms, hasRoles, err := h.rbacSvc.EffectivePermissions(user.ID, models.TenantForUser(user)); err == nil && hasRoles {
		user.Permissions = perms
	}
	if companies, err := h.employmentSvc.ActiveCompanies(user.ID); err == nil && len(companies) > 0 {
		user.Companies = companies
	}
}

type RegisterRequest struct {
	Name        string `json:"name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	UserType    string `json:"user_type" binding:"required"`
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
	EmpleadorID *uint  `json:"empleador_id"`
	PhoneNumber string `json:"phone_number"`
	Country     string `json:"country"`
	State       string `json:"state"`
	City        string `json:"city"`
	Location    string `json:"location"`
	Address     string `json:"address"`
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
		if req.Country == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El país es obligatorio para profesionales"})
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
		if req.Country == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El país es obligatorio para empresas"})
			return
		}
		if req.Industry == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El rubro o industria es obligatorio para empresas"})
			return
		}
	case "superadmin", "customer_success":
		// Allowed types with no additional field validation for now
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipo de usuario no válido"})
		return
	}

	user, access, refresh, err := h.authService.Register(req.Name, req.Email, req.Password, req.UserType, req.CompanyName, req.EmpleadorID, req.PhoneNumber, req.Location, req.JobTitle, req.Industry, req.Country, req.Address, req.State, req.City)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Email already registered" {
			status = http.StatusConflict
		} else if err.Error() == "Ya existe un superadmin registrado" {
			status = http.StatusConflict
		} else if err.Error() == "Tipo de usuario no válido" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// Toda empresa nueva nace con los roles preconfigurados (best-effort:
	// un fallo aquí no debe impedir el registro).
	if user.UserType == models.UserTypeEmployer {
		if err := h.rbacSvc.SeedDefaultRoles(user.ID, user.ID); err != nil {
			log.Printf("[auth.register] no se pudieron sembrar los roles preconfigurados del tenant %d: %v", user.ID, err)
		}
	}
	// Dual-write de la membresía (fase 0). Best-effort.
	if err := h.employmentSvc.SyncActiveForUser(user); err != nil {
		log.Printf("[auth.register] no se pudo sincronizar la membresía del usuario %d: %v", user.ID, err)
	}

	h.auditSvc.RecordAuth("auth.register", &user.ID, user.Email, string(user.UserType), true, c.ClientIP(), c.Request.UserAgent())
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
		h.auditSvc.RecordAuth("auth.login_failed", nil, req.Email, "", false, c.ClientIP(), c.Request.UserAgent())
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	h.auditSvc.RecordAuth("auth.login", &user.ID, user.Email, string(user.UserType), true, c.ClientIP(), c.Request.UserAgent())
	setAuthCookies(c, access, refresh)
	h.attachContext(user)
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
	var actorID *uint
	if id := middleware.GetUserID(c); id > 0 {
		actorID = &id
	}
	h.auditSvc.RecordAuth("auth.logout", actorID, c.GetString("email"), middleware.GetUserRole(c), true, c.ClientIP(), c.Request.UserAgent())
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

	h.attachContext(user)
	c.JSON(http.StatusOK, user)
}

// SwitchCompany cambia la empresa activa del profesional multi-empresa y
// re-emite el JWT de esta sesión con el nuevo tenant.
func (h *AuthHandler) SwitchCompany(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		CompanyID uint `json:"company_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.employmentSvc.SwitchActive(userID, req.CompanyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	access, refresh, err := h.authService.IssueTokens(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo emitir la sesión"})
		return
	}
	setAuthCookies(c, access, refresh)

	if full, err := h.authService.GetUserDetails(user.ID); err == nil {
		user = full
	}
	h.attachContext(user)
	c.JSON(http.StatusOK, user)
}

// --- Expediente propio (FASE 3): el profesional ve su CV vivo ---

// MyEmployments lista las membresías (empleos) del propio usuario autenticado.
func (h *AuthHandler) MyEmployments(c *gin.Context) {
	views, err := h.employmentSvc.ListForUser(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar tus empleos"})
		return
	}
	if views == nil {
		views = []service.EmploymentView{}
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

// MyCV devuelve el CV vivo del propio usuario: su trayectoria unificada en
// todas las empresas, con lo que cada una compartió.
func (h *AuthHandler) MyCV(c *gin.Context) {
	cv, err := h.employmentSvc.GetCV(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar tu CV"})
		return
	}
	c.JSON(http.StatusOK, cv)
}

// slugify deja solo letras/números/guiones para un nombre de archivo seguro.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "documento"
	}
	return out
}

// MyCVPDF descarga el CV del propio usuario en PDF.
func (h *AuthHandler) MyCVPDF(c *gin.Context) {
	bytes, name, err := h.employmentSvc.GetCVPDF(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el PDF"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=cv_%s.pdf", slugify(name)))
	c.Data(http.StatusOK, "application/pdf", bytes)
}

// MyExpediente devuelve el expediente de uno de los empleos del propio usuario,
// con visibilidad de profesional (solo ve lo que la empresa compartió).
func (h *AuthHandler) MyExpediente(c *gin.Context) {
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	exp, err := h.employmentSvc.GetExpediente(uint(empID), service.AudienceProfessional)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Solo el dueño del empleo puede ver su propio expediente.
	if exp.Employment.UserID != middleware.GetUserID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado"})
		return
	}
	c.JSON(http.StatusOK, exp)
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

	h.auditSvc.RecordAuth("auth.password_reset", nil, "", "", true, c.ClientIP(), c.Request.UserAgent())
	c.JSON(http.StatusOK, gin.H{"message": "Contraseña actualizada exitosamente."})
}
