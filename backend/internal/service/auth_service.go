package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type AuthService interface {
	Register(name, email, password, userTypeStr, companyName string, empleadorID *uint) (*models.User, string, error)
	Login(email, password string) (*models.User, string, error)
	GetUserDetails(id uint) (*models.User, error)
	GetPublicCompanies() ([]map[string]interface{}, error)
}

type authService struct {
	userRepo  repository.UserRepository
	jwtSecret string
	jwtExpiry time.Duration
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		jwtExpiry: 24 * time.Hour,
	}
}

func (s *authService) Register(name, email, password, userTypeStr, companyName string, empleadorID *uint) (*models.User, string, error) {
	_, err := s.userRepo.GetByEmail(email)
	if err == nil {
		return nil, "", errors.New("Email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", errors.New("Failed to hash password")
	}

	// Logic to classify role
	userType := models.UserTypeProfessional
	isSuperadmin := false
	switch userTypeStr {
	case "empleador", "empresa":
		userType = models.UserTypeEmployer
	case "superadmin":
		userType = models.UserType("superadmin")
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
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, "", err
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, "", errors.New("Failed to generate token")
	}

	return user, token, nil
}

func (s *authService) Login(email, password string) (*models.User, string, error) {
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, "", errors.New("Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", errors.New("Invalid credentials")
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, "", errors.New("Failed to generate token")
	}

	return user, token, nil
}

func (s *authService) GetUserDetails(id uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	return user, nil
}

func (s *authService) generateToken(user *models.User) (string, error) {
	claims := middleware.Claims{
		UserID:       user.ID,
		Email:        user.Email,
		Role:         string(user.UserType),
		IsManager:    user.IsManager,
		IsSuperadmin: user.IsSuperadmin,
		EmpleadorID:  user.EmpleadorID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *authService) GetPublicCompanies() ([]map[string]interface{}, error) {
	users, _, err := s.userRepo.GetAll("empleador", "", 0, 1000)
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
