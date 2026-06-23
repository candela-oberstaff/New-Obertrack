package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       uint   `json:"user_id"`
	TenantID     *uint  `json:"tenant_id,omitempty"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	IsManager    bool   `json:"is_manager"`
	IsSuperadmin bool   `json:"is_superadmin"`
	EmpleadorID  *uint  `json:"empleador_id,omitempty"`
	TokenVersion int    `json:"tv"`
	TokenType    string `json:"typ"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// TokenVersionGetter returns the current token version for a user, used to
// enforce session revocation (audit finding A-04). When nil, the check is
// skipped.
type TokenVersionGetter func(userID uint) (int, error)

func AuthMiddleware(jwtSecret string, tvGetter TokenVersionGetter) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				tokenString = ""
			}
		}

		// Primary transport: httpOnly cookie set at login (audit finding A-03).
		// The browser sends it automatically, including on same-origin WS upgrades.
		if tokenString == "" {
			if cookie, err := c.Cookie("access_token"); err == nil {
				tokenString = cookie
			}
		}

		if tokenString == "" {
			log.Printf("[AUTH] Missing token for request: %s %s", c.Request.Method, c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Pin the signing algorithm to HMAC to prevent algorithm-confusion
			// attacks (audit finding A-07).
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || !token.Valid {
			log.Printf("[AUTH] Invalid token for request %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Reject refresh tokens on protected routes — only access tokens are valid
		// for API/WS requests.
		if claims.TokenType == "refresh" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token type"})
			c.Abort()
			return
		}

		// Enforce session revocation: the token's version must match the user's
		// current version (audit finding A-04).
		if tvGetter != nil {
			current, err := tvGetter(claims.UserID)
			if err != nil || current != claims.TokenVersion {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("is_manager", claims.IsManager)
		c.Set("is_superadmin", claims.IsSuperadmin)
		c.Set("empleador_id", claims.EmpleadorID)
		c.Set("tenant_id", claims.TenantID)

		c.Next()
	}
}

func RequireSuperadmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsSuperadmin(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Superadmin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePlatformTech permite superadmins y analistas de IT (soporte técnico
// de plataforma: Tools, Métricas y Auditoría).
func RequirePlatformTech() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsSuperadmin(c) || GetUserRole(c) == "analista_it" {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Acceso de soporte técnico requerido"})
		c.Abort()
	}
}

func GetUserID(c *gin.Context) uint {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	return userID.(uint)
}

func GetUserRole(c *gin.Context) string {
	return c.GetString("role")
}

func IsSuperadmin(c *gin.Context) bool {
	return c.GetBool("is_superadmin")
}

func IsManager(c *gin.Context) bool {
	return c.GetBool("is_manager")
}

func GetEmpleadorID(c *gin.Context) uint {
	empID, exists := c.Get("empleador_id")
	if !exists || empID == nil {
		return 0
	}
	// Handle both *uint and uint types
	switch v := empID.(type) {
	case uint:
		return v
	case *uint:
		if v != nil {
			return *v
		}
	}
	return 0
}

func GetTenantID(c *gin.Context) uint {
	tenantID, exists := c.Get("tenant_id")
	if exists && tenantID != nil {
		switch v := tenantID.(type) {
		case uint:
			return v
		case *uint:
			if v != nil {
				return *v
			}
		}
	}

	return GetEmpleadorID(c)
}
