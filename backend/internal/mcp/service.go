package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
)

// Service concentra el estado del MCP: clientes, códigos, conexiones,
// tokens, cuotas y ajustes. Los handlers HTTP solo traducen.
type Service struct {
	DB *gorm.DB
	// Users trae al usuario sin borrar (soft delete incluido).
	Users interface {
		GetByID(id uint) (*models.User, error)
	}
	// CanSignIn son las mismas verificaciones del login (cuenta suspendida,
	// inducción pendiente, empresa suspendida): authService.AssertCanSignIn.
	CanSignIn func(*models.User) error
	Audit     interface{ Record(models.AuditLog) }
	// Dev admite redirect_uri http://localhost (GIN_MODE != release).
	Dev bool
	Now func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// OAuthError es un error del protocolo con su código RFC 6749.
type OAuthError struct {
	Status int
	Code   string
	Desc   string
}

func (e *OAuthError) Error() string { return e.Code + ": " + e.Desc }

func oauthErr(status int, code, desc string) *OAuthError {
	return &OAuthError{Status: status, Code: code, Desc: desc}
}

// ---------------------------------------------------------------------------
// Ajustes y cuotas

// Settings lee la fila única de ajustes; sin fila, valores por defecto.
func (s *Service) Settings() models.MCPSettings {
	var st models.MCPSettings
	if err := s.DB.First(&st, 1).Error; err != nil {
		return models.MCPSettings{ID: 1}
	}
	return st
}

// DefaultQuota es la cuota de un rol sin fila (no debería pasar tras la
// migración): solo el superadmin habilitado.
func DefaultQuota(role string) models.MCPRoleQuota {
	q := models.MCPRoleQuota{RoleName: role, DailyCalls: 300, MaxRows: 50}
	if role == RoleSuperadmin {
		q.Enabled, q.DailyCalls = true, 1500
	}
	return q
}

func (s *Service) Quota(role string) models.MCPRoleQuota {
	var q models.MCPRoleQuota
	if err := s.DB.Where("role_name = ?", role).First(&q).Error; err != nil {
		return DefaultQuota(role)
	}
	return q
}

// ---------------------------------------------------------------------------
// Auditoría

// AuditCtx es lo que aporta la petición HTTP a un evento.
type AuditCtx struct {
	IP, UA, Path, Method string
}

func (s *Service) audit(action string, user *models.User, ok bool, ac AuditCtx, entityID string, detail map[string]any) {
	if s.Audit == nil {
		return
	}
	entry := models.AuditLog{
		Kind:       "activity",
		Action:     action,
		Module:     "mcp",
		EntityType: "mcp_connection",
		EntityID:   entityID,
		Method:     ac.Method,
		Path:       ac.Path,
		Status:     http.StatusOK,
		Success:    ok,
		IP:         ac.IP,
		UserAgent:  truncate(ac.UA, 500),
	}
	if !ok {
		entry.Status = http.StatusBadRequest
	}
	if user != nil {
		id := user.ID
		entry.ActorID = &id
		entry.ActorEmail = user.Email
		entry.ActorRole = RoleKey(user)
		if t := models.TenantForUser(user); t > 0 {
			entry.TenantID = &t
		}
	}
	if len(detail) > 0 {
		if b, err := json.Marshal(detail); err == nil {
			entry.Changes = string(b)
		}
	}
	s.Audit.Record(entry)
}

// AuditRejected registra un rechazo de seguridad (redirect no permitido,
// csrf inválido, PKCE fallido…).
func (s *Service) AuditRejected(user *models.User, ac AuditCtx, reason string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	detail["reason"] = reason
	s.audit("security.rejected", user, false, ac, "", detail)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ---------------------------------------------------------------------------
// Usuarios

// ErrUserInactive cubre usuario borrado, suspendido o sin acceso al MCP.
var ErrUserInactive = errors.New("usuario inactivo")

// ActiveUser trae al usuario y comprueba que sigue pudiendo entrar y que su
// rol tiene habilitado el MCP.
func (s *Service) ActiveUser(userID uint) (*models.User, error) {
	u, err := s.Users.GetByID(userID)
	if err != nil || u == nil {
		return nil, ErrUserInactive
	}
	if s.CanSignIn != nil {
		if err := s.CanSignIn(u); err != nil {
			return nil, ErrUserInactive
		}
	} else if !u.IsActive {
		return nil, ErrUserInactive
	}
	if !s.Quota(RoleKey(u)).Enabled {
		return nil, ErrUserInactive
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// Registro dinámico de clientes (RFC 7591)

type RegisterRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

func (s *Service) RegisterClient(req RegisterRequest, ac AuditCtx) (map[string]any, *OAuthError) {
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 10 {
		return nil, oauthErr(400, "invalid_redirect_uri", "Hace falta al menos una redirect_uri (máximo 10).")
	}
	extra := ParseExtraDomains(s.Settings().ExtraRedirectDomains)
	for _, uri := range req.RedirectURIs {
		if len(uri) > 2000 || !RedirectAllowed(uri, extra, s.Dev) {
			s.AuditRejected(nil, ac, "redirect_uri no permitida en el registro", map[string]any{"redirect_uri": truncate(uri, 300), "client_name": truncate(req.ClientName, 100)})
			return nil, oauthErr(400, "invalid_redirect_uri", "redirect_uri no permitida: "+truncate(uri, 200))
		}
	}
	method := req.TokenEndpointAuthMethod
	if method == "" {
		method = "none"
	}
	if method != "none" && method != "client_secret_post" {
		return nil, oauthErr(400, "invalid_client_metadata", "token_endpoint_auth_method admitidos: none, client_secret_post.")
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = "Asistente sin nombre"
	}
	urisJSON, _ := json.Marshal(req.RedirectURIs)
	client := models.MCPOAuthClient{
		ClientID:                prefixClient + randomString(16),
		ClientName:              truncate(name, 200),
		RedirectURIs:            string(urisJSON),
		TokenEndpointAuthMethod: method,
		CreatedAt:               s.now(),
	}
	var secret string
	if method == "client_secret_post" {
		secret = NewToken(prefixSecret)
		h := HashToken(secret)
		client.ClientSecretHash = &h
	}
	if err := s.DB.Create(&client).Error; err != nil {
		log.Printf("[MCP] registro de cliente: %v", err)
		return nil, oauthErr(500, "server_error", "No se pudo registrar el cliente.")
	}
	s.audit("mcp.client_registered", nil, true, ac, client.ClientID, map[string]any{"client_name": client.ClientName, "redirect_uris": req.RedirectURIs})
	resp := map[string]any{
		"client_id":                  client.ClientID,
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"client_name":                client.ClientName,
		"redirect_uris":              req.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": method,
		"scope":                      "read",
	}
	if secret != "" {
		resp["client_secret"] = secret
		resp["client_secret_expires_at"] = 0
	}
	return resp, nil
}

func (s *Service) Client(clientID string) (*models.MCPOAuthClient, error) {
	if clientID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var c models.MCPOAuthClient
	if err := s.DB.Where("client_id = ?", clientID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ClientHasRedirect compara la redirect_uri exacta contra las registradas.
func ClientHasRedirect(c *models.MCPOAuthClient, uri string) bool {
	var uris []string
	if json.Unmarshal([]byte(c.RedirectURIs), &uris) != nil {
		return false
	}
	for _, u := range uris {
		if u == uri {
			return true
		}
	}
	return false
}

// checkClientSecret valida client_secret_post. Los clientes públicos (none)
// no mandan secreto: los protege PKCE.
func checkClientSecret(c *models.MCPOAuthClient, secret string) bool {
	if c.TokenEndpointAuthMethod != "client_secret_post" {
		return true
	}
	return c.ClientSecretHash != nil && secret != "" && HashToken(secret) == *c.ClientSecretHash
}

// ---------------------------------------------------------------------------
// Códigos de autorización

func (s *Service) CreateCode(user *models.User, client *models.MCPOAuthClient, redirectURI, challenge string, ac AuditCtx) (string, error) {
	code := randomString(32)
	row := models.MCPAuthCode{
		CodeHash:      HashToken(code),
		ClientID:      client.ClientID,
		UserID:        user.ID,
		RedirectURI:   redirectURI,
		CodeChallenge: challenge,
		ExpiresAt:     s.now().Add(CodeTTL),
		CreatedAt:     s.now(),
	}
	if err := s.DB.Create(&row).Error; err != nil {
		return "", err
	}
	s.audit("mcp.authorized", user, true, ac, client.ClientID, map[string]any{"client_name": client.ClientName})
	return code, nil
}

// TokenResponse es la respuesta del endpoint de token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
	RefreshToken string
}

// ExchangeCode canjea un código (grant_type=authorization_code).
func (s *Service) ExchangeCode(req TokenRequest, ac AuditCtx) (*TokenResponse, *OAuthError) {
	invalid := oauthErr(400, "invalid_grant", "El código no es válido, ya se usó o venció.")
	if req.Code == "" || req.CodeVerifier == "" || req.ClientID == "" {
		return nil, oauthErr(400, "invalid_request", "Faltan code, code_verifier o client_id.")
	}
	client, err := s.Client(req.ClientID)
	if err != nil {
		return nil, oauthErr(401, "invalid_client", "Cliente desconocido.")
	}
	if !checkClientSecret(client, req.ClientSecret) {
		return nil, oauthErr(401, "invalid_client", "client_secret incorrecto.")
	}
	hash := HashToken(req.Code)
	var code models.MCPAuthCode
	if err := s.DB.Where("code_hash = ?", hash).First(&code).Error; err != nil {
		return nil, invalid
	}
	// Se quema ANTES de comprobar el resto: un código da un único intento,
	// acierte o no el verificador.
	now := s.now()
	res := s.DB.Model(&models.MCPAuthCode{}).Where("code_hash = ? AND used_at IS NULL", hash).Update("used_at", now)
	if res.Error != nil || res.RowsAffected != 1 {
		s.AuditRejected(nil, ac, "código de autorización reutilizado", map[string]any{"client_id": req.ClientID})
		return nil, invalid
	}
	if now.After(code.ExpiresAt) || code.ClientID != client.ClientID || code.RedirectURI != req.RedirectURI {
		s.AuditRejected(nil, ac, "código vencido o de otro cliente/redirect", map[string]any{"client_id": req.ClientID})
		return nil, invalid
	}
	if !VerifyPKCE(req.CodeVerifier, code.CodeChallenge) {
		s.AuditRejected(nil, ac, "PKCE fallido", map[string]any{"client_id": req.ClientID})
		return nil, oauthErr(400, "invalid_grant", "code_verifier no coincide (PKCE).")
	}
	user, err := s.ActiveUser(code.UserID)
	if err != nil {
		return nil, oauthErr(400, "invalid_grant", "El usuario ya no tiene acceso.")
	}
	conn := models.MCPConnection{
		UserID:     user.ID,
		ClientID:   client.ClientID,
		ClientName: client.ClientName,
		IP:         truncate(ac.IP, 64),
		UserAgent:  truncate(ac.UA, 400),
		CreatedAt:  now,
		ExpiresAt:  now.Add(ConnectionTTL),
	}
	var out *TokenResponse
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&conn).Error; err != nil {
			return err
		}
		var err error
		out, err = issuePair(tx, &conn, now)
		return err
	})
	if err != nil {
		log.Printf("[MCP] canje de código: %v", err)
		return nil, oauthErr(500, "server_error", "No se pudo emitir el token.")
	}
	s.audit("mcp.connected", user, true, ac, fmt.Sprint(conn.ID), map[string]any{"client_name": client.ClientName})
	return out, nil
}

// issuePair emite access (1 h) + refresh (30 días), sin pasar del
// vencimiento de la conexión.
func issuePair(tx *gorm.DB, conn *models.MCPConnection, now time.Time) (*TokenResponse, error) {
	access, refresh := NewToken(PrefixAccess), NewToken(PrefixRefresh)
	accessExp := minTime(now.Add(AccessTTL), conn.ExpiresAt)
	refreshExp := minTime(now.Add(RefreshTTL), conn.ExpiresAt)
	rows := []models.MCPToken{
		{ConnectionID: conn.ID, Kind: models.MCPTokenAccess, TokenHash: HashToken(access), ExpiresAt: accessExp, CreatedAt: now},
		{ConnectionID: conn.ID, Kind: models.MCPTokenRefresh, TokenHash: HashToken(refresh), ExpiresAt: refreshExp, CreatedAt: now},
	}
	if err := tx.Create(&rows).Error; err != nil {
		return nil, err
	}
	return &TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessExp.Sub(now).Seconds()),
		RefreshToken: refresh,
		Scope:        "read",
	}, nil
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Refresh rota el refresh token (grant_type=refresh_token). Un refresh ya
// usado que vuelve a llegar indica robo: se revoca toda la conexión.
func (s *Service) Refresh(req TokenRequest, ac AuditCtx) (*TokenResponse, *OAuthError) {
	invalid := oauthErr(400, "invalid_grant", "El refresh token no es válido.")
	if !strings.HasPrefix(req.RefreshToken, PrefixRefresh) {
		return nil, invalid
	}
	hash := HashToken(req.RefreshToken)
	var tok models.MCPToken
	if err := s.DB.Where("token_hash = ? AND kind = ?", hash, models.MCPTokenRefresh).First(&tok).Error; err != nil {
		return nil, invalid
	}
	var conn models.MCPConnection
	if err := s.DB.First(&conn, tok.ConnectionID).Error; err != nil {
		return nil, invalid
	}
	client, err := s.Client(req.ClientID)
	if err != nil || client.ClientID != conn.ClientID {
		return nil, oauthErr(401, "invalid_client", "El refresh token es de otro cliente.")
	}
	if !checkClientSecret(client, req.ClientSecret) {
		return nil, oauthErr(401, "invalid_client", "client_secret incorrecto.")
	}
	now := s.now()
	if tok.RevokedAt != nil || conn.RevokedAt != nil || now.After(tok.ExpiresAt) || now.After(conn.ExpiresAt) {
		return nil, invalid
	}
	user, _ := s.Users.GetByID(conn.UserID)
	if tok.UsedAt != nil {
		s.reuseDetected(&conn, user, ac)
		return nil, invalid
	}
	res := s.DB.Model(&models.MCPToken{}).Where("id = ? AND used_at IS NULL", tok.ID).Update("used_at", now)
	if res.Error != nil {
		return nil, oauthErr(500, "server_error", "No se pudo rotar el token.")
	}
	if res.RowsAffected != 1 {
		// Otra petición lo usó a la vez: mismo tratamiento que un reuso.
		s.reuseDetected(&conn, user, ac)
		return nil, invalid
	}
	if _, err := s.ActiveUser(conn.UserID); err != nil {
		return nil, oauthErr(400, "invalid_grant", "El usuario ya no tiene acceso.")
	}
	var out *TokenResponse
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = issuePair(tx, &conn, now)
		return err
	}); err != nil {
		return nil, oauthErr(500, "server_error", "No se pudo emitir el token.")
	}
	s.audit("mcp.token_refreshed", user, true, ac, fmt.Sprint(conn.ID), map[string]any{"client_name": conn.ClientName})
	return out, nil
}

