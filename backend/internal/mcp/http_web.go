package mcp

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

// Rutas de la web con sesión: estado, conexiones propias y tokens
// personales (tarjeta "Asistentes de IA" de Integraciones), y el panel de
// admin "Uso de IA".

func (h *Handler) currentUser(c *gin.Context) *models.User {
	u, err := h.Svc.Users.GetByID(middleware.GetUserID(c))
	if err != nil || u == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sesión inválida"})
		return nil
	}
	return u
}

// RequireMCPUse es el equivalente de "accion:usar-mcp": el rol del usuario
// tiene que tener el MCP habilitado en el panel.
func (h *Handler) RequireMCPUse() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := h.Svc.ActiveUser(middleware.GetUserID(c)); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Tu rol no tiene habilitados los asistentes de IA."})
			return
		}
		c.Next()
	}
}

// Status lo pide el menú y la página de Integraciones para saber si mostrar
// la integración. No exige el permiso: responde enabled=false.
func (h *Handler) Status(c *gin.Context) {
	u := h.currentUser(c)
	if u == nil {
		return
	}
	role := RoleKey(u)
	q := h.Svc.Quota(role)
	_, err := h.Svc.ActiveUser(u.ID)
	resp := gin.H{
		"enabled":    err == nil,
		"paused":     h.Svc.Settings().Paused,
		"server_url": h.base() + "/api/mcp",
		"role":       role,
		"role_label": RoleLabels[role],
		"quota":      gin.H{"daily_calls": q.DailyCalls, "max_rows": q.MaxRows},
		"used_today": h.Svc.CallsToday(u.ID),
	}
	if err == nil {
		if s, err := BuildScope(u, h.ScopeDeps); err == nil {
			resp["scope"] = s.Describe()
		}
	}
	c.JSON(http.StatusOK, resp)
}

