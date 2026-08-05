package routes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

func requireSupportInboxAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Support inbox access required"})
		c.Abort()
	}
}

// registerPlatformRoutes wires cross-cutting platform features: file uploads,
// email marketing, surveys, tutorials and the support-ticket inbox.
func registerPlatformRoutes(api *gin.RouterGroup, d *deps) {
	uploads := api.Group("/uploads")
	{
		uploads.POST("", d.upload.UploadFile)
		// Download runs authenticated so ownership is verified (audit A-06).
		uploads.GET("/:filename", d.upload.GetFile)
	}

	// Tools (email marketing y encuestas): superadmins y customer success.
	email := api.Group("/email")
	email.Use(requireSupportInboxAccess())
	{
		// Catálogo de variables de personalización ({{nombre}}, {{empresa}}, ...).
		email.GET("/variables", d.email.GetVariables)
		// Copia de prueba a uno mismo, por el mismo camino que el envío real.
		email.POST("/test-send", d.email.SendTestEmail)
		email.GET("/templates", d.email.GetTemplates)
		email.POST("/templates", d.email.CreateTemplate)
		email.PUT("/templates/:id", d.email.UpdateTemplate)
		email.DELETE("/templates/:id", d.email.DeleteTemplate)
		email.GET("/campaigns", d.email.GetCampaigns)
		email.POST("/campaigns", d.email.CreateCampaign)
		email.PUT("/campaigns/:id", d.email.UpdateCampaign)
		email.DELETE("/campaigns/:id", d.email.DeleteCampaign)
		email.POST("/campaigns/:id/send", d.email.SendCampaign)
		email.GET("/campaigns/:id/events", d.email.GetCampaignEvents)
		// One-off transactional sends (from tenant / employee detail views)
		email.POST("/quick-send", d.email.SendQuickEmail)
		email.POST("/quick-send-bulk", d.email.SendQuickEmailBulk)
		email.POST("/templates/:id/send", d.email.SendTemplate)
	}

	audiences := api.Group("/audiences")
	audiences.Use(middleware.RequireSuperadmin())
	{
		audiences.GET("/groups", d.audience.GetGroups)
		audiences.POST("/groups", d.audience.CreateGroup)
		audiences.GET("/groups/:id", d.audience.GetGroupByID)
		audiences.PUT("/groups/:id", d.audience.UpdateGroup)
		audiences.DELETE("/groups/:id", d.audience.DeleteGroup)
		audiences.POST("/groups/:id/members", d.audience.AddMember)
		audiences.DELETE("/groups/:id/members/:userId", d.audience.RemoveMember)
	}

	surveys := api.Group("/surveys")
	{
		surveys.POST("", requireSupportInboxAccess(), d.survey.CreateSurvey)
		surveys.GET("", requireSupportInboxAccess(), d.survey.GetSurveys)
		surveys.PUT("/:id", requireSupportInboxAccess(), d.survey.UpdateSurvey)
		surveys.DELETE("/:id", requireSupportInboxAccess(), d.survey.DeleteSurvey)
		surveys.POST("/:id/send", requireSupportInboxAccess(), d.survey.SendSurvey)
		surveys.GET("/:id", d.survey.GetSurvey)
		surveys.POST("/:id/responses", d.survey.SubmitResponse)
	}

	// Inducción del profesional recién contratado (video de Novedades +
	// cuestionario calificado de Encuestas). La configuración global es solo de
	// superadmin; consultar el detalle y reiniciar los intentos es la acción de
	// Soporte tras contactar a quien no aprobó.
	//
	// El prefijo es "/inductions" (plural) para no chocar con el comodín de la
	// landing pública, que vive en /api/induction/:token.
	inductions := api.Group("/inductions")
	inductions.Use(requireSupportInboxAccess())
	{
		inductions.GET("/config", d.induction.GetConfig)
		inductions.PUT("/config", middleware.RequireSuperadmin(), d.induction.SaveConfig)
		inductions.GET("/users/:userId", d.induction.Status)
		inductions.POST("/users/:userId/reset", d.induction.Reset)
		// Enviar la inducción a un profesional que ya existe (alta manual,
		// alta desde la empresa o importación).
		inductions.POST("/users/:userId/invite", d.induction.Invite)
	}

	// Módulo "tutorials": ver requiere al menos "view"; la gestión sigue siendo
	// solo de superadmins (que no se restringen por roles).
	tutorialsView := handlers.RequirePermission(d.rbacSvc, "tutorials", models.PermissionView)

	// Novedades queda fuera del alcance de Customer Success. Se bloquea en el
	// grupo entero: si solo se ocultara el enlace del menú, la URL directa y la
	// API seguirían respondiendo.
	tutorials := api.Group("/tutorials")
	tutorials.Use(middleware.BlockCustomerSuccess())
	{
		tutorials.GET("", tutorialsView, d.tutorial.GetAll)
		tutorials.GET("/views", tutorialsView, d.tutorial.GetMyViews)
		tutorials.GET("/:id", tutorialsView, d.tutorial.GetByID)
		tutorials.POST("", middleware.RequireSuperadmin(), d.tutorial.Create)
		tutorials.POST("/reorder", middleware.RequireSuperadmin(), d.tutorial.Reorder)
		tutorials.POST("/:id/view", tutorialsView, d.tutorial.RecordView)
		tutorials.PUT("/:id", middleware.RequireSuperadmin(), d.tutorial.Update)
		tutorials.DELETE("/:id", middleware.RequireSuperadmin(), d.tutorial.Delete)
	}

	// Roles personalizados y grupos (equipos) por empresa. El superadmin opera
	// con ?company_id=; las cuentas empresa quedan acotadas a su propio tenant.
	rbac := api.Group("")
	rbac.Use(handlers.RequireRBACManager())
	{
		rbac.GET("/roles", d.rbac.ListRoles)
		rbac.POST("/roles", d.rbac.CreateRole)
		rbac.PUT("/roles/:id", d.rbac.UpdateRole)
		rbac.DELETE("/roles/:id", d.rbac.DeleteRole)
		rbac.GET("/roles/:id/users", d.rbac.GetRoleUsers)
		rbac.POST("/roles/:id/users", d.rbac.AssignRole)
		rbac.DELETE("/roles/:id/users", d.rbac.UnassignRole)

		rbac.GET("/rbac/users/:userId", d.rbac.GetUserRBAC)

		rbac.GET("/groups", d.rbac.ListGroups)
		rbac.POST("/groups", d.rbac.CreateGroup)
		rbac.PUT("/groups/:id", d.rbac.UpdateGroup)
		rbac.DELETE("/groups/:id", d.rbac.DeleteGroup)
		rbac.GET("/groups/:id/members", d.rbac.GetGroupMembers)
		rbac.POST("/groups/:id/members", d.rbac.AddGroupMember)
		rbac.DELETE("/groups/:id/members", d.rbac.RemoveGroupMember)
	}

	// Módulo de tickets (Zoho + WhatsApp): solo superadmin. Customer success y
	// profesionales no tienen acceso.
	tickets := api.Group("/tickets")
	tickets.Use(requireSupportInboxAccess())
	{
		tickets.GET("/waha/status", func(c *gin.Context) {
			status, err := d.wahaSvc.GetSessionStatusAndQR(d.wahaSvc.GetSession())
			if err != nil {
				// El error interno se registra pero no se devuelve: incluye la URL
				// interna de la instancia de WAHA.
				log.Printf("[WAHA] status check failed: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar el estado de WhatsApp"})
				return
			}
			c.JSON(http.StatusOK, status)
		})
		// Forzar conexión: (re)arranca la sesión en WAHA y devuelve el estado/QR
		// fresco. Útil cuando la sesión se cae (SCAN_QR/FAILED) y hay que
		// re-vincularla sin entrar al dashboard de WAHA.
		tickets.POST("/waha/start", func(c *gin.Context) {
			session := d.wahaSvc.GetSession()
			if err := d.wahaSvc.StartSession(session); err != nil {
				log.Printf("[WAHA] session start failed: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo iniciar la sesión de WhatsApp"})
				return
			}
			status, err := d.wahaSvc.GetSessionStatusAndQR(session)
			if err != nil {
				log.Printf("[WAHA] status check after start failed: %v", err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo consultar el estado de WhatsApp"})
				return
			}
			c.JSON(http.StatusOK, status)
		})
		// Traída manual del historial: la salida cuando el webhook no está
		// entregando y la bandeja se queda atrás.
		tickets.POST("/waha/sync", d.ticket.SyncWhatsAppHistory)
		// Privacidad: inventario y borrado definitivo de las conversaciones de una
		// sesión. El borrado exige superadmin (se valida en el handler).
		tickets.GET("/waha/sessions", d.ticket.ListWhatsAppSessions)
		tickets.DELETE("/waha/sessions/:session", d.ticket.PurgeWhatsAppSession)
		tickets.GET("/statuses", d.ticket.GetTicketStatuses)
		tickets.GET("/agents", d.ticket.GetSupportAgents)
		tickets.GET("/zoho-agents", d.ticket.GetZohoAgents)
		tickets.GET("/transfers", d.ticket.GetTicketTransfers)
		tickets.GET("", d.ticket.GetTickets)
		tickets.GET("/internal/report", d.ticket.GetRejectionReport)
		tickets.GET("/internal/:id", d.ticket.GetInternalTicket)
		tickets.PUT("/internal/:id", d.ticket.UpdateInternalTicket)
		tickets.POST("/internal/:id/notes", d.ticket.AddInternalNote)
		tickets.POST("/internal/:id/transfer", d.ticket.TransferInternalTicket)
		tickets.GET("/wa", d.ticket.ListWhatsAppTickets)
		// Estático antes que el paramétrico, igual que /internal/report.
		tickets.GET("/wa/lookup", d.ticket.LookupWhatsAppChat)
		tickets.GET("/wa/:id", d.ticket.GetWhatsAppTicket)
		tickets.POST("/wa/:id/messages", d.ticket.SendWhatsAppMessage)
		tickets.PATCH("/wa/:id", d.ticket.UpdateWhatsAppTicket)
		tickets.GET("/:id", d.ticket.GetTicket)
		tickets.PUT("/:id", d.ticket.UpdateTicket)
		tickets.POST("/:id/messages", d.ticket.SendMessage)
		tickets.POST("/:id/transfer", d.ticket.TransferZohoTicket)
	}

	chats := api.Group("/chats")
	chats.Use(requireSupportInboxAccess())
	{
		chats.GET("/me", d.whatsapp.GetMyChats)
		chats.GET("/unassigned", d.whatsapp.GetUnassignedChats)
		chats.GET("/templates", d.whatsapp.GetTemplates)
		chats.GET("/:ticketId/messages", d.whatsapp.GetMessages)
		chats.PATCH("/:ticketId/assign", d.whatsapp.AssignToMe)
		chats.POST("/:ticketId/send", d.whatsapp.SendMessage)
		chats.POST("/sync-agent", d.whatsapp.SyncAgentID)
	}

}