func (s *Service) reuseDetected(conn *models.MCPConnection, user *models.User, ac AuditCtx) {
	_ = s.revokeConnection(conn.ID, nil, "reuso de refresh token: posible robo")
	s.audit("security.token_reuse", user, false, ac, fmt.Sprint(conn.ID), map[string]any{"client_name": conn.ClientName})
	if user != nil {
		s.AddAlert(user.ID, "token_reuse", fmt.Sprintf("Se reutilizó un refresh token de la conexión %q de %s; la conexión quedó revocada.", conn.ClientName, user.Email))
	}
}

// revokeConnection marca la conexión y todos sus tokens como revocados.
func (s *Service) revokeConnection(connID uint, by *uint, reason string) error {
	now := s.now()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.MCPConnection{}).Where("id = ? AND revoked_at IS NULL", connID).
			Updates(map[string]any{"revoked_at": now, "revoked_by": by, "revoke_reason": truncate(reason, 300)}).Error; err != nil {
			return err
		}
		return tx.Model(&models.MCPToken{}).Where("connection_id = ? AND revoked_at IS NULL", connID).Update("revoked_at", now).Error
	})
}

// RevokeByToken atiende /oauth/revoke (RFC 7009): siempre "ok" hacia fuera.
func (s *Service) RevokeByToken(token string, ac AuditCtx) {
	if tokenKind(token) == "" {
		return
	}
	var tok models.MCPToken
	if err := s.DB.Where("token_hash = ?", HashToken(token)).First(&tok).Error; err != nil {
		return
	}
	var conn models.MCPConnection
	if err := s.DB.First(&conn, tok.ConnectionID).Error; err != nil || conn.RevokedAt != nil {
		return
	}
	if err := s.revokeConnection(conn.ID, &conn.UserID, "revocado desde el cliente"); err != nil {
		return
	}
	user, _ := s.Users.GetByID(conn.UserID)
	s.audit("mcp.revoked", user, true, ac, fmt.Sprint(conn.ID), map[string]any{"client_name": conn.ClientName, "by": "cliente"})
}

