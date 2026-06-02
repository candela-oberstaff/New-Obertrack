package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// weakDefaultJWTSecret is the placeholder shipped in .env.example. It must never
// be used in a real deployment because it is public knowledge.
const weakDefaultJWTSecret = "your-super-secret-jwt-key-change-in-production"

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	JWTSecret  string
	ServerPort string
	DBSSLMode  string
}

func LoadConfig() *Config {
	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "obertrack"),
		JWTSecret:  getEnv("JWT_SECRET", ""),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),
	}

	// Fail fast on an insecure JWT secret. An empty, default, or short secret
	// allows anyone to forge superadmin tokens (see audit finding C-02).
	if cfg.JWTSecret == "" || cfg.JWTSecret == weakDefaultJWTSecret || len(cfg.JWTSecret) < 32 {
		log.Fatal("FATAL: JWT_SECRET is missing, uses the example default, or is shorter than 32 bytes. " +
			"Generate a strong secret, e.g.: openssl rand -base64 48")
	}

	return cfg
}

func (c *Config) GetDSN() string {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		c.DBHost, c.DBUser, c.DBPassword, c.DBName, c.DBPort, c.DBSSLMode,
	)
	return dsn
}

func InitDB(cfg *Config) (*gorm.DB, error) {
	dsn := cfg.GetDSN()
	// Never log the DSN: it contains the DB password in clear text (audit finding C-04).
	log.Printf("Connecting to database host=%s db=%s sslmode=%s", cfg.DBHost, cfg.DBName, cfg.DBSSLMode)

	// In production, avoid logging every SQL query (it leaks personal/payroll data).
	gormLogLevel := logger.Warn
	if os.Getenv("GIN_MODE") != "release" {
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
