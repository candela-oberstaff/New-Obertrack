package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
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
	Register(name, email, password, userTypeStr, companyName string, empleadorID *uint, phoneNumber, location, jobTitle, industry, country, address, state, city string) (*models.User, string, string, error)
	Login(email, password string) (*models.User, string, string, error)
	Refresh(refreshToken string) (*models.User, string, string, error)
	GetUserDetails(id uint) (*models.User, error)
	GetTokenVersion(id uint) (int, error)
	// IssueTokens genera un par access/refresh para un usuario ya autenticado
	// (usado al cambiar de empresa activa: re-emite el JWT con el nuevo tenant).
	IssueTokens(user *models.User) (string, string, error)
	GetPublicCompanies() ([]map[string]interface{}, error)
	ForgotPassword(email string) error
	// SendPasswordSetupEmail invita a CREAR la contraseña por primera vez (alta
	// de un profesional que aprobó su inducción). Usa el mismo mecanismo seguro
	// de token que ForgotPassword, pero con el mensaje correcto: quien lo recibe
	// nunca tuvo contraseña, así que "restablecer" no aplica.
	SendPasswordSetupEmail(email string) error
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

func (s *authService) Register(name, email, password, userTypeStr, companyName string, empleadorID *uint, phoneNumber, location, jobTitle, industry, country, address, state, city string) (*models.User, string, string, error) {
	if err := ValidatePasswordStrength(password); err != nil {
		return nil, "", "", err
	}

	_, err := s.userRepo.GetByEmail(email)
	if err == nil {
		return nil, "", "", errors.New("Email already registered")
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
	case "customer_success":
		userType = models.UserTypeCustomerSuccess
	case "superadmin":
		userType = models.UserTypeSuperadmin
		isSuperadmin = true
	}

	// Solo profesionales y customer success pueden quedar vinculados a una empresa.
	if userType != models.UserTypeProfessional && userType != models.UserTypeCustomerSuccess {
		empleadorID = nil
	}
	if empleadorID != nil && *empleadorID == 0 {
		empleadorID = nil
	}
	if empleadorID != nil {
		employer, err := s.userRepo.GetByID(*empleadorID)
		if err != nil || employer.UserType != models.UserTypeEmployer {
			return nil, "", "", errors.New("La empresa seleccionada no es válida")
		}
	}

	user := &models.User{
		Name:         name,
		Email:        email,
		Password:     string(hashedPassword),
		UserType:     userType,
		CompanyName:  companyName,
		Industry:     industry,
		IsSuperadmin: isSuperadmin,
		IsActive:     true,
		EmpleadorID:  empleadorID,
		PhoneNumber:  phoneNumber,
		Country:      country,
		State:        state,
		City:         city,
		Location:     location,
		Address:      address,
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

	// Portero de inducción: el profesional recién contratado no entra hasta
	// aprobar. Las cuentas existentes están en 'not_required' y no se ven
	// afectadas (ver models/induction.go).
	switch user.OnboardingStatus {
	case models.OnboardingPending:
		return nil, "", "", errors.New("Aún no completas tu inducción. Revisa el enlace que enviamos a tu correo.")
	case models.OnboardingBlocked:
		return nil, "", "", errors.New("Tu acceso está en revisión. Nuestro equipo de soporte se pondrá en contacto contigo.")
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

func (s *authService) GetPublicCompanies() ([]map[string]interface{}, error) {
	users, _, err := s.userRepo.GetAll("empleador", "", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}

	companies := make([]map[string]interface{}, 0)
	for _, u := range users {
		name := u.CompanyName
		if name == "" {
			name = u.Name
		}
		if name == "" {
			continue
		}
		companies = append(companies, map[string]interface{}{
			"id":   u.ID,
			"name": name,
		})
	}

	return companies, nil
}

// issueResetToken genera un token de un solo uso, guarda solo su HASH en el
// usuario (audit finding M-09) y devuelve el token en claro para el enlace.
// Lo comparten el flujo de recuperación y el de alta de contraseña.
func (s *authService) issueResetToken(user *models.User, ttl time.Duration) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", errors.New("failed to generate reset token")
	}
	token := hex.EncodeToString(tokenBytes)

	expiry := time.Now().Add(ttl)
	user.ResetToken = hashResetToken(token)
	user.ResetTokenExpiry = &expiry

	if err := s.userRepo.Save(user); err != nil {
		return "", errors.New("failed to save reset token")
	}
	return token, nil
}

// frontendBaseURL resuelve la base pública del frontend para los enlaces.
func frontendBaseURL() string {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = os.Getenv("SERVICE_URL_FRONTEND")
	}
	if frontendURL == "" {
		frontendURL = "https://obertrack.com"
	}
	return strings.TrimRight(frontendURL, "/")
}

func (s *authService) ForgotPassword(email string) error {
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		// Don't reveal whether the email exists
		log.Printf("[Auth] ForgotPassword requested for unknown email: %s", email)
		return nil
	}

	token, err := s.issueResetToken(user, 1*time.Hour)
	if err != nil {
		return err
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendBaseURL(), token)

	htmlContent := BuildPasswordResetHTML(user.Name, resetLink)

	if err := s.brevoSvc.SendEmail(user.Email, user.Name, "Obertrack - Recuperar Contraseña", htmlContent); err != nil {
		log.Printf("[Auth] Failed to send reset email to %s: %v", user.Email, err)
		return errors.New("failed to send reset email")
	}

	log.Printf("[Auth] Password reset email sent to %s", user.Email)
	return nil
}

// SendPasswordSetupEmail avisa a un profesional recién aprobado que ya tiene
// acceso y lo invita a CREAR su contraseña. Nunca viaja una clave en claro: se
// usa el mismo token de un solo uso del flujo de recuperación.
//
// La vigencia es de 24 h (no 1 h como la recuperación): esto llega al terminar
// la inducción y la persona puede no atenderlo de inmediato. Si vence, siempre
// queda "¿Olvidaste tu contraseña?" como salida.
func (s *authService) SendPasswordSetupEmail(email string) error {
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		log.Printf("[Auth] SendPasswordSetupEmail for unknown email: %s", email)
		return nil
	}

	token, err := s.issueResetToken(user, 24*time.Hour)
	if err != nil {
		return err
	}
	// setup=1 hace que la pantalla hable de "crear" y no de "restablecer".
	setupLink := fmt.Sprintf("%s/reset-password?token=%s&setup=1", frontendBaseURL(), token)

	htmlContent := BuildPasswordSetupHTML(user.Name, user.Email, setupLink)

	if err := s.brevoSvc.SendEmail(user.Email, user.Name, "Obertrack - Crea tu contraseña", htmlContent); err != nil {
		log.Printf("[Auth] Failed to send setup email to %s: %v", user.Email, err)
		return errors.New("failed to send setup email")
	}

	log.Printf("[Auth] Password setup email sent to %s", user.Email)
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
