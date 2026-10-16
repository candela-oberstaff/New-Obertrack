package routes

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/mcp"
	"github.com/obertrack/backend/internal/middleware"
)

// Rutas del servidor MCP de solo lectura (asistentes de IA). Ver
// docs/features/mcp-asistentes-ia.md.

// registerMCPWellKnown cuelga la metadata OAuth en la raíz del dominio
// (/.well-known/…), fuera de /api. nginx la reenvía al backend.
func registerMCPWellKnown(r *gin.Engine, d *deps) {
	for _, p := range []string{
		"/.well-known/oauth-authorization-server",
		"/.well-known/oauth-authorization-server/api/mcp",
	} {
		r.GET(p, d.mcp.WellKnownAuthorizationServer)
		r.OPTIONS(p, mcp.WellKnownOptions)
	}
	for _, p := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/api/mcp",
	} {
		r.GET(p, d.mcp.WellKnownProtectedResource)
		r.OPTIONS(p, mcp.WellKnownOptions)
	}
}

// registerMCPPublicRoutes: /api/mcp y el Authorization Server. Son públicas
// para la sesión web porque su autenticación es el Bearer o el propio flujo
// OAuth; van antes de AuthMiddleware.
func registerMCPPublicRoutes(api *gin.RouterGroup, d *deps) {
	mcpLimit := middleware.KeyedRateLimit("mcp", 240, time.Minute)
	api.POST("/mcp", mcpLimit, d.mcp.MCP)
	api.GET("/mcp", mcpLimit, d.mcp.MCP)
	api.DELETE("/mcp", mcpLimit, d.mcp.MCP)

	oauth := api.Group("/mcp/oauth")
	oauth.POST("/register", middleware.KeyedRateLimit("mcp-register", 20, time.Hour), d.mcp.Register)
	oauth.GET("/authorize", middleware.KeyedRateLimit("mcp-authorize", 60, time.Minute), d.mcp.AuthorizeGet)
	oauth.POST("/authorize", middleware.KeyedRateLimit("mcp-authorize-post", 30, time.Minute), d.mcp.AuthorizePost)
	oauth.POST("/token", middleware.KeyedRateLimit("mcp-token", 60, time.Minute), d.mcp.Token)
	oauth.POST("/revoke", middleware.KeyedRateLimit("mcp-revoke", 60, time.Minute), d.mcp.Revoke)
}

// registerMCPRoutes: rutas de la web con sesión. Conexiones y tokens exigen
// el permiso de uso del rol; el panel "Uso de IA" es solo para superadmin.
func registerMCPRoutes(api *gin.RouterGroup, d *deps) {
	api.GET("/mcp/status", d.mcp.Status)

	use := d.mcp.RequireMCPUse()
	api.GET("/mcp/connections", use, d.mcp.ListMyConnections)
	api.DELETE("/mcp/connections/:id", use, d.mcp.RevokeMine)
	api.GET("/mcp/tokens", use, d.mcp.ListMyTokens)
	api.POST("/mcp/tokens", use, d.mcp.CreateToken)
	api.DELETE("/mcp/tokens/:id", use, d.mcp.RevokeMine)

	admin := api.Group("/admin/mcp", middleware.RequireSuperadmin())
	admin.GET("/overview", d.mcp.AdminOverview)
	admin.GET("/connections", d.mcp.AdminConnections)
	admin.POST("/connections/:id/revoke", d.mcp.AdminRevoke)
	admin.GET("/usage", d.mcp.AdminUsage)
	admin.PUT("/quotas/:role", d.mcp.AdminUpdateQuota)
	admin.PUT("/settings", d.mcp.AdminUpdateSettings)
	admin.POST("/alerts/seen", d.mcp.AdminMarkAlertsSeen)
	admin.POST("/purge", d.mcp.AdminPurge)
}
