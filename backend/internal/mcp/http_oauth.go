package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

// Handler expone el MCP por HTTP: metadata OAuth, Authorization Server, el
// endpoint /api/mcp y las rutas de la web (conexiones, tokens, panel).
type Handler struct {
	Svc       *Service
	RO        *ReadOnlyDB
	ScopeDeps ScopeDeps
	JWTSecret string
	TVGetter  middleware.TokenVersionGetter
	// BaseURL es el origen público (FRONTEND_URL): issuer y base de todos los
	// endpoints que se anuncian.
	BaseURL func() string
}

func (h *Handler) base() string { return strings.TrimRight(h.BaseURL(), "/") }

func auditCtx(c *gin.Context) AuditCtx {
	return AuditCtx{IP: c.ClientIP(), UA: c.Request.UserAgent(), Path: c.Request.URL.Path, Method: c.Request.Method}
}

// ---------------------------------------------------------------------------
// Metadata (RFC 8414 y RFC 9728)

func wellKnownHeaders(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, MCP-Protocol-Version")
}

// AuthorizationServerMetadata es el documento de RFC 8414.
func (h *Handler) AuthorizationServerMetadata() map[string]any {
	b := h.base()
	return map[string]any{
		"issuer":                                     b,
		"authorization_endpoint":                     b + "/api/mcp/oauth/authorize",
		"token_endpoint":                             b + "/api/mcp/oauth/token",
		"registration_endpoint":                      b + "/api/mcp/oauth/register",
		"revocation_endpoint":                        b + "/api/mcp/oauth/revoke",
		"response_types_supported":                   []string{"code"},
		"grant_types_supported":                      []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":           []string{"S256"},
		"token_endpoint_auth_methods_supported":      []string{"none", "client_secret_post"},
		"revocation_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"scopes_supported":                           []string{"read", "offline_access"},
		"service_documentation":                      b + "/integraciones",
	}
}

func (h *Handler) WellKnownAuthorizationServer(c *gin.Context) {
	wellKnownHeaders(c)
	c.JSON(http.StatusOK, h.AuthorizationServerMetadata())
}

func (h *Handler) WellKnownProtectedResource(c *gin.Context) {
	wellKnownHeaders(c)
	b := h.base()
	c.JSON(http.StatusOK, gin.H{
		"resource":                 b + "/api/mcp",
		"authorization_servers":    []string{b},
		"scopes_supported":         []string{"read"},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Obertrack",
		"resource_documentation":   b + "/integraciones",
	})
}

func WellKnownOptions(c *gin.Context) {
	wellKnownHeaders(c)
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Endpoint MCP

func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (h *Handler) unauthorized(c *gin.Context, withError bool) {
	v := fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource/api/mcp"`, h.base())
	if withError {
		v += `, error="invalid_token", error_description="El token no es válido, venció o fue revocado"`
	}
	c.Header("WWW-Authenticate", v)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token", "error_description": "Hace falta un token válido de Obertrack (Authorization: Bearer)."})
}

// MCP atiende POST/GET/DELETE /api/mcp en modo sin estado: un McpServer por
// petición con los tools del rol del token. Sin SSE persistente (GET y
// DELETE responden 405 una vez autenticado).
func (h *Handler) MCP(c *gin.Context) {
	token := bearerToken(c)
	if token == "" {
		h.unauthorized(c, false)
		return
	}
	p, err := h.Svc.ResolveBearer(token)
	if err != nil {
		h.unauthorized(c, true)
		return
	}
	scope, err := BuildScope(p.User, h.ScopeDeps)
	if err != nil {
		log.Printf("[MCP] alcance de usuario %d: %v", p.User.ID, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No se pudo calcular tu alcance."})
		return
	}
	rt := &Runtime{
		Svc: h.Svc, RO: h.RO, User: p.User, Conn: p.Connection, Scope: scope,
		Quota: h.Svc.Quota(scope.RoleKey), Settings: h.Svc.Settings(),
		FrontendURL: h.base(), Audit: auditCtx(c),
	}
	srv := NewServer(rt)
	gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return srv },
		&gomcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}).ServeHTTP(c.Writer, c.Request)
}

// ---------------------------------------------------------------------------
// Registro dinámico

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata", "error_description": "JSON inválido."})
		return
	}
	resp, oerr := h.Svc.RegisterClient(req, auditCtx(c))
	if oerr != nil {
		c.JSON(oerr.Status, gin.H{"error": oerr.Code, "error_description": oerr.Desc})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, resp)
}

// ---------------------------------------------------------------------------
// Sesión web