type connectionView struct {
	ID           uint       `json:"id"`
	ClientName   string     `json:"client_name"`
	Personal     bool       `json:"personal"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokeReason string     `json:"revoke_reason,omitempty"`
	IP           string     `json:"ip,omitempty"`
	UserID       uint       `json:"user_id,omitempty"`
	UserName     string     `json:"user_name,omitempty"`
	UserEmail    string     `json:"user_email,omitempty"`
}

func (h *Handler) myConnections(c *gin.Context, personal bool) {
	u := h.currentUser(c)
	if u == nil {
		return
	}
	op := "<>"
	if personal {
		op = "="
	}
	var rows []connectionView
	h.Svc.DB.Model(&models.MCPConnection{}).
		Select("id, client_name, client_id = ? AS personal, created_at, last_used_at, expires_at", models.MCPPersonalClientID).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ? AND client_id "+op+" ?", u.ID, time.Now(), models.MCPPersonalClientID).
		Order("created_at DESC").Scan(&rows)
	if rows == nil {
		rows = []connectionView{}
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) ListMyConnections(c *gin.Context) { h.myConnections(c, false) }
func (h *Handler) ListMyTokens(c *gin.Context)      { h.myConnections(c, true) }

func (h *Handler) RevokeMine(c *gin.Context) {
	u := h.currentUser(c)
	if u == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.Svc.RevokeConnection(uint(id), u.ID, u, "revocado por el usuario", auditCtx(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conexión no encontrada"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) CreateToken(c *gin.Context) {
	u := h.currentUser(c)
	if u == nil {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	// Tope de tokens personales vivos para que no se acumulen olvidados.
	var n int64
	h.Svc.DB.Model(&models.MCPConnection{}).
		Where("user_id = ? AND client_id = ? AND revoked_at IS NULL AND expires_at > ?", u.ID, models.MCPPersonalClientID, time.Now()).
		Count(&n)
	if n >= 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ya tienes 5 tokens personales activos. Revoca alguno antes de crear otro."})
		return
	}
	token, conn, err := h.Svc.CreatePersonalToken(u, body.Name, auditCtx(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear el token"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"connection": connectionView{ID: conn.ID, ClientName: conn.ClientName, Personal: true, CreatedAt: conn.CreatedAt, ExpiresAt: conn.ExpiresAt},
	})
}

// ---------------------------------------------------------------------------
// Panel de admin "Uso de IA"

type quotaView struct {
	models.MCPRoleQuota
	Label string `json:"label"`
}

type alertView struct {
	models.MCPAlert
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

func (h *Handler) AdminOverview(c *gin.Context) {
	quotas := make([]quotaView, 0, len(RoleOrder))
	for _, r := range RoleOrder {
		quotas = append(quotas, quotaView{MCPRoleQuota: h.Svc.Quota(r), Label: RoleLabels[r]})
	}
	var alerts []alertView
	h.Svc.DB.Table("mcp_alerts a").
		Select("a.*, u.name AS user_name, u.email AS user_email").
		Joins("LEFT JOIN users u ON u.id = a.user_id").
		Where("a.seen_at IS NULL").Order("a.created_at DESC").Limit(100).Scan(&alerts)
	if alerts == nil {
		alerts = []alertView{}
	}
	now := time.Now()
	var activeConns, callsToday, usersToday int64
	h.Svc.DB.Model(&models.MCPConnection{}).Where("revoked_at IS NULL AND expires_at > ?", now).Count(&activeConns)
	h.Svc.DB.Model(&models.MCPUsage{}).Where("day = ?", dayStart(now)).Count(&callsToday)
	h.Svc.DB.Model(&models.MCPUsage{}).Where("day = ?", dayStart(now)).Distinct("user_id").Count(&usersToday)
	c.JSON(http.StatusOK, gin.H{
		"settings":   h.Svc.Settings(),
		"quotas":     quotas,
		"db_status":  h.RO.Status(),
		"alerts":     alerts,
		"server_url": h.base() + "/api/mcp",
		"totals":     gin.H{"active_connections": activeConns, "calls_today": callsToday, "users_today": usersToday},
	})
}

func (h *Handler) AdminConnections(c *gin.Context) {
	q := h.Svc.DB.Table("mcp_connections cn").
		Select(`cn.id, cn.client_name, cn.client_id = ? AS personal, cn.created_at, cn.last_used_at, cn.expires_at,
			cn.revoked_at, cn.revoke_reason, cn.ip, cn.user_id, u.name AS user_name, u.email AS user_email`, models.MCPPersonalClientID).
		Joins("LEFT JOIN users u ON u.id = cn.user_id")
	if c.Query("status") != "all" {
		q = q.Where("cn.revoked_at IS NULL AND cn.expires_at > ?", time.Now())
	}
	if s := strings.TrimSpace(c.Query("search")); s != "" {
		l := likeArg(s)
		q = q.Where("u.name ILIKE ? OR u.email ILIKE ? OR cn.client_name ILIKE ?", l, l, l)
	}
	var rows []connectionView
	q.Order("cn.created_at DESC").Limit(500).Scan(&rows)
	if rows == nil {
		rows = []connectionView{}
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) AdminRevoke(c *gin.Context) {
	admin := h.currentUser(c)
	if admin == nil {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.Reason) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Indica el motivo de la revocación"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.Svc.RevokeConnection(uint(id), 0, admin, strings.TrimSpace(body.Reason), auditCtx(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conexión no encontrada"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type usageRow struct {
	Day       string  `json:"day"`
	UserID    uint    `json:"user_id"`
	UserName  string  `json:"user_name"`
	UserEmail string  `json:"user_email"`
	Calls     int64   `json:"calls"`
	Errors    int64   `json:"errors"`
	Rows      int64   `json:"rows"`
	AvgMs     float64 `json:"avg_ms"`
}

type toolRow struct {
	Tool   string  `json:"tool"`
	Calls  int64   `json:"calls"`
	Errors int64   `json:"errors"`
	AvgMs  float64 `json:"avg_ms"`
}

func (h *Handler) AdminUsage(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}
	since := dayStart(time.Now()).AddDate(0, 0, -(days - 1))
	var byUser []usageRow
	h.Svc.DB.Table("mcp_usage us").
		Select(`to_char(us.day, 'YYYY-MM-DD') AS day, us.user_id, u.name AS user_name, u.email AS user_email,
			count(*) AS calls, count(*) FILTER (WHERE NOT us.ok) AS errors, COALESCE(sum(us.rows), 0) AS rows,
			COALESCE(avg(us.ms), 0)::float8 AS avg_ms`).
		Joins("LEFT JOIN users u ON u.id = us.user_id").
		Where("us.day >= ?", since).
		Group("us.day, us.user_id, u.name, u.email").Order("us.day DESC, calls DESC").Limit(1000).Scan(&byUser)
	var byTool []toolRow
	h.Svc.DB.Table("mcp_usage").
		Select("tool, count(*) AS calls, count(*) FILTER (WHERE NOT ok) AS errors, COALESCE(avg(ms), 0)::float8 AS avg_ms").
		Where("day >= ?", since).Group("tool").Order("calls DESC").Scan(&byTool)
	if byUser == nil {
		byUser = []usageRow{}
	}
	if byTool == nil {
		byTool = []toolRow{}
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "by_user_day": byUser, "by_tool": byTool})
}

func (h *Handler) AdminUpdateQuota(c *gin.Context) {
	role := c.Param("role")
	if _, ok := RoleLabels[role]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rol desconocido"})
		return
	}
	var body struct {
		Enabled    bool `json:"enabled"`
		DailyCalls int  `json:"daily_calls"`
		MaxRows    int  `json:"max_rows"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.DailyCalls < 0 || body.DailyCalls > 100000 || body.MaxRows < 1 || body.MaxRows > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Valores fuera de rango (llamadas 0–100000, filas 1–500)"})
		return
	}
	// Upsert explícito: los bool se escriben siempre, sin depender de defaults.
	err := h.Svc.DB.Exec(`INSERT INTO mcp_role_quotas (role_name, enabled, daily_calls, max_rows, updated_at)
		VALUES (?, ?, ?, ?, NOW())
		ON CONFLICT (role_name) DO UPDATE SET enabled = EXCLUDED.enabled, daily_calls = EXCLUDED.daily_calls,
			max_rows = EXCLUDED.max_rows, updated_at = NOW()`, role, body.Enabled, body.DailyCalls, body.MaxRows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la cuota"})
		return
	}
	c.JSON(http.StatusOK, quotaView{MCPRoleQuota: h.Svc.Quota(role), Label: RoleLabels[role]})
}

