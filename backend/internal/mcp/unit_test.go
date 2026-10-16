package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/obertrack/backend/internal/models"
)

// ---------------------------------------------------------------------------
// Tokens y PKCE

func challengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestPKCE(t *testing.T) {
	verifier := strings.Repeat("a", 43) + "XYZ-._~"
	ch := challengeFor(verifier)
	if !VerifyPKCE(verifier, ch) {
		t.Fatal("un verificador correcto debe pasar")
	}
	if VerifyPKCE(verifier+"x", ch) {
		t.Fatal("un verificador distinto no debe pasar")
	}
	if VerifyPKCE("corto", challengeFor("corto")) {
		t.Fatal("un verificador de menos de 43 caracteres no debe pasar")
	}
	if VerifyPKCE(verifier, "") {
		t.Fatal("sin code_challenge no debe pasar")
	}
}

func TestNewTokenFormat(t *testing.T) {
	for _, p := range []string{PrefixAccess, PrefixRefresh, PrefixPersonal} {
		tok := NewToken(p)
		if !strings.HasPrefix(tok, p) || len(tok) != len(p)+43 {
			t.Fatalf("token %q: se esperaba prefijo %s + 43 caracteres", tok, p)
		}
	}
	if NewToken(PrefixAccess) == NewToken(PrefixAccess) {
		t.Fatal("dos tokens no pueden coincidir")
	}
	if tokenKind("obs_xx_abc") != "" || tokenKind("eyJhbGciOi") != "" {
		t.Fatal("un token ajeno no tiene tipo")
	}
}

// ---------------------------------------------------------------------------
// redirect_uri

func TestRedirectAllowed(t *testing.T) {
	cases := []struct {
		uri  string
		dev  bool
		want bool
	}{
		{"https://chatgpt.com/connector_platform_oauth_redirect", false, true},
		{"https://claude.ai/api/mcp/auth_callback", false, true},
		{"https://sub.claude.com/cb", false, true},
		{"https://grok.com/oauth/cb", false, true},
		{"claude://oauth/callback", false, true},
		{"vscode://cb", false, true},
		{"https://claude.ai.evil.com/cb", false, false},
		{"https://evilclaude.ai/cb", false, false},
		{"http://claude.ai/cb", false, false},
		{"https://claude.ai/cb#frag", false, false},
		{"https://user:pass@claude.ai/cb", false, false},
		{"javascript:alert(1)", false, false},
		{"http://localhost:6274/oauth/callback", false, false},
		{"http://localhost:6274/oauth/callback", true, true},
		{"http://127.0.0.1:7777/cb", true, true},
		{"http://10.0.0.1/cb", true, false},
	}
	for _, c := range cases {
		if got := RedirectAllowed(c.uri, nil, c.dev); got != c.want {
			t.Errorf("RedirectAllowed(%q, dev=%v) = %v, quería %v", c.uri, c.dev, got, c.want)
		}
	}
	extra := ParseExtraDomains("https://mi-asistente.example.com/, *.otra.io\nmal/dominio")
	if !RedirectAllowed("https://mi-asistente.example.com/cb", extra, false) || !RedirectAllowed("https://x.otra.io/cb", extra, false) {
		t.Fatal("los dominios extra del admin deben admitirse")
	}
	if len(extra) != 2 {
		t.Fatalf("ParseExtraDomains debe descartar entradas con ruta: %v", extra)
	}
}

// ---------------------------------------------------------------------------
// csrf del consentimiento

