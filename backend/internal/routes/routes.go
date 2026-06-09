package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/middleware"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	d := buildDeps(db, cfg)

	api := r.Group("/api")
	{
		registerPublicRoutes(api, d)

		api.Use(middleware.AuthMiddleware(cfg.JWTSecret, d.tvGetter))
		api.Use(middleware.AuditMiddleware(d.auditSvc))
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

	r.GET("/ws/chat", auth, d.chat.HandleWebSocket)
	r.GET("/ws/channels", auth, d.channel.HandleWebSocket)
	r.GET("/ws/notifications", auth, d.notification.HandleWebSocket)
}
