package routes

import (
	"os"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
)

// registerPublicRoutes wires every endpoint that must be reachable without an
// access token: auth entry points, provider webhooks, the seed bootstrap and the
// public survey quick-response.
func registerPublicRoutes(api *gin.RouterGroup, d *deps) {
	auth := api.Group("/auth")
	{
		auth.POST("/register", d.auth.Register)
		// Stricter rate limit on credential endpoints (audit finding A-05).
		auth.POST("/login", middleware.AuthRateLimitMiddleware(), d.auth.Login)
		auth.GET("/companies", d.auth.GetCompanies)
		auth.POST("/forgot-password", middleware.AuthRateLimitMiddleware(), d.auth.ForgotPassword)
		auth.POST("/reset-password", middleware.AuthRateLimitMiddleware(), d.auth.ResetPassword)
		// Refresh / logout read the refresh cookie — no access token required.
		auth.POST("/refresh", d.auth.Refresh)
		auth.POST("/logout", d.auth.Logout)
	}

	// Webhooks — each authenticated with a provider-specific shared secret /
	// signature (audit finding C-03). Without the secret configured they fail
	// closed (503).
	api.POST("/webhooks/brevo",
		middleware.SharedSecretAuth("BREVO_WEBHOOK_TOKEN", "X-Webhook-Token"),
		d.email.HandleBrevoWebhook)
	api.POST("/webhooks/brevo/inbound",
		middleware.SharedSecretAuth("BREVO_WEBHOOK_TOKEN", "X-Webhook-Token"),
		d.brevoInbound.HandleInbound)
	api.POST("/webhooks/waha",
		middleware.WahaHMACAuth(),
		d.waha.HandleWebhook)

	// Administrative seed routes — bootstrap only. Require BOTH a non-release
	// build AND a secret bootstrap token (audit finding C-01).
	if os.Getenv("GIN_MODE") != "release" {
		seed := api.Group("/seed")
		seed.Use(middleware.SharedSecretAuth("SEED_BOOTSTRAP_TOKEN", "X-Seed-Token"))
		{
			seed.POST("/superadmin", d.admin.CreateSuperAdmin)
			seed.POST("/reset-superadmin", d.admin.ResetSuperAdmin)
			seed.POST("/make-superadmin/:email", d.admin.MakeSuperAdmin)
			seed.POST("/create-superadmin", d.admin.CreateSuperAdminForced)
		}
	}

	// Public survey quick response.
	api.GET("/surveys/:id/quick-response", d.survey.QuickResponse)
}
