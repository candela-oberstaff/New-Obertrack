package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

func main() {
	cwd, _ := os.Getwd()

	paths := []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(cwd, "..", ".env"),
		filepath.Join(cwd, "..", "..", ".env"),
		".env",
	}

	var envPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			envPath = p
			break
		}
	}

	if envPath != "" {
		godotenv.Load(envPath)
		log.Printf("Loaded env from: %s", envPath)
	}

	cfg := config.LoadConfig()

	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := models.Migrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database connected and migrated successfully")

	r := gin.Default()

	r.Use(middleware.CORS())
	r.Use(middleware.RateLimitMiddleware())

	authHandler := handlers.NewAuthHandler(db, cfg)
	userHandler := handlers.NewUserHandler(db)
	taskHandler := handlers.NewTaskHandler(db)
	workHourHandler := handlers.NewWorkHourHandler(db)
	chatHandler := handlers.NewChatHandler(db)
	adminHandler := handlers.NewAdminHandler(db)
	boardHandler := handlers.NewBoardHandler(db)
	uploadHandler := handlers.NewUploadHandler(db, os.Getenv("UPLOAD_PATH"))
	notificationHandler := handlers.NewNotificationHandler(db)
	channelHandler := handlers.NewChannelHandler(db)

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/hash", authHandler.GeneratePasswordHash)
		}

		seed := api.Group("/seed")
		{
			seed.POST("/superadmin", adminHandler.CreateSuperAdmin)
			seed.POST("/reset-superadmin", adminHandler.ResetSuperAdmin)
			seed.POST("/make-superadmin/:email", adminHandler.MakeSuperAdmin)
			seed.POST("/create-superadmin", adminHandler.CreateSuperAdminForced)
		}

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
				boards.GET("/:id", boardHandler.GetByID)
				boards.PUT("/:id", boardHandler.Update)
				boards.DELETE("/:id", boardHandler.Delete)
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
			}

			workHours := api.Group("/work-hours")
			{
				workHours.GET("", workHourHandler.GetAll)
				workHours.POST("", workHourHandler.Create)
				workHours.PUT("/:id", workHourHandler.Update)
				workHours.POST("/approve", workHourHandler.Approve)
				workHours.GET("/summary", workHourHandler.GetSummary)
				workHours.GET("/pending", workHourHandler.GetPending)
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
		}
	}

	r.GET("/api/uploads/:filename", uploadHandler.GetFile)
	r.GET("/ws/chat", middleware.AuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		chatHandler.HandleWebSocket(c)
	})
	r.GET("/ws/channels", middleware.AuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		channelHandler.HandleWebSocket(c)
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
