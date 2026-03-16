package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

func main() {
	godotenv.Load("../.env")

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

	authHandler := handlers.NewAuthHandler(db, cfg)
	userHandler := handlers.NewUserHandler(db)
	taskHandler := handlers.NewTaskHandler(db)
	workHourHandler := handlers.NewWorkHourHandler(db)
	chatHandler := handlers.NewChatHandler(db)
	adminHandler := handlers.NewAdminHandler(db)

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		seed := api.Group("/seed")
		{
			seed.POST("/superadmin", adminHandler.CreateSuperAdmin)
			seed.POST("/reset-superadmin", adminHandler.ResetSuperAdmin)
			seed.POST("/make-superadmin/:email", adminHandler.MakeSuperAdmin)
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
				users.GET("/:id", userHandler.GetByID)
				users.PUT("/:id", userHandler.Update)
				users.POST("/:id/toggle-status", userHandler.ToggleStatus)
				users.POST("/:id/promote-manager", userHandler.PromoteToManager)
			}

			admin := api.Group("/admin")
			admin.Use(middleware.RoleMiddleware("superadmin"))
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
		}
	}

	r.GET("/ws/chat", func(c *gin.Context) {
		chatHandler.HandleWebSocket(c)
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
