package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

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
