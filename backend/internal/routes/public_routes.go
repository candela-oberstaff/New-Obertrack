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

	// Integración Obersuite (captación) → Obertrack (gestión). Protegida con un
	// token de servicio estático compartido (OBERSUITE_SERVICE_TOKEN, header
	// X-Service-Token), no con sesión de usuario. Fail-closed si el token no
	// está configurado (mismo patrón que los webhooks de arriba).
	obersuite := api.Group("/integrations/obersuite")
	obersuite.Use(middleware.SharedSecretAuth("OBERSUITE_SERVICE_TOKEN", "X-Service-Token"))
	obersuite.Use(middleware.IntegrationRateLimitMiddleware())
	{
		// Qué está desplegado. Sin sesión (va dentro del grupo con token) para
		// que Obersuite pueda comprobar si un cambio nuestro ya llegó a
		// producción en vez de deducirlo del JSON que recibe.
		obersuite.GET("/version", d.version.Version)
		// Lista de empresas para el dropdown de contratación en Obersuite.
		obersuite.GET("/companies", d.onboarding.ListCompanies)
		// Webhook de contratación: crea/actualiza el profesional y abre su empleo.
		obersuite.POST("/hire", d.onboarding.Hire)

		// Ficha de una empresa, un bloque por pestaña de la pantalla de
		// Obersuite. Van sueltos y no en una respuesta con todo dentro porque
		// las pestañas se abren de una en una: la que nadie abre no se pide.
		empresa := obersuite.Group("/companies/:id")
		{
			empresa.GET("", d.obersuiteCompany.Detail)
			// Alimenta "Profesionales" y "Horarios": la jornada de cada
			// persona viaja en su propia fila, no hace falta otra consulta.
			empresa.GET("/professionals", d.obersuiteCompany.Professionals)
			empresa.GET("/org-chart", d.obersuiteCompany.OrgChart)
			// OJO: el expediente es la cronología. La pestaña que ellos
			// llaman "Actividad" es la de abajo, no esta.
			empresa.GET("/timeline", d.obersuiteCompany.Timeline)
			empresa.GET("/attention", d.obersuiteCompany.Attention)
			empresa.GET("/tickets", d.obersuiteCompany.Tickets)
			empresa.GET("/archived", d.obersuiteCompany.Archived)
			empresa.GET("/usage", d.obersuiteCompany.Usage)

			// Ficha de UNA persona dentro de la empresa: el espejo de nuestra
			// pantalla de empleado, una ruta por pestaña. Cuelga de la empresa
			// porque el expediente, las jornadas y las tareas son del EMPLEO,
			// y alguien con dos empresas tiene dos de cada.
			persona := empresa.Group("/professionals/:uid")
			{
				persona.GET("", d.obersuiteProfessional.Detail)
				persona.GET("/record", d.obersuiteProfessional.Record)
				persona.GET("/workdays", d.obersuiteProfessional.Workdays)
				persona.GET("/tasks", d.obersuiteProfessional.Tasks)
				persona.GET("/onboarding", d.obersuiteProfessional.Onboarding)
				persona.GET("/support", d.obersuiteProfessional.Support)
				persona.GET("/activity", d.obersuiteProfessional.Activity)
			}
		}
	}

	// Callback de OAuth de Google (integración con Calendar). Público a
	// propósito: llega como navegación del navegador desde accounts.google.com,
	// y la identidad NO sale de la sesión sino del state firmado. Así el flujo
	// funciona aunque el access token del usuario haya expirado durante el
	// consentimiento, y sirve igual para el deep link de la app móvil.
	api.GET("/integrations/google/callback", d.googleCal.Callback)

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

	// Inducción del profesional recién contratado. Es PÚBLICA a propósito: la
	// persona todavía no tiene cuenta activa, su única credencial es el token
	// del enlace que recibió por correo. Se limita la tasa para que el token no
	// sea adivinable por fuerza bruta.
	induction := api.Group("/induction")
	induction.Use(middleware.AuthRateLimitMiddleware())
	{
		induction.GET("/:token", d.induction.Landing)
		induction.POST("/:token/submit", d.induction.Submit)
	}

	// Testimonios. PÚBLICA a propósito, igual que la inducción: quien firma
	// puede no tener sesión (una empresa que delega en su gerente, un
	// profesional cuyo empleo ya terminó) y su única credencial es el token del
	// enlace que recibió por correo. Se limita la tasa para que el token no sea
	// adivinable por fuerza bruta.
	//
	// El prefijo es "/testimonial" (singular) para no chocar con el grupo
	// interno "/testimonials", que vive bajo sesión.
	testimonial := api.Group("/testimonial")
	testimonial.Use(middleware.AuthRateLimitMiddleware())
	{
		testimonial.GET("/:token", d.testimonial.Landing)
		testimonial.POST("/:token/submit", d.testimonial.Submit)
	}

	// Vista previa de plantillas de correo — SOLO en desarrollo. Permite iterar
	// el diseño de los correos sin enviarlos. En release no se registra: expone
	// la estructura de correos internos.
	if os.Getenv("GIN_MODE") != "release" {
		devPreview := api.Group("/dev/email-preview")
		{
			devPreview.GET("", d.emailPreview.Index)
			devPreview.GET("/:slug", d.emailPreview.Show)
		}
	}

	// Public survey quick response.
	api.GET("/surveys/:id/quick-response", d.survey.QuickResponse)

	// Public file serving (no auth required) — used for email client image loading.
	api.GET("/public/uploads/:filename", d.upload.GetPublicFile)
}
