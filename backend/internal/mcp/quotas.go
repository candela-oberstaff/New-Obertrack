package mcp

import (
	"fmt"
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// Cuotas, uso y alertas.
//
// La cuota es de llamadas por día y por rol (whoami no cuenta) y de filas
// máximas por respuesta. Al 80 % y al 100 % se deja una alerta en el panel
// de admin; no se manda ningún correo.

// dayStart es el inicio del día (UTC) de t: el "día" de mcp_usage.
func dayStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// CallsToday cuenta las llamadas de hoy que consumen cuota.
func (s *Service) CallsToday(userID uint) int64 {
	var n int64
	s.DB.Model(&models.MCPUsage{}).
		Where("user_id = ? AND day = ? AND tool <> ?", userID, dayStart(s.now()), "whoami").
		Count(&n)
	return n
}

// RecordUsage guarda una llamada. Un fallo aquí no rompe la respuesta.
func (s *Service) RecordUsage(userID, connID uint, tool string, ok bool, ms, rows int) {
	now := s.now()
	if err := s.DB.Create(&models.MCPUsage{
		UserID: userID, ConnectionID: connID, Tool: tool, OK: ok, Ms: ms, Rows: rows,
		Day: dayStart(now), CreatedAt: now,
	}).Error; err != nil {
		log.Printf("[MCP] no se pudo registrar el uso de %s: %v", tool, err)
	}
}

// AddAlert inserta una alerta. Las de cuota se crean una sola vez por
// usuario, tipo y día; devuelve si se insertó.
func (s *Service) AddAlert(userID uint, kind, detail string) bool {
	now := s.now()
	if kind == "quota_80" || kind == "quota_100" {
		var n int64
		s.DB.Model(&models.MCPAlert{}).
			Where("user_id = ? AND kind = ? AND created_at >= ?", userID, kind, dayStart(now)).
			Count(&n)
		if n > 0 {
			return false
		}
	}
	if err := s.DB.Create(&models.MCPAlert{UserID: userID, Kind: kind, Detail: detail, CreatedAt: now}).Error; err != nil {
		log.Printf("[MCP] no se pudo guardar la alerta %s: %v", kind, err)
		return false
	}
	return true
}

// QuotaCheck decide si el usuario puede hacer otra llamada hoy. Devuelve el
// mensaje para el asistente si no puede.
func (s *Service) QuotaCheck(user *models.User, q models.MCPRoleQuota, ac AuditCtx) (used int64, blocked string) {
	used = s.CallsToday(user.ID)
	if used < int64(q.DailyCalls) {
		return used, ""
	}
	if s.AddAlert(user.ID, "quota_100", fmt.Sprintf("%s llegó al 100 %% de su cuota diaria (%d llamadas).", user.Email, q.DailyCalls)) {
		s.audit("mcp.quota_exceeded", user, false, ac, "", map[string]any{"daily_calls": q.DailyCalls})
	}
	return used, fmt.Sprintf("Llegaste al límite de %d consultas por día de tu rol. Se reinicia a las 00:00 UTC; si necesitas más, pídeselo a un administrador.", q.DailyCalls)
}

// AfterCall deja la alerta del 80 % cuando esta llamada cruza el umbral.
func (s *Service) AfterCall(user *models.User, q models.MCPRoleQuota, usedBefore int64) {
	threshold := int64(q.DailyCalls) * 8 / 10
	if threshold > 0 && usedBefore < threshold && usedBefore+1 >= threshold {
		s.AddAlert(user.ID, "quota_80", fmt.Sprintf("%s usó el 80 %% de su cuota diaria (%d de %d llamadas).", user.Email, usedBefore+1, q.DailyCalls))
	}
}
