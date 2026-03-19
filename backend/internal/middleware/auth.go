package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       uint   `json:"user_id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	IsManager    bool   `json:"is_manager"`
	IsSuperadmin bool   `json:"is_superadmin"`
	EmpleadorID  *uint  `json:"empleador_id,omitempty"`
	jwt.RegisteredClaims
}

func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				tokenString = ""
			}
		}

		if tokenString == "" {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
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

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("is_manager", claims.IsManager)
		c.Set("is_superadmin", claims.IsSuperadmin)
		c.Set("empleador_id", claims.EmpleadorID)

		c.Next()
	}
}

func RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		isSuperadmin := c.GetBool("is_superadmin")
		isManager := c.GetBool("is_manager")

		if isSuperadmin {
			c.Next()
			return
		}

		for _, r := range allowedRoles {
			if role == r || (r == "manager" && isManager) {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
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
