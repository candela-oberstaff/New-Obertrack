package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Alcances de las sesiones embebidas que emite el canje del CRM (contrato CRM,
// §9): cada una abre una sola vista.
const (
	ScopeTasks = "tasks" // vista de Tareas
	ScopeHours = "hours" // vista de Horas
)

// ScopeAllowlist dice, por alcance, qué rutas puede usar una sesión acotada.
// La clave interior es "MÉTODO patrón", con el patrón tal como lo devuelve
// c.FullPath() (p. ej. "GET /api/tasks/:id").
type ScopeAllowlist map[string]map[string]bool

// GetScope devuelve el alcance de la sesión, o "" si es una sesión completa.
func GetScope(c *gin.Context) string {
	return c.GetString("scope")
}

// RequireScope deja pasar sin mirar a las sesiones completas y, a las acotadas,
// solo a las rutas de su lista. Es fail-closed: un alcance que no está en la
// lista, o una ruta sin patrón (404 de Gin), se rechazan con 403.
//
// Va justo después de AuthMiddleware, para que ninguna ruta protegida quede
// fuera de su control.
func RequireScope(allow ScopeAllowlist) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope := GetScope(c)
		if scope == "" {
			c.Next()
			return
		}
		routes, ok := allow[scope]
		if !ok || c.FullPath() == "" || !routes[c.Request.Method+" "+c.FullPath()] {
			c.JSON(http.StatusForbidden, gin.H{"error": "Esta sesión no tiene acceso a este recurso"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RejectScoped cierra una ruta a cualquier sesión acotada. Se usa en los
// websockets: la vista embebida refresca por intervalo y no abre sockets.
func RejectScoped() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetScope(c) != "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Esta sesión no tiene acceso a este recurso"})
			c.Abort()
			return
		}
		c.Next()
	}
}
