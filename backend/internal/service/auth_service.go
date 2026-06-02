package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// hashResetToken returns a SHA-256 hex digest. Reset tokens are stored hashed so
// a DB leak does not allow password resets (audit finding M-09).
func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ValidatePasswordStrength enforces a minimum password policy (audit finding
// M-08): at least 8 chars with a mix of letters and digits.
func ValidatePasswordStrength(pw string) error {
	if len(pw) < 8 {
		return errors.New("la contraseña debe tener al menos 8 caracteres")
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("la contraseña debe incluir letras y números")
	}
	return nil
}

type AuthService interface {
	Register(name, email, password, userTypeStr, companyName string, empleadorID *uint, phoneNumber, location, jobTitle string) (*models.User, string, string, error)
	Login(email, password string) (*models.User, string, string, error)
	Refresh(refreshToken string) (*models.User, string, string, error)
	GetUserDetails(id uint) (*models.User, error)
	GetTokenVersion(id uint) (int, error)
	GetPublicCompanies() ([]map[string]interface{}, error)
	ForgotPassword(email string) error
	ResetPassword(token, newPassword string) error
}

const (
	accessTokenTTL  = 2 * time.Hour
	refreshTokenTTL = 7 * 24 * time.Hour
)

type authService struct {
	userRepo  repository.UserRepository
	jwtSecret string
	jwtExpiry time.Duration
	brevoSvc  *BrevoService
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, brevoSvc *BrevoService) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		jwtExpiry: 24 * time.Hour,
		brevoSvc:  brevoSvc,
	}
}

func (s *authService) Register(name, email, password, userTypeStr, companyName string, empleadorID *uint, phoneNumber, location, jobTitle string) (*models.User, string, string, error) {
	if err := ValidatePasswordStrength(password); err != nil {
		return nil, "", "", err
	}

	_, err := s.userRepo.GetByEmail(email)
	if err == nil {
		return nil, "", "", errors.New("Email already registered")
	}

	// Prevent privilege escalation via public self-registration: superadmin
	// accounts can never be created through this endpoint (audit finding B-01).
	if userTypeStr == "superadmin" {
		return nil, "", "", errors.New("Tipo de usuario no válido")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", errors.New("Failed to hash password")
	}

	// Logic to classify role
	userType := models.UserTypeProfessional
	isSuperadmin := false
	switch userTypeStr {
	case "empleador", "empresa":
		userType = models.UserTypeEmployer
	case "superadmin":
		userType = models.UserTypeSuperadmin
		isSuperadmin = true
	}

	user := &models.User{
		Name:         name,
		Email:        email,
		Password:     string(hashedPassword),
		UserType:     userType,
		CompanyName:  companyName,
		IsSuperadmin: isSuperadmin,
		IsActive:     true,
		EmpleadorID:  empleadorID,
		PhoneNumber:  phoneNumber,
		Location:     location,
		JobTitle:     jobTitle,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, "", "", err
	}

	access, refresh, err := s.generateTokenPair(user)
	if err != nil {
		return nil, "", "", errors.New("Failed to generate token")
	}

	return user, access, refresh, nil
}

func (s *authService) Login(email, password string) (*models.User, string, string, error) {
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, "", "", errors.New("Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", "", errors.New("Invalid credentials")
	}

	if !user.IsActive {
		return nil, "", "", errors.New("Tu cuenta ha sido suspendida. Contacta al administrador.")
	}

	if user.UserType == models.UserTypeProfessional && user.EmpleadorID != nil {
		if employer, err := s.userRepo.GetByID(*user.EmpleadorID); err == nil && !employer.IsActive {
			return nil, "", "", errors.New("El acceso de tu empresa ha sido suspendido. Contacta al administrador.")
		}
	}

	access, refresh, err := s.generateTokenPair(user)
	if err != nil {
		return nil, "", "", errors.New("Failed to generate token")
	}

	return user, access, refresh, nil
}

