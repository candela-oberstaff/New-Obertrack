package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

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

	email := api.Group("/email")
	email.Use(middleware.RequireSuperadmin())
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
		email.GET("/campaigns/:id/events", d.email.GetCampaignEvents)
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
		surveys.POST("", middleware.RequireSuperadmin(), d.survey.CreateSurvey)
		surveys.GET("", middleware.RequireSuperadmin(), d.survey.GetSurveys)
		surveys.PUT("/:id", middleware.RequireSuperadmin(), d.survey.UpdateSurvey)
		surveys.DELETE("/:id", middleware.RequireSuperadmin(), d.survey.DeleteSurvey)
		surveys.POST("/:id/send", middleware.RequireSuperadmin(), d.survey.SendSurvey)
		surveys.GET("/:id", d.survey.GetSurvey)
		surveys.POST("/:id/responses", d.survey.SubmitResponse)
	}

	tutorials := api.Group("/tutorials")
	{
		tutorials.GET("", d.tutorial.GetAll)
		tutorials.GET("/views", d.tutorial.GetMyViews)
		tutorials.GET("/:id", d.tutorial.GetByID)
		tutorials.POST("", middleware.RequireSuperadmin(), d.tutorial.Create)
		tutorials.POST("/reorder", middleware.RequireSuperadmin(), d.tutorial.Reorder)
		tutorials.POST("/:id/view", d.tutorial.RecordView)
		tutorials.PUT("/:id", middleware.RequireSuperadmin(), d.tutorial.Update)
		tutorials.DELETE("/:id", middleware.RequireSuperadmin(), d.tutorial.Delete)
	}

	tickets := api.Group("/tickets")
	tickets.Use(requireSupportInboxAccess())
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
		tickets.GET("/internal/report", d.ticket.GetRejectionReport)
		tickets.GET("/internal/:id", d.ticket.GetInternalTicket)
		tickets.PUT("/internal/:id", d.ticket.UpdateInternalTicket)
		tickets.POST("/internal/:id/notes", d.ticket.AddInternalNote)
		tickets.POST("/internal/:id/transfer", d.ticket.TransferInternalTicket)
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
		chats.GET("/:ticketId/messages", d.whatsapp.GetMessages)
		chats.PATCH("/:ticketId/assign", d.whatsapp.AssignToMe)
		chats.POST("/:ticketId/send", d.whatsapp.SendMessage)
		chats.POST("/sync-agent", d.whatsapp.SyncAgentID)
	}
}
