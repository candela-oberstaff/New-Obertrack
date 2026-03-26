package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/migrations"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/routes"
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

	log.Println("Starting database migration...")
	if err := migrations.Run(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed successfully")

	log.Println("Initializing routes...")
	r := gin.Default()

	r.Use(middleware.CORS())
	r.Use(middleware.RateLimitMiddleware())

	routes.RegisterRoutes(r, db, cfg)

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