// sessionUser lee la sesión de la web (cookie access_token o, si venció,
// refresh_token) con las mismas reglas que AuthMiddleware: HS256, versión de
// token vigente y sin sesiones acotadas del CRM.
func (h *Handler) sessionUser(c *gin.Context) *models.User {
	for _, name := range []string{"access_token", "refresh_token"} {
		raw, err := c.Cookie(name)
		if err != nil || raw == "" {
			continue
		}
		claims := &middleware.Claims{}
		tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			return []byte(h.JWTSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !tok.Valid || claims.Scope != "" {
			continue
		}
		wantType := "access"
		if name == "refresh_token" {
			wantType = "refresh"
		}
		if claims.TokenType != wantType {
			continue
		}
		if h.TVGetter != nil {
			if tv, err := h.TVGetter(claims.UserID); err != nil || tv != claims.TokenVersion {
				continue
			}
		}
		u, err := h.Svc.Users.GetByID(claims.UserID)
		if err == nil && u != nil {
			return u
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// /authorize

type authorizeParams struct {
	ClientID, RedirectURI, ResponseType, State, Challenge, Method, Scope string
}

func readAuthorizeParams(get func(string) string) authorizeParams {
	return authorizeParams{
		ClientID:     get("client_id"),
		RedirectURI:  get("redirect_uri"),
		ResponseType: get("response_type"),
		State:        get("state"),
		Challenge:    get("code_challenge"),
		Method:       get("code_challenge_method"),
		Scope:        get("scope"),
	}
}

// redirectWith devuelve al cliente con los parámetros dados.
func redirectWith(c *gin.Context, redirectURI string, params map[string]string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		renderError(c, http.StatusBadRequest, "La redirect_uri no es válida.")
		return
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, u.String())
}

// validateClient comprueba cliente y redirect_uri exacta. Si fallan, NO se
// redirige (podría ser un redirect abierto): se muestra un error aquí.
func (h *Handler) validateClient(c *gin.Context, p authorizeParams, user *models.User) (*models.MCPOAuthClient, bool) {
	client, err := h.Svc.Client(p.ClientID)
	if err != nil {
		renderError(c, http.StatusBadRequest, "Este asistente no está registrado en Obertrack. Vuelve a agregar el conector desde tu asistente.")
		return nil, false
	}
	if !ClientHasRedirect(client, p.RedirectURI) {
		h.Svc.AuditRejected(user, auditCtx(c), "redirect_uri distinta de las registradas", map[string]any{"client_id": p.ClientID, "redirect_uri": truncate(p.RedirectURI, 300)})
		renderError(c, http.StatusBadRequest, "La dirección de retorno no coincide con la registrada por este asistente.")
		return nil, false
	}
	return client, true
}

// validateRequest valida lo que, si falla, se le devuelve al cliente por su
// redirect_uri (ya verificada).
func validateRequest(p authorizeParams) string {
	switch {
	case p.ResponseType != "code":
		return "response_type debe ser code"
	case p.Challenge == "" || p.Method != "S256":
		return "PKCE obligatorio: code_challenge con code_challenge_method=S256"
	case len(p.Challenge) < 43 || len(p.Challenge) > 128:
		return "code_challenge inválido"
	}
	// El scope NO se valida: siempre se concede "read" (y la respuesta de
	// token lo dice), pida lo que pida el cliente. ChatGPT pide
	// "offline_access" para recibir refresh tokens y otros clientes añaden
	// openid/profile; rechazarlos rompería la conexión a la primera, y
	// ignorarlos no da más acceso (RFC 6749 §3.3 permite conceder menos).
	return ""
}

// AuthorizeGet muestra la página de consentimiento.
func (h *Handler) AuthorizeGet(c *gin.Context) {
	p := readAuthorizeParams(c.Query)
	user := h.sessionUser(c)
	if user == nil {
		returnTo := c.Request.URL.Path + "?" + c.Request.URL.RawQuery
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusFound, h.base()+"/login?returnTo="+url.QueryEscape(returnTo))
		return
	}
	client, ok := h.validateClient(c, p, user)
	if !ok {
		return
	}
	if msg := validateRequest(p); msg != "" {
		redirectWith(c, p.RedirectURI, map[string]string{"error": "invalid_request", "error_description": msg, "state": p.State})
		return
	}
	active, err := h.Svc.ActiveUser(user.ID)
	if err != nil {
		renderError(c, http.StatusForbidden, "Tu usuario no tiene habilitado el acceso de asistentes de IA. Pídeselo a un administrador.")
		return
	}
	if h.Svc.Settings().Paused {
		renderError(c, http.StatusServiceUnavailable, "El acceso de asistentes de IA está pausado por un administrador.")
		return
	}
	scope, err := BuildScope(active, h.ScopeDeps)
	if err != nil {
		renderError(c, http.StatusInternalServerError, "No se pudo calcular tu alcance. Inténtalo de nuevo.")
		return
	}
	csrf := NewConsentToken(h.JWTSecret, active.ID, client.ClientID, p.RedirectURI, p.Challenge, time.Now())
	renderConsent(c, consentData{
		ClientName:   client.ClientName,
		RedirectHost: hostOf(p.RedirectURI),
		UserName:     active.Name,
		UserEmail:    active.Email,
		Role:         RoleLabels[scope.RoleKey],
		ScopeText:    scope.Describe(),
		Params:       p,
		CSRF:         csrf,
		Dev:          h.Svc.Dev,
	})
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Host != "" {
		return u.Host
	}
	return u.Scheme + ":"
}

// AuthorizePost recibe Autorizar / Cancelar.
func (h *Handler) AuthorizePost(c *gin.Context) {
	ac := auditCtx(c)
	// Sin Origin (va null por no-referrer): solo se rechaza de plano lo que el
	// navegador marca como cross-site; el resto lo decide el csrf firmado.
	if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		h.Svc.AuditRejected(nil, ac, "POST de consentimiento cross-site", nil)
		renderError(c, http.StatusForbidden, "Solicitud no permitida.")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		renderError(c, http.StatusBadRequest, "Formulario inválido.")
		return
	}
	p := readAuthorizeParams(c.Request.PostForm.Get)
	user := h.sessionUser(c)
	if user == nil {
		renderError(c, http.StatusUnauthorized, "Tu sesión venció. Vuelve a empezar la conexión desde tu asistente.")
		return
	}
	client, ok := h.validateClient(c, p, user)
	if !ok {
		return
	}
	if !VerifyConsentToken(h.JWTSecret, c.Request.PostForm.Get("csrf"), user.ID, client.ClientID, p.RedirectURI, p.Challenge, time.Now()) {
		h.Svc.AuditRejected(user, ac, "csrf de consentimiento inválido o vencido", map[string]any{"client_id": client.ClientID})
		renderError(c, http.StatusForbidden, "La página de autorización venció o no es válida. Vuelve a empezar la conexión desde tu asistente.")
		return
	}
	if c.Request.PostForm.Get("decision") != "allow" {
		redirectWith(c, p.RedirectURI, map[string]string{"error": "access_denied", "error_description": "El usuario canceló la autorización", "state": p.State})
		return
	}
	if msg := validateRequest(p); msg != "" {
		redirectWith(c, p.RedirectURI, map[string]string{"error": "invalid_request", "error_description": msg, "state": p.State})
		return
	}
	active, err := h.Svc.ActiveUser(user.ID)
	if err != nil {
		renderError(c, http.StatusForbidden, "Tu usuario no tiene habilitado el acceso de asistentes de IA.")
		return
	}
	code, err := h.Svc.CreateCode(active, client, p.RedirectURI, p.Challenge, ac)
	if err != nil {
		log.Printf("[MCP] crear código: %v", err)
		renderError(c, http.StatusInternalServerError, "No se pudo completar la autorización. Inténtalo de nuevo.")
		return
	}
	redirectWith(c, p.RedirectURI, map[string]string{"code": code, "state": p.State, "iss": h.base()})
}

// ---------------------------------------------------------------------------
// /token y /revoke

func tokenError(c *gin.Context, e *OAuthError) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(e.Status, gin.H{"error": e.Code, "error_description": e.Desc})
}

