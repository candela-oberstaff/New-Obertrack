package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

var limiter = &rateLimiter{
	requests: make(map[string][]time.Time),
	limit:    600,
	window:   time.Minute,
}

func init() {
	// Periodically evict stale keys so the map cannot grow unbounded and be used
	// as a memory-exhaustion DoS vector (audit finding A-05).
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			limiter.cleanup()
		}
	}()
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	windowStart := time.Now().Add(-rl.window)
	for key, times := range rl.requests {
		var valid []time.Time
		for _, t := range times {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = valid
		}
	}
}

func (rl *rateLimiter) isAllowed(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	if _, exists := rl.requests[key]; !exists {
		rl.requests[key] = []time.Time{}
	}

	var validRequests []time.Time
	for _, t := range rl.requests[key] {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}
	rl.requests[key] = validRequests

	if len(validRequests) >= rl.limit {
		return false
	}

	rl.requests[key] = append(rl.requests[key], now)
	return true
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()

		if !limiter.isAllowed(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// authLimiter is a stricter limiter for credential endpoints (login, password
// reset) to slow down brute-force / credential-stuffing (audit finding A-05).
var authLimiter = &rateLimiter{
	requests: make(map[string][]time.Time),
	limit:    10,
	window:   time.Minute,
}

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			authLimiter.cleanup()
		}
	}()
}

// AuthRateLimitMiddleware applies the stricter per-IP limit to auth endpoints.
func AuthRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "auth:" + c.ClientIP()
		if !authLimiter.isAllowed(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many attempts. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// integrationLimiter acota el tráfico servidor-a-servidor de las integraciones.
//
// El volumen real de Obersuite es de unas 200 peticiones al día repartidas entre
// 28 personas; 120 por minuto deja tres órdenes de magnitud de margen. No está
// puesto para racionar el uso normal, sino para que un bucle de reintentos suyo
// —o un despliegue que arranque varias réplicas sincronizando a la vez— no
// arrastre a la aplicación entera: al venir todo de una sola IP, sin este corte
// consumirían el cupo global de 600/min que comparten con el resto del tráfico.
var integrationLimiter = &rateLimiter{
	requests: make(map[string][]time.Time),
	limit:    120,
	window:   time.Minute,
}

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			integrationLimiter.cleanup()
		}
	}()
}

// IntegrationRateLimitMiddleware limita un grupo de integración por IP.
//
// Devuelve Retry-After porque del otro lado hay un reintentador automático que
// hoy repite ante cualquier respuesta que no sea 2xx: si no se le dice cuánto
// esperar, vuelve de inmediato y se realimenta el corte. La cabecera convierte
// el 429 en "espera un minuto" en vez de en un fallo más que reintentar.
func IntegrationRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !integrationLimiter.isAllowed("integration:" + c.ClientIP()) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Demasiadas peticiones seguidas. Reintenta en un minuto.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
