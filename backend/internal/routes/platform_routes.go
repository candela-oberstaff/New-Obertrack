package routes

import (
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

// requireSupportManager limita acciones de gestión del soporte (reporte de
// rechazos y transferencias de tickets) a superadmins y Customer Success
// Managers (customer_success con flag de manager); los CS analistas operan
// sus propios tickets pero no gestionan al equipo.
func requireSupportManager() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) ||
			(middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess) && middleware.IsManager(c)) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere permisos de Customer Success Manager"})
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
		email.GET("/templates", d.email.GetTemplates)
		email.POST("/templates", d.email.CreateTemplate)
		email.PUT("/templates/:id", d.email.UpdateTemplate)
		email.DELETE("/templates/:id", d.email.DeleteTemplate)
		email.GET("/campaigns", d.email.GetCampaigns)
		email.POST("/campaigns", d.email.CreateCampaign)
		email.PUT("/campaigns/:id", d.email.UpdateCampaign)
		email.DELETE("/campaigns/:id", d.email.DeleteCampaign)
		email.POST("/campaigns/:id/send", d.email.SendCampaign)
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

	// Módulo "tutorials": ver requiere al menos "view"; la gestión sigue siendo
	// solo de superadmins (que no se restringen por roles).
	tutorialsView := handlers.RequirePermission(d.rbacSvc, "tutorials", models.PermissionView)

	tutorials := api.Group("/tutorials")
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

	// Módulo "tickets": aplica sobre el acceso de soporte existente (los
	// customer success con roles pueden quedar en solo lectura).
	ticketsView := handlers.RequirePermission(d.rbacSvc, "tickets", models.PermissionView)
	ticketsEdit := handlers.RequirePermission(d.rbacSvc, "tickets", models.PermissionEdit)

	tickets := api.Group("/tickets")
	tickets.Use(requireSupportInboxAccess(), ticketsView)
	{
		tickets.GET("/waha/status", func(c *gin.Context) {
			status, err := d.wahaSvc.GetSessionStatusAndQR("default")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, status)
		})
		tickets.GET("/statuses", d.ticket.GetTicketStatuses)
		tickets.GET("/agents", d.ticket.GetSupportAgents)
		tickets.GET("/zoho-agents", d.ticket.GetZohoAgents)
		tickets.GET("/transfers", d.ticket.GetTicketTransfers)
		tickets.GET("", d.ticket.GetTickets)
		tickets.GET("/internal/report", requireSupportManager(), d.ticket.GetRejectionReport)
		tickets.GET("/internal/:id", d.ticket.GetInternalTicket)
		tickets.PUT("/internal/:id", ticketsEdit, d.ticket.UpdateInternalTicket)
		tickets.POST("/internal/:id/notes", ticketsEdit, d.ticket.AddInternalNote)
		tickets.POST("/internal/:id/transfer", ticketsEdit, requireSupportManager(), d.ticket.TransferInternalTicket)
		tickets.GET("/:id", d.ticket.GetTicket)
		tickets.PUT("/:id", ticketsEdit, d.ticket.UpdateTicket)
		tickets.POST("/:id/messages", ticketsEdit, d.ticket.SendMessage)
		tickets.POST("/:id/transfer", ticketsEdit, requireSupportManager(), d.ticket.TransferZohoTicket)
	}

	chats := api.Group("/chats")
	chats.Use(requireSupportInboxAccess(), ticketsView)
	{
		chats.GET("/me", d.whatsapp.GetMyChats)
		chats.GET("/unassigned", d.whatsapp.GetUnassignedChats)
		chats.GET("/:ticketId/messages", d.whatsapp.GetMessages)
		chats.PATCH("/:ticketId/assign", ticketsEdit, d.whatsapp.AssignToMe)
		chats.POST("/:ticketId/send", ticketsEdit, d.whatsapp.SendMessage)
		chats.POST("/sync-agent", ticketsEdit, d.whatsapp.SyncAgentID)
	}
}
