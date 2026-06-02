package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// SharedSecretAuth protects an endpoint with a static shared secret read from the
// given environment variable. The caller must send the same value in the
// `headerName` request header. Comparison is constant-time.
//
// If the environment variable is empty the endpoint is rejected with 503 so that
// a misconfiguration fails closed instead of leaving the route wide open
// (audit findings C-01 / C-03).
func SharedSecretAuth(envVar, headerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv(envVar)
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "endpoint not configured",
			})
			return
		}
		provided := c.GetHeader(headerName)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// WahaHMACAuth verifies the HMAC-SHA512 signature WAHA sends in the
// `X-Webhook-Hmac` header, computed over the raw request body with the shared
// secret stored in WAHA_WEBHOOK_HMAC. The body is buffered and restored so the
// downstream handler can still read it.
//
// If the secret is unset the request is rejected (fail closed).
func WahaHMACAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv("WAHA_WEBHOOK_HMAC")
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "webhook not configured",
			})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		// Restore the body for the actual handler.
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))

		provided := c.GetHeader("X-Webhook-Hmac")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
		c.Next()
	}
}