// Refresh validates a refresh token and, if the session is still valid, issues a
// fresh access+refresh pair (rotation).
func (s *authService) Refresh(refreshToken string) (*models.User, string, string, error) {
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(refreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.jwtSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid || claims.TokenType != "refresh" {
		return nil, "", "", errors.New("invalid refresh token")
	}

	user, err := s.userRepo.GetByID(claims.UserID)
	if err != nil {
		return nil, "", "", errors.New("invalid refresh token")
	}
	// Session revocation check (audit finding A-04).
	if !user.IsActive || user.TokenVersion != claims.TokenVersion {
		return nil, "", "", errors.New("session expired")
	}

	access, refresh, err := s.generateTokenPair(user)
	if err != nil {
		return nil, "", "", errors.New("Failed to generate token")
	}
	return user, access, refresh, nil
}

func (s *authService) GetUserDetails(id uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	return user, nil
}

func (s *authService) GetTokenVersion(id uint) (int, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return 0, err
	}
	return user.TokenVersion, nil
}

func (s *authService) generateTokenPair(user *models.User) (string, string, error) {
	tenantID := user.EmpleadorID
	if tenantID == nil && user.UserType == models.UserTypeEmployer {
		tenantID = &user.ID
	}

	base := middleware.Claims{
		UserID:       user.ID,
		TenantID:     tenantID,
		Email:        user.Email,
		Role:         string(user.UserType),
		IsManager:    user.IsManager,
		IsSuperadmin: user.IsSuperadmin,
		EmpleadorID:  user.EmpleadorID,
		TokenVersion: user.TokenVersion,
	}

	accessClaims := base
	accessClaims.TokenType = "access"
	accessClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}

	refreshClaims := base
	refreshClaims.TokenType = "refresh"
	refreshClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(refreshTokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

func (s *authService) GetPublicCompanies() ([]map[string]interface{}, error) {
	users, _, err := s.userRepo.GetAll("empleador", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}

	companies := make([]map[string]interface{}, 0)
	for _, u := range users {
		name := u.CompanyName
		if name == "" {
			name = u.Name
		}
		
		companies = append(companies, map[string]interface{}{
			"id":   u.ID,
			"name": name,
		})
	}

	return companies, nil
}

func (s *authService) ForgotPassword(email string) error {
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		// Don't reveal whether the email exists
		log.Printf("[Auth] ForgotPassword requested for unknown email: %s", email)
		return nil
	}

	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return errors.New("failed to generate reset token")
	}
	token := hex.EncodeToString(tokenBytes)

	// Set token with 1-hour expiry. Store only the HASH; the raw token is sent
	// in the email link (audit finding M-09).
	expiry := time.Now().Add(1 * time.Hour)
	user.ResetToken = hashResetToken(token)
	user.ResetTokenExpiry = &expiry

	if err := s.userRepo.Save(user); err != nil {
		return errors.New("failed to save reset token")
	}

	// Build reset URL
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = os.Getenv("SERVICE_URL_FRONTEND")
	}
	if frontendURL == "" {
		if os.Getenv("GIN_MODE") == "release" {
			frontendURL = "https://obertrack.com"
		} else {
			frontendURL = "https://obertrack.com" // Always default to obertrack.com as requested by user
		}
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, token)

	// Send branded email
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
</head>
<body style="font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f2fb; margin: 0; padding: 20px; color: #060b23;">
	<div style="max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 20px; overflow: hidden; box-shadow: 0 10px 15px -3px rgba(6, 11, 35, 0.1), 0 4px 6px -2px rgba(6, 11, 35, 0.05); border: 1px solid #ddd9ef;">

		<!-- Banner con Logo -->
		<div style="background: linear-gradient(135deg, #060b23 0%%, #cc33cc 100%%); padding: 32px 24px; color: #ffffff; text-align: center;">
			<img src="https://obertrack.com/logos/Horizontal_Blanco.png" alt="Obertrack Logo" height="40" style="display: block; margin: 0 auto 12px auto; height: 40px; border: 0; outline: none;" />
			<h1 style="font-size: 20px; font-weight: 700; opacity: 0.95; margin: 0; color: #ffffff; font-family: sans-serif;">Recuperar Contraseña</h1>
		</div>

		<!-- Contenido -->
		<div style="padding: 32px 24px;">
			<p style="font-size: 16px; line-height: 1.6; margin-bottom: 24px; color: #060b23; font-family: sans-serif;">Hola <strong>%s</strong>,</p>
			<p style="font-size: 15px; line-height: 1.6; margin-bottom: 24px; color: #5c5680; font-family: sans-serif;">Recibimos una solicitud para restablecer la contraseña de tu cuenta. Haz clic en el botón de abajo para crear una nueva contraseña.</p>

			<!-- Botón CTA -->
			<div style="text-align: center; margin: 32px 0;">
				<a href="%s" style="display: inline-block; background: linear-gradient(135deg, #cc33cc 0%%, #8a2be2 100%%); color: #ffffff; text-decoration: none; padding: 14px 40px; border-radius: 12px; font-size: 15px; font-weight: 700; font-family: sans-serif; box-shadow: 0 4px 16px rgba(204, 51, 204, 0.35);">Restablecer Contraseña</a>
			</div>

			<p style="font-size: 13px; line-height: 1.6; color: #8880a8; font-family: sans-serif;">Si no solicitaste este cambio, puedes ignorar este correo. Tu contraseña actual seguirá siendo la misma.</p>
			<p style="font-size: 13px; line-height: 1.6; color: #8880a8; font-family: sans-serif;">Este enlace expirará en <strong>1 hora</strong>.</p>

			<!-- Link alternativo -->
			<div style="background: #f5f2fb; border: 1px solid #ddd9ef; border-radius: 12px; padding: 16px; margin-top: 24px;">
				<p style="font-size: 12px; color: #8880a8; margin: 0 0 8px 0; font-family: sans-serif;">Si el botón no funciona, copia y pega este enlace en tu navegador:</p>
				<p style="font-size: 12px; color: #8a2be2; word-break: break-all; margin: 0; font-family: sans-serif;">%s</p>
			</div>
		</div>

		<!-- Footer -->
		<div style="background: #f5f2fb; padding: 24px; text-align: center; font-size: 12px; color: #8880a8; border-top: 1px solid #ddd9ef; font-family: sans-serif;">
			Este es un correo automático generado de forma segura por <strong>Obertrack</strong>.<br>
			&copy; 2026 Obertrack. Todos los derechos reservados.
		</div>
	</div>