func TestConsentToken(t *testing.T) {
	const secret = "una-clave-de-pruebas-larga-de-32-bytes!!"
	now := time.Now()
	tok := NewConsentToken(secret, 7, "obc_x", "https://claude.ai/cb", "ch", now)
	if !VerifyConsentToken(secret, tok, 7, "obc_x", "https://claude.ai/cb", "ch", now) {
		t.Fatal("el csrf recién firmado debe valer")
	}
	if VerifyConsentToken(secret, tok, 7, "obc_x", "https://claude.ai/cb", "ch", now.Add(ConsentTTL+time.Second)) {
		t.Fatal("un csrf vencido no debe valer")
	}
	if VerifyConsentToken(secret, tok, 8, "obc_x", "https://claude.ai/cb", "ch", now) {
		t.Fatal("un csrf de otro usuario no debe valer")
	}
	if VerifyConsentToken(secret, tok, 7, "obc_y", "https://claude.ai/cb", "ch", now) ||
		VerifyConsentToken(secret, tok, 7, "obc_x", "https://claude.ai/otro", "ch", now) ||
		VerifyConsentToken(secret, tok, 7, "obc_x", "https://claude.ai/cb", "otro", now) {
		t.Fatal("el csrf está atado a cliente, redirect y challenge")
	}
	if VerifyConsentToken("otra-clave-de-pruebas-larga-de-32-bytes!", tok, 7, "obc_x", "https://claude.ai/cb", "ch", now) {
		t.Fatal("con otra clave no debe valer")
	}
	// Alargar la fecha invalida la firma.
	_, sig, _ := strings.Cut(tok, ".")
	forged := "99999999999." + sig
	if VerifyConsentToken(secret, forged, 7, "obc_x", "https://claude.ai/cb", "ch", now) {
		t.Fatal("cambiar la fecha debe invalidar la firma")
	}
}

// ---------------------------------------------------------------------------
// Política de datos

func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("+58 412-555-1234"); got != "•••1234" {
		t.Fatalf("MaskPhone = %q", got)
	}
	if MaskPhone("") != "" || MaskPhone("12") != "•••" {
		t.Fatal("teléfono vacío o corto")
	}
}

func TestRedactData(t *testing.T) {
	in := map[string]any{
		"name":               "Ana",
		"phone":              "+1 305 555 9876",
		"professional_phone": "04125551111",
		"description":        "texto interno",
		"items": []map[string]any{
			{"phone_number": "5551234567", "activities": "hice cosas", "title": strings.Repeat("x", 300)},
		},
	}
	full := RedactData(in, false).(map[string]any)
	if full["phone"] != "•••9876" || full["professional_phone"] != "•••1111" {
		t.Fatalf("teléfonos sin enmascarar: %v", full)
	}
	if full["description"] != "texto interno" {
		t.Fatal("fuera de modo mínimo, la descripción se mantiene")
	}
	item := full["items"].([]any)[0].(map[string]any)
	if item["phone_number"] != "•••4567" {
		t.Fatalf("teléfono anidado sin enmascarar: %v", item)
	}

	min := RedactData(in, true).(map[string]any)
	if _, ok := min["description"]; ok {
		t.Fatal("modo mínimo debe quitar la descripción")
	}
	item = min["items"].([]any)[0].(map[string]any)
	if _, ok := item["activities"]; ok {
		t.Fatal("modo mínimo debe quitar actividades")
	}
	if l := len([]rune(item["title"].(string))); l != longTextLimit+1 {
		t.Fatalf("modo mínimo debe recortar textos largos, quedó de %d", l)
	}
	if in["phone"] != "+1 305 555 9876" {
		t.Fatal("RedactData no debe modificar la entrada")
	}
}

// ---------------------------------------------------------------------------
// Alcance por rol

func scopeFor(role string) *Scope {
	s := &Scope{UserID: 10, RoleKey: role}
	switch role {
	case RoleSuperadmin, RoleCustomerSuccess:
		s.CrossCompany = true
	case RoleEmployer:
		s.TenantID = 10
	case RoleManager, RoleSupervisor:
		s.TenantID, s.IsManager, s.TeamMode = 3, true, "direct"
	case RoleProfessional:
		s.TenantID = 3
	}
	return s
}

