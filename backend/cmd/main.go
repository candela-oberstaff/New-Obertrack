package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/obertrack/backend/internal/audit"
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

	// Record every row change (any table) as a "data" audit entry.
	audit.RegisterDataAuditHooks(db)

	log.Println("Initializing routes...")
	r := gin.Default()

	// Trust only known reverse proxies so X-Forwarded-For cannot be spoofed to
	// bypass rate limiting (audit finding A-05). Configure TRUSTED_PROXIES as a
	// comma-separated list of CIDRs; defaults to private ranges (e.g. the Docker
	// network where Nginx runs).
	trusted := []string{"127.0.0.1/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	if tp := os.Getenv("TRUSTED_PROXIES"); tp != "" {
		trusted = strings.Split(tp, ",")
		for i := range trusted {
			trusted[i] = strings.TrimSpace(trusted[i])
		}
	}
	if err := r.SetTrustedProxies(trusted); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	// health/root endpoint
	r.GET("/", func(c *gin.Context) { c.String(200, "Backend online") })

	r.Use(middleware.CORS())
	r.Use(middleware.RateLimitMiddleware())

	routes.RegisterRoutes(r, db, cfg)

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
