// Command seed carga un juego de datos de demostración en una base de datos de
// desarrollo: empresas, equipos, tableros con tareas, jornadas, chat, soporte e
// incidentes. Existe para que alguien que acaba de clonar el repositorio tenga
// una aplicación con contenido —y con usuarios de cada tipo para entrar— en un
// solo comando, en vez de crear todo a mano por la UI.
//
//	docker compose --profile seed run --rm seeder    # camino normal (la BD no
//	                                                 # publica puerto al host)
//	go run ./cmd/seed                                # con una BD accesible
//
// Es idempotente: se puede correr las veces que haga falta sin duplicar nada.
// TODO lo que crea vive bajo el dominio de correo demo.obertrack.test, y -reset
// borra exactamente eso (y nada más) antes de volver a sembrar.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm/logger"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/migrations"
)

// demoDomain es el dominio de TODOS los correos que crea el seeder. Es la marca
// que hace posible un -reset quirúrgico: nada fuera de este dominio se toca.
// El TLD .test está reservado por la RFC 2606, así que ningún correo de prueba
// puede salir de verdad hacia una bandeja real.
const demoDomain = "demo.obertrack.test"

func main() {
	var (
		password  = flag.String("password", "Obertrack2026!", "Contraseña de todas las cuentas sembradas")
		reset     = flag.Bool("reset", false, "Borra los datos de demo previos antes de sembrar")
		resetOnly = flag.Bool("reset-only", false, "Solo borra los datos de demo y termina")
		migrate   = flag.Bool("migrate", true, "Corre las migraciones antes de sembrar")
		force     = flag.Bool("force", false, "Permite ejecutar con GIN_MODE=release (NO usar contra producción)")
		verbose   = flag.Bool("verbose", false, "Registra cada consulta SQL")
	)
	flag.Parse()

	loadEnv()

	// El seeder crea cuentas con una contraseña conocida y publicada en este
	// archivo: en un entorno marcado como producción se niega a correr.
	if os.Getenv("GIN_MODE") == "release" && !*force {
		log.Fatal("GIN_MODE=release: el seeder no corre contra un entorno de producción. " +
			"Si de verdad es un entorno de prueba, repite con -force.")
	}

	db, err := config.InitDB(&config.Config{
		DBHost:     env("DB_HOST", "localhost"),
		DBPort:     env("DB_PORT", "5432"),
		DBUser:     env("DB_USER", "postgres"),
		DBPassword: env("DB_PASSWORD", ""),
		DBName:     env("DB_NAME", "obertrack"),
		DBSSLMode:  env("DB_SSL_MODE", "disable"),
	})
	if err != nil {
		log.Fatalf("No se pudo conectar a la base de datos: %v", err)
	}

	// InitDB deja el logger en Info fuera de release: con cientos de inserts eso
	// sepulta el resumen final bajo el SQL. El "record not found" tampoco es un
	// error aquí: es la mitad normal de cada "busca o crea".
	if !*verbose {
		db.Logger = logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
		})
	}

	if *migrate {
		log.Println("Corriendo migraciones...")
		if err := migrations.Run(db); err != nil {
			log.Fatalf("Fallaron las migraciones: %v", err)
		}
	}

	if *reset || *resetOnly {
		log.Println("Borrando datos de demo previos...")
		if err := resetDemoData(db); err != nil {
			log.Fatalf("No se pudo limpiar: %v", err)
		}
		log.Println("Datos de demo borrados")
	}
	if *resetOnly {
		return
	}

	s := &seeder{db: db, password: *password, now: time.Now().UTC()}
	if err := s.Run(); err != nil {
		log.Fatalf("Falló el seeder: %v", err)
	}
}

// loadEnv reproduce la búsqueda de .env del servidor (cmd/main.go) para que el
// seeder funcione igual corriéndolo desde backend/, desde la raíz o dentro del
// contenedor (donde no hay .env y las variables llegan por el entorno).
func loadEnv() {
	cwd, _ := os.Getwd()
	for _, p := range []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(cwd, "..", ".env"),
		filepath.Join(cwd, "..", "..", ".env"),
	} {
		if _, err := os.Stat(p); err == nil {
			godotenv.Load(p)
			log.Printf("Variables cargadas de: %s", p)
			return
		}
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