func TestHoursFilterByRole(t *testing.T) {
	if _, err := scopeFor(RoleSuperadmin).HoursFilter(0); err == nil {
		t.Fatal("plataforma sin company_id debe pedir empresa, como la web")
	}
	w, err := scopeFor(RoleSuperadmin).HoursFilter(5)
	if err != nil || !strings.Contains(w.SQL(), "wh.tenant_id = ?") || w.args[0] != uint(5) {
		t.Fatalf("plataforma con empresa: %v %v", w, err)
	}
	if _, err := scopeFor(RoleEmployer).HoursFilter(99); err == nil {
		t.Fatal("una empresa no puede leer otra empresa")
	}
	w, _ = scopeFor(RoleEmployer).HoursFilter(0)
	if w.args[0] != uint(10) {
		t.Fatalf("la empresa queda en su tenant: %v", w.args)
	}
	w, _ = scopeFor(RoleManager).HoursFilter(0)
	if !strings.Contains(w.SQL(), "e_md.manager_id = ?") {
		t.Fatalf("un manager queda en su equipo directo: %s", w.SQL())
	}
	tree := scopeFor(RoleSupervisor)
	tree.TeamMode, tree.TeamIDs = "tree", []uint{}
	w, _ = tree.HoursFilter(0)
	if !strings.Contains(w.SQL(), "1 = 0") {
		t.Fatal("un árbol vacío no puede caer a todo el tenant")
	}
	w, _ = scopeFor(RoleProfessional).HoursFilter(0)
	if !strings.Contains(w.SQL(), "wh.user_id = ?") || w.args[0] != uint(10) {
		t.Fatalf("un profesional ve solo lo suyo: %s %v", w.SQL(), w.args)
	}
	if _, err := scopeFor(RoleIT).HoursFilter(0); err == nil {
		t.Fatal("IT no ve jornadas")
	}
}

func TestTasksAndBoardsFilterByRole(t *testing.T) {
	w, _ := scopeFor(RoleProfessional).TasksFilter(0)
	if !strings.Contains(w.SQL(), "board_members") {
		t.Fatal("un profesional solo ve tareas de sus tableros")
	}
	w, _ = scopeFor(RoleManager).TasksFilter(0)
	if strings.Contains(w.SQL(), "board_members") {
		t.Fatal("un manager ve todas las tareas del tenant, como en la web")
	}
	w, _ = scopeFor(RoleManager).BoardsFilter(0)
	if !strings.Contains(w.SQL(), "board_members") {
		t.Fatal("en tableros, el manager ve solo los suyos, como en la web")
	}
	w, _ = scopeFor(RoleEmployer).BoardsFilter(0)
	if strings.Contains(w.SQL(), "board_members") {
		t.Fatal("la empresa ve todos sus tableros")
	}
	if _, err := scopeFor(RoleProfessional).PeopleFilter(0); err == nil {
		t.Fatal("un profesional no lista profesionales")
	}
	if _, err := scopeFor(RoleManager).CompaniesFilter(); err == nil {
		t.Fatal("un manager no lista empresas")
	}
}

func TestCanModule(t *testing.T) {
	s := scopeFor(RoleProfessional)
	if !s.CanModule("hours") {
		t.Fatal("sin roles RBAC asignados, pasa (comportamiento histórico)")
	}
	s.HasRBACRoles, s.Perms = true, map[string]string{"hours": "none", "tasks": "view"}
	if s.CanModule("hours") || !s.CanModule("tasks") {
		t.Fatal("con roles, hace falta al menos view")
	}
}