</body>
</html>`, user.Name, resetLink, resetLink)

	if err := s.brevoSvc.SendEmail(user.Email, user.Name, "Obertrack - Recuperar Contraseña", htmlContent); err != nil {
		log.Printf("[Auth] Failed to send reset email to %s: %v", user.Email, err)
		return errors.New("failed to send reset email")
	}

	log.Printf("[Auth] Password reset email sent to %s", user.Email)
	return nil
}

func (s *authService) ResetPassword(token, newPassword string) error {
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}

	// Look up by the hashed token (audit finding M-09).
	user, err := s.userRepo.GetByResetToken(hashResetToken(token))
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	// Check expiry
	if user.ResetTokenExpiry == nil || time.Now().After(*user.ResetTokenExpiry) {
		// Clear expired token
		user.ResetToken = ""
		user.ResetTokenExpiry = nil
		s.userRepo.Save(user)
		return errors.New("reset token has expired")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	// Update password, clear token, and revoke all existing sessions (A-04).
	user.Password = string(hashedPassword)
	user.ResetToken = ""
	user.ResetTokenExpiry = nil
	user.TokenVersion++

	if err := s.userRepo.Save(user); err != nil {
		return errors.New("failed to update password")
	}

	log.Printf("[Auth] Password successfully reset for user %d (%s)", user.ID, user.Email)
	return nil
}