func (h *Handler) AdminUpdateSettings(c *gin.Context) {
	admin := h.currentUser(c)
	if admin == nil {
		return
	}
	var body struct {
		Paused               bool   `json:"paused"`
		MinimalData          bool   `json:"minimal_data"`
		ExtraRedirectDomains string `json:"extra_redirect_domains"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	domains := strings.Join(ParseExtraDomains(body.ExtraRedirectDomains), "\n")
	before := h.Svc.Settings()
	err := h.Svc.DB.Exec(`INSERT INTO mcp_settings (id, paused, minimal_data, extra_redirect_domains, updated_by, updated_at)
		VALUES (1, ?, ?, ?, ?, NOW())
		ON CONFLICT (id) DO UPDATE SET paused = EXCLUDED.paused, minimal_data = EXCLUDED.minimal_data,
			extra_redirect_domains = EXCLUDED.extra_redirect_domains, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		body.Paused, body.MinimalData, domains, admin.ID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron guardar los ajustes"})
		return
	}
	if before.Paused != body.Paused {
		h.Svc.audit("mcp.paused", admin, true, auditCtx(c), "", map[string]any{"paused": body.Paused})
	}
	c.JSON(http.StatusOK, h.Svc.Settings())
}

func (h *Handler) AdminMarkAlertsSeen(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	_ = c.ShouldBindJSON(&body)
	q := h.Svc.DB.Model(&models.MCPAlert{}).Where("seen_at IS NULL")
	if len(body.IDs) > 0 {
		q = q.Where("id IN ?", body.IDs)
	}
	q.Update("seen_at", time.Now())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AdminPurge borra conexiones vencidas o revocadas hace más de 30 días (con
// sus tokens) y los códigos de autorización viejos. La auditoría queda.
func (h *Handler) AdminPurge(c *gin.Context) {
	cutoff := time.Now().AddDate(0, 0, -30)
	var ids []uint
	h.Svc.DB.Model(&models.MCPConnection{}).
		Where("(revoked_at IS NOT NULL AND revoked_at < ?) OR expires_at < ?", cutoff, time.Now()).
		Pluck("id", &ids)
	var tokens, codes int64
	if len(ids) > 0 {
		tokens = h.Svc.DB.Where("connection_id IN ?", ids).Delete(&models.MCPToken{}).RowsAffected
		h.Svc.DB.Where("id IN ?", ids).Delete(&models.MCPConnection{})
	}
	codes = h.Svc.DB.Where("expires_at < ?", time.Now().Add(-24*time.Hour)).Delete(&models.MCPAuthCode{}).RowsAffected
	c.JSON(http.StatusOK, gin.H{"connections": len(ids), "tokens": tokens, "codes": codes})
}