func TestRoleKey(t *testing.T) {
	cases := []struct {
		u    models.User
		want string
	}{
		{models.User{IsSuperadmin: true, UserType: models.UserTypeSuperadmin}, RoleSuperadmin},
		{models.User{UserType: models.UserTypeCustomerSuccess}, RoleCustomerSuccess},
		{models.User{UserType: models.UserTypeITAnalyst}, RoleIT},
		{models.User{UserType: models.UserTypeEmployer}, RoleEmployer},
		{models.User{UserType: models.UserTypeProfessional, IsManager: true, IsSupervisor: true}, RoleSupervisor},
		{models.User{UserType: models.UserTypeProfessional, IsManager: true}, RoleManager},
		{models.User{UserType: models.UserTypeProfessional}, RoleProfessional},
	}
	for _, c := range cases {
		if got := RoleKey(&c.u); got != c.want {
			t.Errorf("RoleKey(%+v) = %s, quería %s", c.u, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tools registrados por rol (tools/list)

func listTools(t *testing.T, s *Scope) []string {
	t.Helper()
	rt := &Runtime{Scope: s, User: &models.User{ID: s.UserID}, Conn: &models.MCPConnection{}}
	srv := NewServer(rt)
	ctx := context.Background()
	st, ct := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := gomcp.NewClient(&gomcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatalf("el tool %s debe declarar readOnlyHint", tool.Name)
		}
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func TestToolsPerRole(t *testing.T) {
	has := func(names []string, n string) bool {
		for _, x := range names {
			if x == n {
				return true
			}
		}
		return false
	}
	super := listTools(t, scopeFor(RoleSuperadmin))
	for _, n := range []string{"whoami", "general_overview", "list_companies", "list_tickets", "list_incidents", "list_tasks", "search", "fetch"} {
		if !has(super, n) {
			t.Errorf("superadmin debería tener %s: %v", n, super)
		}
	}
	it := listTools(t, scopeFor(RoleIT))
	if strings.Join(it, ",") != "general_overview,whoami" {
		t.Errorf("IT solo ve totales agregados: %v", it)
	}
	cs := listTools(t, scopeFor(RoleCustomerSuccess))
	if has(cs, "list_tickets") || !has(cs, "list_incidents") {
		t.Errorf("CS: incidentes sí, tickets no (en la web son solo de superadmin): %v", cs)
	}
	pro := listTools(t, scopeFor(RoleProfessional))
	for _, n := range []string{"list_companies", "list_professionals", "list_tickets", "general_overview", "list_incidents"} {
		if has(pro, n) {
			t.Errorf("un profesional no debe ver %s", n)
		}
	}
	if !has(pro, "list_work_hours") || !has(pro, "list_tasks") {
		t.Errorf("un profesional ve sus jornadas y tareas: %v", pro)
	}
	noHours := scopeFor(RoleProfessional)
	noHours.HasRBACRoles, noHours.Perms = true, map[string]string{"hours": "none", "tasks": "edit"}
	if names := listTools(t, noHours); has(names, "list_work_hours") || has(names, "hours_summary") {
		t.Errorf("sin permiso RBAC de horas no aparecen los tools de horas: %v", names)
	}
	mgr := listTools(t, scopeFor(RoleManager))
	if !has(mgr, "list_professionals") || has(mgr, "list_companies") {
		t.Errorf("manager: su equipo sí, empresas no: %v", mgr)
	}
}

// ---------------------------------------------------------------------------
// Metadata OAuth

func TestWellKnownMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{BaseURL: func() string { return "https://obertrack.com/" }}
	r := gin.New()
	r.GET("/.well-known/oauth-authorization-server", h.WellKnownAuthorizationServer)
	r.GET("/.well-known/oauth-protected-resource/api/mcp", h.WellKnownProtectedResource)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("metadata: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" || w.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatal("la metadata va con CORS abierto y caché de una hora")
	}
	var meta map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta["issuer"] != "https://obertrack.com" {
		t.Fatalf("issuer = %v", meta["issuer"])
	}
	for _, k := range []string{"authorization_endpoint", "token_endpoint", "registration_endpoint", "revocation_endpoint"} {
		v, _ := meta[k].(string)
		if !strings.HasPrefix(v, "https://obertrack.com/api/mcp/oauth/") {
			t.Errorf("%s debe ser absoluto: %q", k, v)
		}
	}
	if m := meta["code_challenge_methods_supported"].([]any); len(m) != 1 || m[0] != "S256" {
		t.Fatalf("solo S256: %v", m)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/api/mcp", nil))
	var pr map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &pr)
	if pr["resource"] != "https://obertrack.com/api/mcp" {
		t.Fatalf("resource = %v", pr["resource"])
	}
}

func TestMCPWithoutTokenIs401WithResourceMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{BaseURL: func() string { return "https://obertrack.com" }}
	r := gin.New()
	r.POST("/api/mcp", h.MCP)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader("{}")))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("sin token: %d", w.Code)
	}
	want := `Bearer resource_metadata="https://obertrack.com/.well-known/oauth-protected-resource/api/mcp"`
	if w.Header().Get("WWW-Authenticate") != want {
		t.Fatalf("WWW-Authenticate = %q", w.Header().Get("WWW-Authenticate"))
	}
}

func TestAuthorizeAcceptsClientScopes(t *testing.T) {
	base := authorizeParams{ResponseType: "code", Challenge: strings.Repeat("c", 43), Method: "S256"}
	for _, scope := range []string{"", "read", "read offline_access", "offline_access", "openid profile email"} {
		p := base
		p.Scope = scope
		if msg := validateRequest(p); msg != "" {
			t.Errorf("scope %q no debe romper la autorización: %s", scope, msg)
		}
	}
	p := base
	p.Method = "plain"
	if validateRequest(p) == "" {
		t.Fatal("PKCE plain sigue rechazado")
	}
}