// RevokeConnection revoca desde la web: el propio usuario (ownerID = su id)
// o un admin (ownerID = 0) con motivo.
func (s *Service) RevokeConnection(connID, ownerID uint, actor *models.User, reason string, ac AuditCtx) error {
	var conn models.MCPConnection
	q := s.DB.Where("id = ?", connID)
	if ownerID != 0 {
		q = q.Where("user_id = ?", ownerID)
	}
	if err := q.First(&conn).Error; err != nil {
		return err
	}
	if conn.RevokedAt != nil {
		return nil
	}
	by := actor.ID
	if reason == "" {
		reason = "revocado por el usuario"
	}
	if err := s.revokeConnection(conn.ID, &by, reason); err != nil {
		return err
	}
	who := "usuario"
	if ownerID == 0 {
		who = "admin"
	}
	owner, _ := s.Users.GetByID(conn.UserID)
	if owner == nil {
		owner = actor
	}
	s.audit("mcp.revoked", owner, true, ac, fmt.Sprint(conn.ID), map[string]any{"client_name": conn.ClientName, "by": who, "by_user": actor.Email, "reason": reason})
	return nil
}

// ---------------------------------------------------------------------------
// Tokens personales

// CreatePersonalToken crea una conexión "personal" con un token que se
// muestra una sola vez.
func (s *Service) CreatePersonalToken(user *models.User, name string, ac AuditCtx) (string, *models.MCPConnection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Token personal"
	}
	now := s.now()
	conn := models.MCPConnection{
		UserID:     user.ID,
		ClientID:   models.MCPPersonalClientID,
		ClientName: truncate(name, 200),
		IP:         truncate(ac.IP, 64),
		UserAgent:  truncate(ac.UA, 400),
		CreatedAt:  now,
		ExpiresAt:  now.Add(PersonalTTL),
	}
	token := NewToken(PrefixPersonal)
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&conn).Error; err != nil {
			return err
		}
		return tx.Create(&models.MCPToken{
			ConnectionID: conn.ID, Kind: models.MCPTokenPersonal, TokenHash: HashToken(token),
			ExpiresAt: conn.ExpiresAt, CreatedAt: now,
		}).Error
	})
	if err != nil {
		return "", nil, err
	}
	s.audit("mcp.personal_token_created", user, true, ac, fmt.Sprint(conn.ID), map[string]any{"name": conn.ClientName})
	return token, &conn, nil
}

