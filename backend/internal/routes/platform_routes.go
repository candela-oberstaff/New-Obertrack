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
		tickets.GET("", d.ticket.GetTickets)
		tickets.GET("/:id", d.ticket.GetTicket)
		tickets.PUT("/:id", d.ticket.UpdateTicket)
		tickets.POST("/:id/messages", d.ticket.SendMessage)
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
