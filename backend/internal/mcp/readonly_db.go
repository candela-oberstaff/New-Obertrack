package mcp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Capa de base de datos de solo lectura de los tools.
//
// Dos barreras independientes:
//  1. Con MCP_DB_USER y MCP_DB_PASSWORD, un pool propio con el rol
//     <obertrack>_mcp_ro (sql/mcp_readonly_role.sql): solo SELECT, solo en las
//     tablas y columnas que usan los tools, y sin columnas secretas de users.
//  2. Siempre, cada consulta corre en BEGIN READ ONLY con un statement_timeout
//     local de 15 s, y termina en ROLLBACK.
//
// Si el rol dedicado falla (contraseña cambiada, falta un GRANT), se cae al
// pool general, se registra como CRITICAL y el panel de admin lo muestra. El
// MCP nunca queda caído por eso: la barrera 2 sigue en pie.

const statementTimeoutMs = 15000

// DBStatus es lo que el panel de admin muestra del rol dedicado.
type DBStatus struct {
	Mode   string    `json:"mode"` // dedicated | general | fallback
	Detail string    `json:"detail"`
	Since  time.Time `json:"since"`
}

type ReadOnlyDB struct {
	general *gorm.DB

	mu        sync.RWMutex
	dedicated *gorm.DB
	status    DBStatus
}

// DSNConfig son los datos de conexión que se reutilizan del pool general.
type DSNConfig struct {
	Host, Port, Name, SSLMode string
}

// NewReadOnlyDB abre el pool dedicado si hay credenciales; si no, usa el
// general. No falla nunca: en el peor caso funciona con el pool general.
func NewReadOnlyDB(general *gorm.DB, c DSNConfig) *ReadOnlyDB {
	r := &ReadOnlyDB{general: general}
	user, pass := os.Getenv("MCP_DB_USER"), os.Getenv("MCP_DB_PASSWORD")
	if user == "" || pass == "" {
		r.status = DBStatus{Mode: "general", Detail: "MCP_DB_USER y MCP_DB_PASSWORD no están definidas: las consultas usan el pool general, siempre en transacciones READ ONLY.", Since: time.Now()}
		log.Println("[MCP] Consultas del MCP con el pool general (READ ONLY por transacción); define MCP_DB_USER/MCP_DB_PASSWORD para el rol dedicado.")
		return r
	}
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC application_name=obertrack_mcp",
		c.Host, user, pass, c.Name, c.Port, c.SSLMode)
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err == nil {
		var sqlDB *sql.DB
		if sqlDB, err = db.DB(); err == nil {
			sqlDB.SetMaxOpenConns(10)
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetConnMaxIdleTime(5 * time.Minute)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = sqlDB.PingContext(ctx)
			cancel()
		}
	}
	if err != nil {
		r.fallback("no se pudo conectar con el rol dedicado " + user + ": " + safeDBError(err))
		return r
	}
	r.dedicated = db
	r.status = DBStatus{Mode: "dedicated", Detail: "Rol de solo lectura " + user + ".", Since: time.Now()}
	log.Printf("[MCP] Consultas del MCP con el rol de solo lectura %s", user)
	return r
}

// NewReadOnlyDBFrom envuelve un pool ya abierto (tests).
func NewReadOnlyDBFrom(general *gorm.DB) *ReadOnlyDB {
	return &ReadOnlyDB{general: general, status: DBStatus{Mode: "general", Since: time.Now()}}
}

func (r *ReadOnlyDB) fallback(detail string) {
	r.mu.Lock()
	r.dedicated = nil
	r.status = DBStatus{Mode: "fallback", Detail: detail, Since: time.Now()}
	r.mu.Unlock()
	log.Printf("[MCP] CRITICAL: %s. Las consultas del MCP siguen con el pool general en transacciones READ ONLY.", detail)
}

// Status para el panel de admin.
func (r *ReadOnlyDB) Status() DBStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// Query ejecuta fn en una transacción de solo lectura con timeout, y la
// deshace al final pase lo que pase.
func (r *ReadOnlyDB) Query(ctx context.Context, fn func(tx *gorm.DB) error) error {
	r.mu.RLock()
	db := r.dedicated
	r.mu.RUnlock()
	if db != nil {
		err := runReadOnly(ctx, db, fn)
		if err == nil || !isRoleProblem(err) {
			return err
		}
		r.fallback("el rol dedicado falló en una consulta: " + safeDBError(err))
	}
	return runReadOnly(ctx, r.general, fn)
}

func runReadOnly(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := db.WithContext(ctx).Begin(&sql.TxOptions{ReadOnly: true})
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	if err := tx.Exec(fmt.Sprintf("SET LOCAL statement_timeout = %d", statementTimeoutMs)).Error; err != nil {
		return err
	}
	return fn(tx)
}

// isRoleProblem distingue un fallo del rol (sin permiso, credenciales, sin
// conexión) de un fallo de la consulta en sí, que no justifica cambiar de
// pool.
func isRoleProblem(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42501", // insufficient_privilege: falta un GRANT
			"28000", "28P01", // autenticación
			"3D000": // la base no existe
			return true
		}
		return false
	}
	var connErr *pgconn.ConnectError
	return errors.As(err, &connErr)
}

// safeDBError resume un error de base sin arrastrar la consulta.
func safeDBError(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code + " " + pgErr.Message
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}
