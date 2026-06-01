package routes

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/websocket"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Repositories
	userRepo := repository.NewUserRepository(db)
	chatRepo := repository.NewChatRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	channelRepo := repository.NewChannelRepository(db)
	workHourRepo := repository.NewWorkHourRepository(db)
	emailRepo := repository.NewEmailRepository(db)
	surveyRepo := repository.NewSurveyRepository(db)
	metricsRepo := repository.NewMetricsRepository(db)

	// Services
	userSvc := service.NewUserService(userRepo)
	notifSvc := service.NewNotificationService(notifRepo)
	chatSvc := service.NewChatService(chatRepo)
	channelSvc := service.NewChannelService(channelRepo, userRepo, notifSvc)

	// Initialize Google Chat & Brevo
	googleChatSvc := service.NewGoogleChatService()
	brevoSvc := service.NewBrevoService()

	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, brevoSvc)

	workHourSvc := service.NewWorkHourService(workHourRepo, userRepo, notifSvc, googleChatSvc, brevoSvc)
	uploadSvc := service.NewUploadService(os.Getenv("UPLOAD_PATH"))

	boardRepo := repository.NewBoardRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	taskSvc := service.NewTaskService(taskRepo, userRepo, boardRepo, notifSvc, googleChatSvc)
	adminRepo := repository.NewAdminRepository(db)
	adminSvc := service.NewAdminService(adminRepo, userRepo, taskRepo, workHourRepo)
	boardSvc := service.NewBoardService(boardRepo, userRepo)
	// Handlers
	googleChatHandler := handlers.NewGoogleChatHandler(workHourSvc, userSvc)
	authHandler := handlers.NewAuthHandler(authSvc)
	notificationHandler := handlers.NewNotificationHandler(notifSvc)
	userHandler := handlers.NewUserHandler(userSvc)
	taskHandler := handlers.NewTaskHandler(taskSvc)
	adminHandler := handlers.NewAdminHandler(adminSvc)
	boardHandler := handlers.NewBoardHandler(boardSvc)
	workHourHandler := handlers.NewWorkHourHandler(workHourSvc)
	uploadHandler := handlers.NewUploadHandler(uploadSvc, os.Getenv("UPLOAD_PATH"))
	emailHandler := handlers.NewEmailHandler(emailRepo, brevoSvc)
	surveyHandler := handlers.NewSurveyHandler(surveyRepo, userRepo, brevoSvc, notifSvc)
	metricsHandler := handlers.NewMetricsHandler(metricsRepo)
	wahaSvc := service.NewWahaService()
	wahaHandler := handlers.NewWahaHandler(db, wahaSvc)
	zohoSvc := service.NewZohoService()

	// Start background WAHA Contact Synchronizer (runs every 5 minutes)
	contactSyncSvc := service.NewContactSyncService(db, wahaSvc)
	contactSyncSvc.Start(5 * time.Minute)

	// WebSocket hubs
	chatHub := websocket.NewChatHub(func(msg websocket.ChatWSMessage) {
		// Chat message handler - persist if needed
	})
	channelHub := websocket.NewChannelHub(func(msg websocket.ChannelWSMessage) {
		// Channel message handler - persist if needed
	})
	go chatHub.Run()
	go channelHub.Run()

	// Chat and Channel handlers with WebSocket hubs
	chatHandler := handlers.NewChatHandler(chatSvc, chatHub)
	channelHandler := handlers.NewChannelHandler(channelSvc, channelHub)

	api := r.Group("/api")
	{


		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/companies", authHandler.GetCompanies)
			auth.POST("/forgot-password", authHandler.ForgotPassword)
			auth.POST("/reset-password", authHandler.ResetPassword)
		}

		// Webhooks
		brevoInboundHandler := handlers.NewBrevoInboundHandler(db)
		api.POST("/webhooks/brevo", emailHandler.HandleBrevoWebhook)
		api.POST("/webhooks/brevo/inbound", brevoInboundHandler.HandleInbound)
		api.POST("/webhooks/waha", wahaHandler.HandleWebhook)

		// Google Chat Public Callback
		api.POST("/google-chat/callback", googleChatHandler.HandleCallback)

		// Administrative seed routes - should be disabled in production or strictly protected
		if os.Getenv("GIN_MODE") != "release" {
			seed := api.Group("/seed")
			{
				seed.POST("/superadmin", adminHandler.CreateSuperAdmin)
				seed.POST("/reset-superadmin", adminHandler.ResetSuperAdmin)
				seed.POST("/make-superadmin/:email", adminHandler.MakeSuperAdmin)
				seed.POST("/create-superadmin", adminHandler.CreateSuperAdminForced)
			}
		}

		// Public Survey Quick Response
		api.GET("/surveys/:id/quick-response", surveyHandler.QuickResponse)

		api.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		{
			auth := api.Group("/auth")
			{
				auth.GET("/me", authHandler.Me)
			}

			users := api.Group("/users")
			{
				users.GET("", userHandler.GetAll)
				users.GET("/employees", userHandler.GetEmployees)
				users.GET("/my-team", userHandler.GetMyTeam)
				users.GET("/:id", userHandler.GetByID)
				users.PUT("/:id", userHandler.Update)
				users.POST("/:id/toggle-status", userHandler.ToggleStatus)
				users.POST("/:id/promote-manager", userHandler.PromoteToManager)
				users.POST("/:id/assign-manager", userHandler.AssignToManager)
				users.POST("/:id/change-password", userHandler.ChangePassword)
			}

			admin := api.Group("/admin")
			admin.Use(middleware.RequireSuperadmin())
			{
				admin.GET("/dashboard", adminHandler.GetDashboard)
				admin.GET("/companies", adminHandler.GetCompanies)
				admin.GET("/inactive-users", adminHandler.GetInactiveUsers)
				admin.GET("/recent-activity", adminHandler.GetRecentActivity)
				admin.GET("/stats", adminHandler.GetStats)
				admin.GET("/users", adminHandler.GetAllUsers)
				admin.POST("/users", adminHandler.CreateUser)
				admin.PUT("/users/:id", adminHandler.UpdateUser)
				admin.DELETE("/users/:id", adminHandler.DeleteUser)
				admin.POST("/users/:id/reset-password", adminHandler.ResetPassword)
			}

			boards := api.Group("/boards")
			{
				boards.GET("", boardHandler.GetAll)
				boards.POST("", boardHandler.Create)
				boards.GET("/public", boardHandler.GetPublicBoards)
				boards.POST("/:id/join", boardHandler.JoinBoard)
				boards.GET("/:id", boardHandler.GetByID)
				boards.PUT("/:id", boardHandler.Update)
				boards.DELETE("/:id", boardHandler.Delete)
				boards.POST("/:id/phases", boardHandler.AddPhase)
				boards.DELETE("/:id/phases/:phaseId", boardHandler.RemovePhase)
				boards.PUT("/:id/phases/reorder", boardHandler.ReorderPhases)
			}

			tasks := api.Group("/tasks")
			{
				tasks.GET("", taskHandler.GetAll)
				tasks.POST("", taskHandler.Create)
				tasks.GET("/:id", taskHandler.GetByID)
				tasks.PUT("/:id", taskHandler.Update)
				tasks.DELETE("/:id", taskHandler.Delete)
				tasks.POST("/:id/toggle-completion", taskHandler.ToggleCompletion)
				tasks.POST("/:id/comments", taskHandler.AddComment)
				tasks.POST("/:id/attachments", taskHandler.AddAttachment)
				tasks.DELETE("/:id/attachments/:attachmentId", taskHandler.DeleteAttachment)
			}

			workHours := api.Group("/work-hours")
			{
				workHours.GET("", workHourHandler.GetAll)
				workHours.POST("", workHourHandler.Create)
				workHours.PUT("/:id", workHourHandler.Update)
				workHours.POST("/approve", workHourHandler.Approve)
				workHours.GET("/summary", workHourHandler.GetSummary)
				workHours.GET("/pending", workHourHandler.GetPending)
				workHours.POST("/send-report", workHourHandler.SendReport)
				workHours.GET("/report/pdf", workHourHandler.DownloadPDF)
				workHours.GET("/report/excel", workHourHandler.DownloadExcel)
			}

			chat := api.Group("/chat")
			{
				chat.GET("/messages", chatHandler.GetMessages)
				chat.POST("/messages", chatHandler.SendMessage)
			}

			uploads := api.Group("/uploads")
			{
				uploads.POST("", uploadHandler.UploadFile)
			}

			notifications := api.Group("/notifications")
			{
				notifications.GET("", notificationHandler.GetNotifications)
				notifications.GET("/unread-count", notificationHandler.GetUnreadCount)
				notifications.POST("/:id/read", notificationHandler.MarkAsRead)
				notifications.POST("/read-all", notificationHandler.MarkAllAsRead)
			}

			channels := api.Group("/channels")
			{
				channels.GET("/unread/total", channelHandler.GetTotalUnreadCount)
				channels.GET("", channelHandler.GetChannels)
				channels.POST("", channelHandler.CreateChannel)
				channels.GET("/all-users", channelHandler.GetAllUsers)
				channels.GET("/:id", channelHandler.GetChannel)
				channels.PUT("/:id", channelHandler.UpdateChannel)
				channels.DELETE("/:id", channelHandler.DeleteChannel)
				channels.GET("/:id/messages", channelHandler.GetMessages)
				channels.POST("/:id/messages", channelHandler.SendMessage)
				channels.PUT("/:id/messages/:messageId", channelHandler.EditMessage)
				channels.DELETE("/:id/messages/:messageId", channelHandler.DeleteMessage)
				channels.GET("/:id/messages/:messageId/reactions", channelHandler.GetReactions)
				channels.POST("/:id/messages/:messageId/reactions", channelHandler.AddReaction)
				channels.DELETE("/:id/messages/:messageId/reactions", channelHandler.RemoveReaction)
				channels.GET("/:id/messages/:messageId/replies", channelHandler.GetThreadReplies)
				channels.POST("/:id/messages/:messageId/replies", channelHandler.SendThreadReply)
				channels.GET("/:id/members", channelHandler.GetMembers)
				channels.POST("/:id/members", channelHandler.AddMember)
				channels.DELETE("/:id/members", channelHandler.RemoveMember)
				channels.POST("/:id/join", channelHandler.JoinChannel)
				channels.POST("/:id/leave", channelHandler.LeaveChannel)
				channels.POST("/:id/read", channelHandler.MarkAsRead)
				channels.POST("/:id/pin/:messageId", channelHandler.PinMessage)
				channels.POST("/:id/unpin/:messageId", channelHandler.UnpinMessage)
				channels.GET("/:id/pinned", channelHandler.GetPinnedMessages)
				channels.GET("/:id/search", channelHandler.SearchMessages)
				channels.POST("/star/:messageId", channelHandler.StarMessage)
				channels.DELETE("/star/:messageId", channelHandler.UnstarMessage)
				channels.GET("/starred", channelHandler.GetStarredMessages)
				channels.POST("/status", channelHandler.UpdateStatus)
				channels.GET("/statuses", channelHandler.GetStatuses)
				channels.POST("/dm", channelHandler.CreateDirectMessage)
			}

			email := api.Group("/email")
			email.Use(middleware.RequireSuperadmin())
			{
				email.GET("/templates", emailHandler.GetTemplates)
				email.POST("/templates", emailHandler.CreateTemplate)
				email.PUT("/templates/:id", emailHandler.UpdateTemplate)
				email.DELETE("/templates/:id", emailHandler.DeleteTemplate)
				email.GET("/campaigns", emailHandler.GetCampaigns)
				email.POST("/campaigns", emailHandler.CreateCampaign)
				email.PUT("/campaigns/:id", emailHandler.UpdateCampaign)
				email.DELETE("/campaigns/:id", emailHandler.DeleteCampaign)
				email.POST("/campaigns/:id/send", emailHandler.SendCampaign)
			}

			surveys := api.Group("/surveys")
			{
				// Admin routes
				surveys.POST("", middleware.RequireSuperadmin(), surveyHandler.CreateSurvey)
				surveys.GET("", middleware.RequireSuperadmin(), surveyHandler.GetSurveys)
				surveys.PUT("/:id", middleware.RequireSuperadmin(), surveyHandler.UpdateSurvey)
				surveys.DELETE("/:id", middleware.RequireSuperadmin(), surveyHandler.DeleteSurvey)
				surveys.POST("/:id/send", middleware.RequireSuperadmin(), surveyHandler.SendSurvey)
				
				// User routes
				surveys.GET("/:id", surveyHandler.GetSurvey)
				surveys.POST("/:id/responses", surveyHandler.SubmitResponse)
			}
			// Metrics logic
			api.GET("/metrics", middleware.RequireSuperadmin(), metricsHandler.GetGlobalMetrics)

			// Tickets logic
			tickets := api.Group("/tickets")
			{
				ticketHandler := handlers.NewTicketHandler(db, zohoSvc, brevoSvc)
				tickets.GET("", ticketHandler.GetTickets)
				tickets.GET("/:id", ticketHandler.GetTicket)
				tickets.PUT("/:id", ticketHandler.UpdateTicket)
				tickets.POST("/:id/messages", ticketHandler.SendMessage)
				tickets.POST("/:id/sendReply", ticketHandler.SendReply)
				tickets.GET("/contacts", ticketHandler.GetContacts)
				tickets.PUT("/contacts/:contactId", ticketHandler.UpdateContact)
				tickets.GET("/zoho/status", func(c *gin.Context) {
					// Zoho integration status stub
					c.JSON(200, gin.H{"status": "connected", "service": "zoho_desk"})
				})
			}

			// WhatsApp Chats — per-agent view + unassigned queue
			whatsappHandler := handlers.NewWhatsAppHandler(db, zohoSvc)
			chats := api.Group("/chats")
			{
				// GET  /api/chats/me           → tickets assigned to the logged-in agent
				chats.GET("/me", whatsappHandler.GetMyChats)
				// GET  /api/chats/unassigned   → tickets with no assignee
				chats.GET("/unassigned", whatsappHandler.GetUnassignedChats)
				// POST /api/chats/sync-agent   → force-refresh zoho_agent_id from Zoho by email
				chats.POST("/sync-agent", whatsappHandler.SyncAgentID)
				// GET  /api/chats/:ticketId/messages  → conversation thread
				chats.GET("/:ticketId/messages", whatsappHandler.GetMessages)
				// PATCH /api/chats/:ticketId/assign   → take ownership of an unassigned chat
				chats.PATCH("/:ticketId/assign", whatsappHandler.AssignToMe)
				// POST  /api/chats/:ticketId/send     → send a WhatsApp message
				chats.POST("/:ticketId/send", whatsappHandler.SendMessage)
			}
		}
	}

	r.GET("/api/uploads/:filename", uploadHandler.GetFile)
	r.GET("/ws/chat", middleware.AuthMiddleware(cfg.JWTSecret), chatHandler.HandleWebSocket)
	r.GET("/ws/channels", middleware.AuthMiddleware(cfg.JWTSecret), channelHandler.HandleWebSocket)
	r.GET("/ws/notifications", middleware.AuthMiddleware(cfg.JWTSecret), notificationHandler.HandleWebSocket)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

}