func (h *Handler) Token(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.Request.ParseForm(); err != nil {
		tokenError(c, oauthErr(400, "invalid_request", "Cuerpo inválido: se espera application/x-www-form-urlencoded."))
		return
	}
	f := c.Request.PostForm
	req := TokenRequest{
		GrantType:    f.Get("grant_type"),
		Code:         f.Get("code"),
		RedirectURI:  f.Get("redirect_uri"),
		ClientID:     f.Get("client_id"),
		ClientSecret: f.Get("client_secret"),
		CodeVerifier: f.Get("code_verifier"),
		RefreshToken: f.Get("refresh_token"),
	}
	var (
		resp *TokenResponse
		oerr *OAuthError
	)
	switch req.GrantType {
	case "authorization_code":
		resp, oerr = h.Svc.ExchangeCode(req, auditCtx(c))
	case "refresh_token":
		resp, oerr = h.Svc.Refresh(req, auditCtx(c))
	default:
		oerr = oauthErr(400, "unsupported_grant_type", "grant_type admitidos: authorization_code, refresh_token.")
	}
	if oerr != nil {
		tokenError(c, oerr)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, resp)
}

// Revoke (RFC 7009) responde 200 siempre, exista o no el token.
func (h *Handler) Revoke(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.Request.ParseForm(); err == nil {
		h.Svc.RevokeByToken(c.Request.PostForm.Get("token"), auditCtx(c))
	}
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
}
