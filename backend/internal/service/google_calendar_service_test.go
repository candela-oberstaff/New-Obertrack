package service

import (
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/middleware"
)

const testJWTSecret = "un-secreto-de-sesion-de-mas-de-32-bytes-para-pruebas"

func newTestGoogleService(t *testing.T) *googleCalendarService {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("no se pudo generar la clave: %v", err)
	}
	cfg := &config.Config{
		JWTSecret:             testJWTSecret,
		GoogleCalendarEnabled: true,
		GoogleClientID:        "client-id-de-prueba.apps.googleusercontent.com",
		GoogleClientSecret:    "secreto-de-prueba",
		GoogleRedirectURI:     "https://app.obertrack.test/api/integrations/google/callback",
		GoogleTokenEncKey:     base64.StdEncoding.EncodeToString(key),
	}
	svc, ok := NewGoogleCalendarService(nil, cfg).(*googleCalendarService)
	if !ok {
		t.Fatal("NewGoogleCalendarService no devolvió *googleCalendarService")
	}
	if !svc.Enabled() {
		t.Fatal("el servicio debería estar habilitado con la config de prueba")
	}
	return svc
}

// Con el flag apagado el servicio queda inerte en vez de nil, y todo responde
// ErrGoogleDisabled para que los handlers puedan devolver 503 sin comprobar nil.
func TestDisabledServiceIsInert(t *testing.T) {
	svc := NewGoogleCalendarService(nil, &config.Config{JWTSecret: testJWTSecret})
	if svc.Enabled() {
		t.Fatal("sin GOOGLE_CALENDAR_ENABLED el servicio no debe estar habilitado")
	}
	if _, err := svc.AuthURL(1, "/perfil"); err != ErrGoogleDisabled {
		t.Errorf("AuthURL deshabilitado: se esperaba ErrGoogleDisabled, se obtuvo %v", err)
	}
	if _, _, err := svc.HandleCallback("code", "state"); err != ErrGoogleDisabled {
		t.Errorf("HandleCallback deshabilitado: se esperaba ErrGoogleDisabled, se obtuvo %v", err)
	}
	if err := svc.Disconnect(1); err != ErrGoogleDisabled {
		t.Errorf("Disconnect deshabilitado: se esperaba ErrGoogleDisabled, se obtuvo %v", err)
	}
}

