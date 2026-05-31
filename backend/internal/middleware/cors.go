package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS restricts cross-origin access to an explicit allow-list instead of the
// previous wildcard "*" (audit finding C-06).
//
// Allowed origins are read from the CORS_ALLOWED_ORIGINS environment variable as
// a comma-separated list, e.g.:
//
//	CORS_ALLOWED_ORIGINS=https://obertrack.com,https://app.obertrack.com
//
// If the variable is empty we fall back to a safe localhost dev default.
func CORS() gin.HandlerFunc {
	allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func parseAllowedOrigins(raw string) map[string]bool {
	allowed := make(map[string]bool)
	if strings.TrimSpace(raw) == "" {
		// Safe development defaults.
		allowed["http://localhost:5173"] = true
		allowed["http://localhost:3000"] = true
		return allowed
	}
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return allowed
}
