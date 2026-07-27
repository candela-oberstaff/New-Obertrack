package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/obertrack/backend/internal/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// weakDefaultJWTSecret is the placeholder shipped in .env.example. It must never
// be used in a real deployment because it is public knowledge.
const weakDefaultJWTSecret = "your-super-secret-jwt-key-change-in-production"

type Config struct {
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	JWTSecret    string
	ServerPort   string
	DBSSLMode    string
	SupportEmail string
	// MultiManagerReads activa las lecturas de manager via tabla N-a-N
	// (employment_managers) con semántica "cualquier manager" (Fase 2).
	// Default false: comportamiento actual (puntero employments.manager_id).
	MultiManagerReads bool

	// --- Integración Ontop (módulo Wallet) ---
	// Credenciales de la cuenta Ontop usadas por el backend como proxy. NUNCA se
	// exponen al frontend/móvil: todas las llamadas a Ontop salen del servidor.
	OntopAPIURL   string
	OntopEmail    string
	OntopPassword string
	// OntopClientID es el ID de cliente de la billetera Ontop (usado en los
	// endpoints /client-wallet y en el payload de las paylists).
	OntopClientID string

	// --- Integración Google Calendar ---
	// Vínculo PERSONAL de cada usuario con su cuenta de Google. La pantalla de
	// consentimiento es de tipo "External", así que sirve cualquier cuenta
	// (gmail.com o dominio propio como los de Oberstaff) sin configuración extra
	// por dominio.
	GoogleCalendarEnabled bool
	GoogleClientID        string
	GoogleClientSecret    string
	// GoogleRedirectURI debe coincidir EXACTAMENTE con uno de los redirect URIs
	// registrados en la credencial OAuth de Google Cloud, o el canje falla con
	// redirect_uri_mismatch.
	GoogleRedirectURI string
	// GoogleTokenEncKey es la clave AES-256 (32 bytes en base64) con la que se
	// cifran los tokens de OAuth antes de guardarlos. Generar con:
	// openssl rand -base64 32
	GoogleTokenEncKey string

	// FrontendURL es la base a la que redirige el callback de OAuth al terminar
	// (el flujo vuelve al navegador, no a una respuesta JSON).
	FrontendURL string
}

func LoadConfig() *Config {
	cfg := &Config{
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "postgres"),
		DBPassword:   getEnv("DB_PASSWORD", ""),
		DBName:       getEnv("DB_NAME", "obertrack"),
		JWTSecret:    getEnv("JWT_SECRET", ""),
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		DBSSLMode:    getEnv("DB_SSL_MODE", "disable"),
		SupportEmail: getEnv("SUPPORT_EMAIL", ""),
		// Feature flag Fase 2: OFF por defecto; "true"/"1" lo activan.
		MultiManagerReads: getBoolEnv("MULTI_MANAGER_READS", false),

		// Ontop (Wallet). Default a sandbox/staging; las credenciales se inyectan
		// por entorno (nunca se hardcodean).
		OntopAPIURL:   getEnv("ONTOP_API_URL", "https://api.stg.getontop.com"),
		OntopEmail:    getEnv("ONTOP_EMAIL", ""),
		OntopPassword: getEnv("ONTOP_PASSWORD", ""),
		OntopClientID: getEnv("ONTOP_CLIENT_ID", ""),

		// Google Calendar. Apagado por defecto: sin el flag el módulo no se
		// construye y las rutas responden 503.
		GoogleCalendarEnabled: getBoolEnv("GOOGLE_CALENDAR_ENABLED", false),
		GoogleClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI:     getEnv("GOOGLE_REDIRECT_URI", ""),
		GoogleTokenEncKey:     getEnv("GOOGLE_TOKEN_ENC_KEY", ""),

		FrontendURL: strings.TrimRight(getEnv("FRONTEND_URL", ""), "/"),
	}

	// Fail fast on an insecure JWT secret. An empty, default, or short secret
	// allows anyone to forge superadmin tokens (see audit finding C-02).
	if cfg.JWTSecret == "" || cfg.JWTSecret == weakDefaultJWTSecret || len(cfg.JWTSecret) < 32 {
		log.Fatal("FATAL: JWT_SECRET is missing, uses the example default, or is shorter than 32 bytes. " +
			"Generate a strong secret, e.g.: openssl rand -base64 48")
	}

	cfg.validateGoogleCalendar()

	return cfg
}

// validateGoogleCalendar aborta el arranque si la integración se declaró activa
// pero le falta configuración. Es deliberadamente fail-fast: un botón "Conectar
// con Google" que falla en mitad del consentimiento —dejando al usuario en una
// pantalla de error de Google— es peor que un contenedor que no levanta.
// Con el flag apagado no valida nada: el módulo simplemente no se construye.
func (c *Config) validateGoogleCalendar() {
	if !c.GoogleCalendarEnabled {
		return
	}

	var missing []string
	if c.GoogleClientID == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}
	if c.GoogleClientSecret == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}
	if c.GoogleRedirectURI == "" {
		missing = append(missing, "GOOGLE_REDIRECT_URI")
	}
	if c.GoogleTokenEncKey == "" {
		missing = append(missing, "GOOGLE_TOKEN_ENC_KEY")
	}
	if len(missing) > 0 {
		log.Fatalf("FATAL: GOOGLE_CALENDAR_ENABLED=true pero faltan variables: %s. "+
			"Configúralas o apaga el flag.", strings.Join(missing, ", "))
	}

	// La clave se valida aquí (y no al primer vínculo) para que un error de
	// formato salte en el despliegue y no cuando un usuario pulse "Conectar".
	if _, err := utils.NewSecretSealer(c.GoogleTokenEncKey); err != nil {
		log.Fatalf("FATAL: GOOGLE_TOKEN_ENC_KEY inválida: %v. "+
			"Genera una con: openssl rand -base64 32", err)
	}

	if c.FrontendURL == "" {
		log.Println("WARN: FRONTEND_URL vacío — el callback de Google redirigirá a rutas relativas")
	}
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
		// No crear foreign keys físicas al hacer AutoMigrate: las relaciones del
		// modelo (p.ej. User.Empleador) son solo para Preload/lectura. Crear la FK
		// users.empleador_id → users.id rompía la migración en producción por
		// empleador_id huérfanos preexistentes (SQLSTATE 23503). La integridad se
		// valida en la capa de aplicación.
		DisableForeignKeyConstraintWhenMigrating: true,
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

// getBoolEnv lee un flag booleano de entorno. Acepta "true"/"1" (cualquier
// caja) como verdadero; cualquier otro valor (o ausencia) cae al default.
func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1":
		return true
	default:
		return false
	}
}