// access_type=offline y prompt=consent son obligatorios: sin ellos Google deja
// de devolver refresh_token en el segundo consentimiento de una misma cuenta y
// la integración se rompe semanas más tarde, sin señal al conectar.
func TestAuthURLCarriesOfflineConsent(t *testing.T) {
	svc := newTestGoogleService(t)

	raw, err := svc.AuthURL(42, "/perfil")
	if err != nil {
		t.Fatalf("AuthURL: %v", err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("URL inválida: %v", err)
	}
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != googleAuthEndpoint {
		t.Errorf("endpoint = %q, se esperaba %q", got, googleAuthEndpoint)
	}

	q := parsed.Query()
	for param, want := range map[string]string{
		"access_type":   "offline",
		"prompt":        "consent",
		"response_type": "code",
		"client_id":     svc.clientID,
		"redirect_uri":  svc.redirectURI,
		"scope":         googleScopes,
	} {
		if got := q.Get(param); got != want {
			t.Errorf("%s = %q, se esperaba %q", param, got, want)
		}
	}
	if q.Get("state") == "" {
		t.Error("la URL de consentimiento no lleva state")
	}
}

func TestStateRoundTrip(t *testing.T) {
	svc := newTestGoogleService(t)

	state, err := svc.buildState(7, "/perfil?tab=integraciones")
	if err != nil {
		t.Fatalf("buildState: %v", err)
	}
	claims, err := svc.parseState(state)
	if err != nil {
		t.Fatalf("parseState: %v", err)
	}
	if claims.UserID != 7 {
		t.Errorf("UserID = %d, se esperaba 7", claims.UserID)
	}
	if claims.ReturnTo != "/perfil?tab=integraciones" {
		t.Errorf("ReturnTo = %q", claims.ReturnTo)
	}
}

// EL test de seguridad central: el state viaja por la URL (queda en historial,
// logs de proxy y cabecera Referer). Si se firmara con JWT_SECRET a secas,
// alguien podría presentarlo como cookie access_token. La clave derivada lo
// impide en ambas direcciones.
func TestStateTokenIsNotUsableAsSession(t *testing.T) {
	svc := newTestGoogleService(t)

	state, err := svc.buildState(1, "/perfil")
	if err != nil {
		t.Fatalf("buildState: %v", err)
	}

	// Un state NO puede validarse con la clave de sesión.
	_, err = jwt.ParseWithClaims(state, &middleware.Claims{}, func(*jwt.Token) (interface{}, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err == nil {
		t.Fatal("un state token se validó como token de sesión: la clave no está separada")
	}

	// Y a la inversa: un token de sesión legítimo no vale como state.
	session, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{
		UserID:    99,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("no se pudo firmar el token de sesión: %v", err)
	}
	if _, err := svc.parseState(session); err != ErrInvalidState {
		t.Errorf("un token de sesión fue aceptado como state (err=%v)", err)
	}
}

func TestParseStateRejectsBadTokens(t *testing.T) {
	svc := newTestGoogleService(t)

	valid, err := svc.buildState(5, "/perfil")
	if err != nil {
		t.Fatalf("buildState: %v", err)
	}

	// Expirado.
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, googleStateClaims{
		UserID: 5,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"google-oauth-state"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}).SignedString(svc.stateSecret)
	if err != nil {
		t.Fatalf("firma: %v", err)
	}

	// Audiencia incorrecta: firmado con la clave buena pero para otro propósito.
	wrongAudience, err := jwt.NewWithClaims(jwt.SigningMethodHS256, googleStateClaims{
		UserID: 5,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"otra-cosa"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(svc.stateSecret)
	if err != nil {
		t.Fatalf("firma: %v", err)
	}

	// Sin user id: no identifica a nadie, no debe pasar.
	noUser, err := jwt.NewWithClaims(jwt.SigningMethodHS256, googleStateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"google-oauth-state"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(svc.stateSecret)
	if err != nil {
		t.Fatalf("firma: %v", err)
	}

	cases := map[string]string{
		"vacío":                "",
		"basura":               "no-es-un-jwt",
		"firma manipulada":     valid[:len(valid)-4] + "AAAA",
		"expirado":             expired,
		"audiencia incorrecta": wrongAudience,
		"sin user id":          noUser,
	}
	for name, token := range cases {
		if _, err := svc.parseState(token); err != ErrInvalidState {
			t.Errorf("%s: se esperaba ErrInvalidState, se obtuvo %v", name, err)
		}
	}
}

// El algoritmo 'none' es el ataque clásico de confusión de algoritmo: un token
// sin firma no debe aceptarse jamás.
func TestParseStateRejectsUnsignedToken(t *testing.T) {
	svc := newTestGoogleService(t)

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, googleStateClaims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"google-oauth-state"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("firma none: %v", err)
	}
	if _, err := svc.parseState(unsigned); err != ErrInvalidState {
		t.Errorf("se aceptó un token con alg=none (err=%v)", err)
	}
}

func TestParseIDToken(t *testing.T) {
	// id_token real de Google: header.payload.signature, todo base64url sin
	// padding. Aquí solo importa el payload (la firma no se verifica porque el
	// token llegó por TLS directo desde el endpoint de Google).
	build := func(payload string) string {
		enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
		return enc(`{"alg":"RS256"}`) + "." + enc(payload) + "." + enc("firma-no-verificada")
	}

	sub, email, err := parseIDToken(build(`{"sub":"1029384756","email":"hector@oberstaff.com","email_verified":true}`))
	if err != nil {
		t.Fatalf("parseIDToken: %v", err)
	}
	if sub != "1029384756" {
		t.Errorf("sub = %q", sub)
	}
	if email != "hector@oberstaff.com" {
		t.Errorf("email = %q", email)
	}

	bad := map[string]string{
		"vacío":        "",
		"no es jwt":    "abc",
		"sin email":    build(`{"sub":"123"}`),
		"sin sub":      build(`{"email":"a@b.com"}`),
		"payload roto": build(`{"sub":`),
	}
	for name, token := range bad {
		if _, _, err := parseIDToken(token); err == nil {
			t.Errorf("%s: parseIDToken debería fallar", name)
		}
	}
}

// La clave del state debe derivarse del JWT_SECRET y ser estable, pero distinta
// del secreto de sesión.
func TestDeriveStateSecret(t *testing.T) {
	a := deriveStateSecret(testJWTSecret)
	b := deriveStateSecret(testJWTSecret)
	if string(a) != string(b) {
		t.Error("la derivación no es determinista")
	}
	if string(a) == testJWTSecret {
		t.Error("la clave derivada coincide con el secreto de sesión")
	}
	if strings.Contains(string(a), testJWTSecret) {
		t.Error("la clave derivada contiene el secreto de sesión")
	}
	if c := deriveStateSecret(testJWTSecret + "x"); string(a) == string(c) {
		t.Error("secretos distintos derivan la misma clave")
	}
}
