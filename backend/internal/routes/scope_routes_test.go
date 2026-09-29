package routes

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/middleware"
)

const scopeTestSecret = "scope-test-secret"

// scopeTestRouter monta el router real con dependencias vacías. Nada llega a
// ejecutarse de verdad: las rutas que el alcance rechaza cortan antes del
// handler, y las que deja pasar revientan contra una dependencia nil, que
// Recovery convierte en 500. Lo que se mira es solo si el alcance cortó.
func scopeTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// Los pánicos recuperados y los "Missing token" son esperados aquí; sin
	// silenciarlos, la salida del test son miles de líneas de pila.
	prevLog, prevErr := log.Writer(), gin.DefaultErrorWriter
	log.SetOutput(io.Discard)
	gin.DefaultErrorWriter = io.Discard
	t.Cleanup(func() { log.SetOutput(prevLog); gin.DefaultErrorWriter = prevErr })

	r := gin.New()
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) { c.AbortWithStatus(http.StatusInternalServerError) }))
	mountRoutes(r, &deps{cfg: &config.Config{JWTSecret: scopeTestSecret}})
	return r
}

func scopedToken(t *testing.T, scope string) string {
	t.Helper()
	tenant := uint(7)
	claims := middleware.Claims{
		UserID:    99,
		TenantID:  &tenant,
		Email:     "vendedor@oberstaff.com",
		Role:      "profesional",
		TokenType: "access",
		Scope:     scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(scopeTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

var routeParam = regexp.MustCompile(`[:*][A-Za-z_]+`)

func call(r *gin.Engine, method, pattern, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, routeParam.ReplaceAllString(pattern, "1"), nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// rejectedByScope distingue el 403 del alcance de cualquier otro 403 (RBAC,
// porteros propios de cada módulo).
func rejectedByScope(w *httptest.ResponseRecorder) bool {
	return w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "Esta sesión no tiene acceso")
}

// Cada entrada de la lista tiene que existir: una ruta renombrada dejaría la
// lista desfasada en silencio y la vista embebida rota.
func TestScopeAllowlistRoutesExist(t *testing.T) {
	r := scopeTestRouter(t)
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for scope, routes := range scopeAllowlist {
		for route := range routes {
			if !registered[route] {
				t.Errorf("alcance %q: la ruta %q de la lista no existe en el router", scope, route)
			}
		}
	}
}

// Recorre TODAS las rutas protegidas: con un token "tasks", las de la lista
// pasan el alcance y todas las demás (websockets incluidos) dan 403. Una ruta
// nueva queda cerrada a las sesiones acotadas sin que nadie tenga que acordarse.
func TestScopeTasksOnlyReachesAllowlist(t *testing.T) {
	r := scopeTestRouter(t)
	token := scopedToken(t, middleware.ScopeTasks)
	allowed := scopeAllowlist[middleware.ScopeTasks]

	protected := 0
	for _, ri := range r.Routes() {
		// Las rutas públicas no pasan por la autenticación y el alcance no se
		// les aplica. Se reconocen por el mensaje de AuthMiddleware, no solo
		// por el 401: /auth/refresh es pública y también da 401 sin cookie.
		if w := call(r, ri.Method, ri.Path, ""); w.Code != http.StatusUnauthorized ||
			!strings.Contains(w.Body.String(), "Authorization token required") {
			continue
		}
		protected++
		key := ri.Method + " " + ri.Path
		w := call(r, ri.Method, ri.Path, token)
		if allowed[key] && rejectedByScope(w) {
			t.Errorf("%s está en la lista y el alcance la rechazó", key)
		}
		if !allowed[key] && !rejectedByScope(w) {
			t.Errorf("%s NO está en la lista y el alcance la dejó pasar (status %d)", key, w.Code)
		}
	}
	if protected < 100 {
		t.Fatalf("solo se recorrieron %d rutas protegidas: el montaje del router de prueba no es el real", protected)
	}
}

func TestUnknownScopeIsRejected(t *testing.T) {
	r := scopeTestRouter(t)
	w := call(r, http.MethodGet, "/api/tasks", scopedToken(t, "otro"))
	if !rejectedByScope(w) {
		t.Fatalf("un alcance desconocido debe dar 403, got %d", w.Code)
	}
}

// Una sesión completa no se ve afectada por el alcance.
func TestFullSessionIgnoresScope(t *testing.T) {
	r := scopeTestRouter(t)
	for _, pattern := range []string{"/api/work-hours", "/api/tasks"} {
		if w := call(r, http.MethodGet, pattern, scopedToken(t, "")); rejectedByScope(w) {
			t.Errorf("%s: una sesión sin alcance no debe pasar por la lista", pattern)
		}
	}
}
