package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

// IssueTokens expone generateTokenPair para re-emitir el JWT de una sesión ya
// autenticada (p.ej. al cambiar la empresa activa).
func (s *authService) IssueTokens(user *models.User) (string, string, error) {
	return s.generateTokenPair(user)
}

// baseClaims son los claims comunes a toda sesión del usuario, con el tenant
// que le corresponde por su cuenta.
func baseClaims(user *models.User) middleware.Claims {
	tenantID := user.EmpleadorID
	if tenantID == nil && user.UserType == models.UserTypeEmployer {
		tenantID = &user.ID
	}

	return middleware.Claims{
		UserID:       user.ID,
		TenantID:     tenantID,
		Email:        user.Email,
		Role:         string(user.UserType),
		IsManager:    user.IsManager,
		IsSupervisor: user.IsSupervisor,
		IsSuperadmin: user.IsSuperadmin,
		EmpleadorID:  user.EmpleadorID,
		TokenVersion: user.TokenVersion,
	}
}

func (s *authService) sign(claims middleware.Claims, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.TokenType = tokenType
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
}

func (s *authService) generateTokenPair(user *models.User) (string, string, error) {
	base := baseClaims(user)

	access, err := s.sign(base, "access", accessTokenTTL)
	if err != nil {
		return "", "", err
	}
	refresh, err := s.sign(base, "refresh", refreshTokenTTL)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// IssueScopedAccess emite SOLO un access token acotado a un alcance (p. ej.
// "tasks" para el acceso embebido desde el CRM). No hay refresh: al caducar,
// quien lo pidió vuelve a canjear. Refresh rechaza además cualquier token con
// alcance, para que una sesión acotada nunca se convierta en una completa.
//
// El tenant se fuerza al indicado sin tocar users.empleador_id (a diferencia
// de SwitchActive): la empresa activa del usuario en su Obertrack normal no
// cambia por abrir Tareas desde el CRM.
func (s *authService) IssueScopedAccess(user *models.User, tenantID uint, scope string, ttl time.Duration) (string, error) {
	claims := baseClaims(user)
	tid := tenantID
	claims.TenantID = &tid
	if user.UserType != models.UserTypeEmployer {
		claims.EmpleadorID = &tid
	}
	claims.Scope = scope
	return s.sign(claims, "access", ttl)
}