// ---------------------------------------------------------------------------
// Resolución del Bearer

// Principal es quién llama al endpoint /api/mcp.
type Principal struct {
	User       *models.User
	Connection *models.MCPConnection
}

// ResolveBearer valida un access token o un token personal: prefijo, hash,
// no revocado, no vencido, conexión viva y usuario activo. Actualiza
// last_used_at como mucho una vez por minuto.
func (s *Service) ResolveBearer(token string) (*Principal, error) {
	kind := tokenKind(token)
	if kind != "access" && kind != "personal" {
		return nil, ErrInvalidToken
	}
	var tok models.MCPToken
	if err := s.DB.Where("token_hash = ? AND kind = ?", HashToken(token), kind).First(&tok).Error; err != nil {
		return nil, ErrInvalidToken
	}
	now := s.now()
	if tok.RevokedAt != nil || now.After(tok.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	var conn models.MCPConnection
	if err := s.DB.First(&conn, tok.ConnectionID).Error; err != nil {
		return nil, ErrInvalidToken
	}
	if conn.RevokedAt != nil || now.After(conn.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	user, err := s.ActiveUser(conn.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if conn.LastUsedAt == nil || now.Sub(*conn.LastUsedAt) > time.Minute {
		s.DB.Model(&models.MCPConnection{}).
			Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", conn.ID, now.Add(-time.Minute)).
			Update("last_used_at", now)
	}
	return &Principal{User: user, Connection: &conn}, nil
}

var ErrInvalidToken = errors.New("token inválido")
