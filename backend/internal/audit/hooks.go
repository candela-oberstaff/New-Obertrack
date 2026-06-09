// Package audit registers global GORM callbacks that record every row change
// (create/update/delete) in any table as a "data" audit entry. This captures
// mutations regardless of origin: API handlers, webhooks, background goroutines
// or seeds. It complements the route-level "activity" audit middleware.
package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// auditSkipTables are never audited at the data layer. audit_logs is mandatory
// (prevents infinite recursion). Add high-volume/ephemeral tables here if needed.
var auditSkipTables = map[string]bool{
	"audit_logs": true,
}

// redactKeys are masked in the changes JSON to avoid storing secrets.
var redactKeys = map[string]bool{
	"password": true, "token": true, "secret": true,
	"reset_token": true, "reset_token_expiry": true, "refresh_token": true,
	"remember_token": true, "token_version": true,
}

// RegisterDataAuditHooks wires the create/update/delete callbacks. Call once
// after migrations and before serving.
func RegisterDataAuditHooks(db *gorm.DB) {
	_ = db.Callback().Create().After("gorm:create").Register("audit:data_create", record("created"))
	_ = db.Callback().Update().After("gorm:update").Register("audit:data_update", record("updated"))
	_ = db.Callback().Delete().After("gorm:delete").Register("audit:data_delete", record("deleted"))
}

func record(op string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db.Statement == nil || db.Statement.Schema == nil || db.Statement.Error != nil {
			return
		}
		table := db.Statement.Table
		if table == "" || auditSkipTables[table] {
			return
		}

		changes := ""
		if op == "updated" {
			if m, ok := db.Statement.Dest.(map[string]interface{}); ok {
				changes = redactJSON(m)
			}
		}

		ids := primaryKeys(db)
		var entries []models.AuditLog
		appendEntry := func(id string) {
			entries = append(entries, models.AuditLog{
				Kind:       "data",
				Action:     table + "." + op,
				Module:     table,
				EntityType: table,
				EntityID:   id,
				Changes:    changes,
				Method:     "DB",
				Status:     200,
				Success:    true,
			})
		}
		if len(ids) == 0 {
			appendEntry("")
		}
		for _, id := range ids {
			appendEntry(id)
		}

		// Insert without re-triggering hooks; the audit_logs skip-guard also
		// prevents recursion.
		sess := db.Session(&gorm.Session{NewDB: true, SkipHooks: true})
		if err := sess.Create(&entries).Error; err != nil {
			log.Printf("[Audit] data hook insert failed for %s.%s: %v", table, op, err)
		}
	}
}

// primaryKeys extracts the primary key value(s) from the statement, handling
// both single-record and batch operations. Returns empty when not resolvable
// (e.g. a conditional delete without a loaded model).
func primaryKeys(db *gorm.DB) []string {
	field := db.Statement.Schema.PrioritizedPrimaryField
	if field == nil {
		return nil
	}
	rv := db.Statement.ReflectValue
	var ids []string
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if v, zero := field.ValueOf(db.Statement.Context, rv.Index(i)); !zero {
				ids = append(ids, fmt.Sprintf("%v", v))
			}
		}
	case reflect.Struct:
		if v, zero := field.ValueOf(db.Statement.Context, rv); !zero {
			ids = append(ids, fmt.Sprintf("%v", v))
		}
	}
	return ids
}

func redactJSON(m map[string]interface{}) string {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if redactKeys[strings.ToLower(k)] {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}
