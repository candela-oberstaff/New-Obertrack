package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/models"
)

// AuditRecorder is the minimal surface the audit middleware needs (implemented
// by service.AuditService). Kept here to avoid a service→middleware import cycle.
type AuditRecorder interface {
	Record(entry models.AuditLog)
}

var mutatingMethods = map[string]bool{
	"POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// AuditMiddleware records every authenticated MUTATING request (POST/PUT/PATCH/
// DELETE) as an audit entry. Reads must be excluded to avoid noise. Writes run
// asynchronously so they never slow the request.
func AuditMiddleware(audit AuditRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !mutatingMethods[c.Request.Method] {
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		module := moduleFromPath(path)
		status := c.Writer.Status()

		var actorID *uint
		if id := GetUserID(c); id > 0 {
			actorID = &id
		}
		var tenantID *uint
		if t := GetTenantID(c); t > 0 {
			tenantID = &t
		}

		target := firstParam(c, "id", "ticketId", "messageId", "attachmentId", "phaseId", "eid")
		entry := models.AuditLog{
			Kind:       "activity",
			ActorID:    actorID,
			ActorEmail: c.GetString("email"),
			ActorRole:  GetUserRole(c),
			TenantID:   tenantID,
			Action:     deriveAction(c.Request.Method, path, module),
			Module:     module,
			EntityType: module,
			EntityID:   target,
			Method:     c.Request.Method,
			Path:       path,
			TargetID:   target,
			Status:     status,
			Success:    status < 400,
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		}

		go audit.Record(entry)
	}
}

// firstParam returns the first non-empty route param among the given names.
func firstParam(c *gin.Context, names ...string) string {
	for _, n := range names {
		if v := c.Param(n); v != "" {
			return v
		}
	}
	return ""
}

// moduleFromPath returns the first path segment after /api (e.g. "work-hours").
func moduleFromPath(path string) string {
	p := strings.TrimPrefix(path, "/api/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return p
}

// deriveAction builds a readable label like "work-hours.reject" or "tickets.update".
func deriveAction(method, path, module string) string {
	// Recognize meaningful action suffixes in the route.
	for _, suffix := range []string{"approve", "reject", "transfer", "suspend", "activate", "promote-manager", "toggle-status", "toggle-completion", "reset-password", "change-password", "send", "notes"} {
		if strings.Contains(path, "/"+suffix) {
			return module + "." + strings.ReplaceAll(suffix, "-", "_")
		}
	}
	switch method {
	case "POST":
		return module + ".create"
	case "PUT", "PATCH":
		return module + ".update"
	case "DELETE":
		return module + ".delete"
	default:
		return module + "." + strings.ToLower(method)
	}
}
