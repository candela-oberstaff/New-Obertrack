package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/middleware"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	d := buildDeps(db, cfg)
	// Volcado periódico del contador de uso (ver ActivityMiddleware).
	d.activitySvc.Start()
	mountRoutes(r, d)
}

// mountRoutes cuelga todas las rutas de unas dependencias ya construidas. Va
// aparte de RegisterRoutes para que los tests puedan recorrer el router real
// (p. ej. scope_routes_test.go) sin base de datos.
func mountRoutes(r *gin.Engine, d *deps) {
	api := r.Group("/api")
	{
		registerPublicRoutes(api, d)

		api.Use(middleware.AuthMiddleware(d.cfg.JWTSecret, d.tvGetter))
		// Sesiones acotadas (acceso embebido desde el CRM): solo su lista de
		// rutas. Va pegado a la autenticación para que nada quede fuera.
		api.Use(middleware.RequireScope(scopeAllowlist))
		api.Use(middleware.AuditMiddleware(d.auditSvc))
		// Contador de uso real (pestaña Uso de Métricas). Va después de la
		// auditoría y solo suma en memoria: la auditoría dice QUÉ cambió
		// alguien —solo escrituras—, y esto dice si la app se usa, que incluye
		// a quien entra únicamente a mirar.
		api.Use(middleware.ActivityMiddleware(d.activitySvc))
		{
			registerAccountRoutes(api, d)
			registerWorkRoutes(api, d)
			registerMessagingRoutes(api, d)
			registerPlatformRoutes(api, d)
		}
	}

	registerWebSocketRoutes(r, d)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func registerWebSocketRoutes(r *gin.Engine, d *deps) {
	auth := middleware.AuthMiddleware(d.cfg.JWTSecret, d.tvGetter)
	// Ningún socket admite sesiones acotadas: la vista embebida refresca por
	// intervalo (contrato CRM, §9).
	noScoped := middleware.RejectScoped()

	r.GET("/ws/chat", auth, noScoped, d.chat.HandleWebSocket)
	r.GET("/ws/channels", auth, noScoped, d.channel.HandleWebSocket)
	r.GET("/ws/notifications", auth, noScoped, d.notification.HandleWebSocket)
}
