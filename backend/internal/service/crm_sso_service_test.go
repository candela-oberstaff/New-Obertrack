package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

// ---- fakes ----

type fakeCrmUsers struct{ byID map[uint]*models.User }

func (f *fakeCrmUsers) GetByID(id uint) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("record not found")
}

func (f *fakeCrmUsers) GetByEmail(email string) (*models.User, error) {
	for _, u := range f.byID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("record not found")
}

type fakeCrmEmployments struct{ active map[[2]uint]bool }

func (f *fakeCrmEmployments) GetActive(userID, companyID uint) (*models.Employment, error) {
	if f.active[[2]uint{userID, companyID}] {
		return &models.Employment{UserID: userID, CompanyID: companyID}, nil
	}
	return nil, errors.New("record not found")
}

type fakeCrmNonces struct{ used map[string]bool }

func (f *fakeCrmNonces) Insert(n *models.CrmLoginNonce) error {
	if f.used[n.JTI] {
		return errors.New(`ERROR: duplicate key value violates unique constraint "crm_login_nonces_pkey" (SQLSTATE 23505)`)
	}
	f.used[n.JTI] = true
	return nil
}

func (f *fakeCrmNonces) DeleteExpired(time.Time) error { return nil }

type fakeCrmSessions struct {
	blocked  map[uint]string
	tenant   uint
	scope    string
	ttl      time.Duration
	issuedTo uint
}

func (f *fakeCrmSessions) AssertCanSignIn(u *models.User) error {
	if msg, ok := f.blocked[u.ID]; ok {
		return errors.New(msg)
	}
	return nil
}

func (f *fakeCrmSessions) IssueScopedAccess(u *models.User, tenantID uint, scope string, ttl time.Duration) (string, error) {
	f.issuedTo, f.tenant, f.scope, f.ttl = u.ID, tenantID, scope, ttl
	return "access-token", nil
}

// fakeCrmPerms: sin entrada en roles, el usuario no tiene roles asignados
// (hasRoles=false), que es el caso de la mayoría de cuentas hoy.
type fakeCrmPerms struct {
	roles   map[uint]map[string]string
	failFor map[uint]bool
}

func (f *fakeCrmPerms) EffectivePermissions(userID, _ uint) (map[string]string, bool, error) {
	if f.failFor[userID] {
		return nil, false, errors.New("db caída")
	}
	perms, ok := f.roles[userID]
	return perms, ok, nil
}

// ---- escenario ----

const (
	oberstaffID  uint = 10 // empresa Oberstaff (la que manda el CRM)
	otherCompany uint = 20
	sellerID     uint = 11 // vendedor con empleo activo en Oberstaff
	exSellerID   uint = 12 // sin empleo activo
	adminID      uint = 13 // superadmin con empleo en Oberstaff
	csID         uint = 14 // customer success con empleo en Oberstaff
	blockedID    uint = 15 // inducción pendiente
	personalID   uint = 16 // profesional en profesional (no empresa)
	itAnalystID  uint = 17 // analista de IT con empleo activo en Oberstaff
	tasksOnlyID  uint = 18 // rol con Tareas y sin Horas
	permsErrID   uint = 19 // leer sus permisos falla
	suspendedCo  uint = 30
)

type crmFixture struct {
	svc      *CrmSSOService
	sessions *fakeCrmSessions
	perms    *fakeCrmPerms
	keys     map[string]*rsa.PrivateKey
	now      time.Time
}

var testKeys = map[string]*rsa.PrivateKey{}

func testKey(t *testing.T, kid string) *rsa.PrivateKey {
	t.Helper()
	if k, ok := testKeys[kid]; ok {
		return k
	}
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	testKeys[kid] = k
	return k
}

func publicPEM(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func newCrmFixture(t *testing.T) *crmFixture {
	t.Helper()
	keys := map[string]*rsa.PrivateKey{
		"crm-prod-2026-09": testKey(t, "crm-prod-2026-09"),
		"crm-prod-2026-10": testKey(t, "crm-prod-2026-10"),
	}
	pems := map[string]string{}
	for kid, k := range keys {
		pems[kid] = publicPEM(t, k)
	}
	raw, _ := json.Marshal(pems)

	users := &fakeCrmUsers{byID: map[uint]*models.User{
		oberstaffID:  {ID: oberstaffID, Email: "admin@oberstaff.com", UserType: models.UserTypeEmployer, IsActive: true},
		otherCompany: {ID: otherCompany, Email: "cliente@otra.com", UserType: models.UserTypeEmployer, IsActive: true},
		suspendedCo:  {ID: suspendedCo, Email: "susp@x.com", UserType: models.UserTypeEmployer, IsActive: false},
		sellerID:     {ID: sellerID, Email: "vendedora@oberstaff.com", UserType: models.UserTypeProfessional, IsActive: true},
		exSellerID:   {ID: exSellerID, Email: "ex@oberstaff.com", UserType: models.UserTypeProfessional, IsActive: true},
		adminID:      {ID: adminID, Email: "root@oberstaff.com", UserType: models.UserTypeSuperadmin, IsSuperadmin: true, IsActive: true},
		csID:         {ID: csID, Email: "cs@oberstaff.com", UserType: models.UserTypeCustomerSuccess, IsActive: true},
		blockedID:    {ID: blockedID, Email: "nueva@oberstaff.com", UserType: models.UserTypeProfessional, IsActive: true},
		personalID:   {ID: personalID, Email: "persona@x.com", UserType: models.UserTypeProfessional, IsActive: true},
		itAnalystID:  {ID: itAnalystID, Email: "it@oberstaff.com", UserType: models.UserTypeITAnalyst, IsActive: true},
		tasksOnlyID:  {ID: tasksOnlyID, Email: "solo.tareas@oberstaff.com", UserType: models.UserTypeProfessional, IsActive: true},
		permsErrID:   {ID: permsErrID, Email: "roto@oberstaff.com", UserType: models.UserTypeProfessional, IsActive: true},
	}}
	emps := &fakeCrmEmployments{active: map[[2]uint]bool{
		{sellerID, oberstaffID}:    true,
		{adminID, oberstaffID}:     true,
		{csID, oberstaffID}:        true,
		{blockedID, oberstaffID}:   true,
		{itAnalystID, oberstaffID}: true,
		{tasksOnlyID, oberstaffID}: true,
		{permsErrID, oberstaffID}:  true,
	}}
	sessions := &fakeCrmSessions{blocked: map[uint]string{blockedID: "Aún no completas tu inducción."}}

	perms := &fakeCrmPerms{
		roles: map[uint]map[string]string{
			// Tareas en "edit", Horas explícitamente en "none".
			tasksOnlyID: {"tasks": models.PermissionEdit, "hours": models.PermissionNone},
		},
		failFor: map[uint]bool{permsErrID: true},
	}
	svc := NewCrmSSOService(string(raw), users, emps, &fakeCrmNonces{used: map[string]bool{}}, sessions, perms)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	return &crmFixture{svc: svc, sessions: sessions, perms: perms, keys: keys, now: now}
}

var jtiSeq int

// claims válidos por defecto; cada test cambia lo suyo.
func (f *crmFixture) claims(email string, company uint) jwt.MapClaims {
	jtiSeq++
	return jwt.MapClaims{
		"sub":                  email,
		"obertrack_company_id": company,
		"jti":                  fmt.Sprintf("6f1c2a3b-4d5e-4f60-8a7b-%012d", jtiSeq),
		"iss":                  "oberstaff-crm",
		"aud":                  "obertrack",
		"iat":                  f.now.Unix(),
		"exp":                  f.now.Add(60 * time.Second).Unix(),
	}
}

func (f *crmFixture) sign(t *testing.T, kid string, c jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	// Sin kid o con uno desconocido se firma con una clave válida: lo que se
	// prueba es que el kid, por sí solo, ya cierra el paso.
	key, ok := f.keys[kid]
	if !ok {
		key = f.keys["crm-prod-2026-09"]
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *CrmSSOError
	if !errors.As(err, &e) {
		t.Fatalf("esperaba el código %q, got %v", code, err)
	}
	if e.Code != code {
		t.Fatalf("código = %q (%s), esperaba %q", e.Code, e.Reason, code)
	}
}

// ---- tests ----

func TestCrmExchangeSellerOK(t *testing.T) {
	f := newCrmFixture(t)
	res, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims("vendedora@oberstaff.com", oberstaffID)))
	if err != nil {
		t.Fatalf("canje válido rechazado: %v", err)
	}
	if res.AccessToken == "" || res.CompanyID != oberstaffID || res.Kid != "crm-prod-2026-09" {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if f.sessions.tenant != oberstaffID || f.sessions.scope != middleware.ScopeTasks || f.sessions.ttl != CrmSessionTTL {
		t.Fatalf("sesión emitida con tenant=%d scope=%q ttl=%s", f.sessions.tenant, f.sessions.scope, f.sessions.ttl)
	}
}

// Rotación (§3): mientras convivan, las dos claves sirven.
func TestCrmExchangeSecondKid(t *testing.T) {
	f := newCrmFixture(t)
	if _, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-10", f.claims("vendedora@oberstaff.com", oberstaffID))); err != nil {
		t.Fatalf("la segunda clave debe servir: %v", err)
	}
}

// Contrato v1.1, §8: los analistas de IT con empleo activo en Oberstaff entran.
// Solo superadmin y Customer Success se rechazan siempre.
func TestCrmExchangeITAnalystAllowed(t *testing.T) {
	f := newCrmFixture(t)
	if _, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims("it@oberstaff.com", oberstaffID))); err != nil {
		t.Fatalf("un analista de IT con empleo activo debe entrar: %v", err)
	}
}

func TestCrmExchangeCompanyAccountOK(t *testing.T) {
	f := newCrmFixture(t)
	if _, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims("admin@oberstaff.com", oberstaffID))); err != nil {
		t.Fatalf("la cuenta de la propia empresa debe entrar: %v", err)
	}
}

func TestCrmExchangeEmailCase(t *testing.T) {
	f := newCrmFixture(t)
	if _, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims("Vendedora@Oberstaff.com", oberstaffID))); err != nil {
		t.Fatalf("el correo con otras mayúsculas debe resolverse: %v", err)
	}
}

func TestCrmExchangeInvalidTokens(t *testing.T) {
	f := newCrmFixture(t)
	other := testKey(t, "ajena")

	cases := []struct {
		name  string
		token func() string
	}{
		{"sin kid", func() string { return f.sign(t, "", f.claims("vendedora@oberstaff.com", oberstaffID)) }},
		{"kid desconocido", func() string { return f.sign(t, "crm-prod-2025-01", f.claims("vendedora@oberstaff.com", oberstaffID)) }},
		{"firmado con la clave de otro kid", func() string {
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, f.claims("vendedora@oberstaff.com", oberstaffID))
			tok.Header["kid"] = "crm-prod-2026-09"
			s, _ := tok.SignedString(f.keys["crm-prod-2026-10"])
			return s
		}},
		{"firmado con una clave ajena", func() string {
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, f.claims("vendedora@oberstaff.com", oberstaffID))
			tok.Header["kid"] = "crm-prod-2026-09"
			s, _ := tok.SignedString(other)
			return s
		}},
		{"HS256 con la clave pública como secreto", func() string {
			tok := jwt.NewWithClaims(jwt.SigningMethodHS256, f.claims("vendedora@oberstaff.com", oberstaffID))
			tok.Header["kid"] = "crm-prod-2026-09"
			s, _ := tok.SignedString([]byte(publicPEM(t, f.keys["crm-prod-2026-09"])))
			return s
		}},
		{"alg none", func() string {
			tok := jwt.NewWithClaims(jwt.SigningMethodNone, f.claims("vendedora@oberstaff.com", oberstaffID))
			tok.Header["kid"] = "crm-prod-2026-09"
			s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
			return s
		}},
		{"iss equivocado", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			c["iss"] = "otro"
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"aud equivocado", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			c["aud"] = "obersuite"
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"exp - iat = 61", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			c["exp"] = f.now.Add(61 * time.Second).Unix()
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"sin exp", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			delete(c, "exp")
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"sin iat", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			delete(c, "iat")
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"sin sub", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			delete(c, "sub")
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"jti que no es UUID v4", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			c["jti"] = "abc-123"
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"sin obertrack_company_id", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			delete(c, "obertrack_company_id")
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"obertrack_company_id como texto", func() string {
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			c["obertrack_company_id"] = "10"
			return f.sign(t, "crm-prod-2026-09", c)
		}},
		{"basura", func() string { return "no.es.un-jwt" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.Exchange(tc.token())
			wantCode(t, err, CrmCodeInvalidToken)
		})
	}
}

// Leeway de 30 s (§2): un exp vencido hace 20 s entra; uno de hace 40 s no.
func TestCrmExchangeLeeway(t *testing.T) {
	f := newCrmFixture(t)
	tok := f.sign(t, "crm-prod-2026-09", f.claims("vendedora@oberstaff.com", oberstaffID))

	f.svc.now = func() time.Time { return f.now.Add(80 * time.Second) } // exp + 20 s
	if _, err := f.svc.Exchange(tok); err != nil {
		t.Fatalf("exp vencido hace 20 s debe entrar con el leeway: %v", err)
	}

	tok = f.sign(t, "crm-prod-2026-09", f.claims("vendedora@oberstaff.com", oberstaffID))
	f.svc.now = func() time.Time { return f.now.Add(100 * time.Second) } // exp + 40 s
	_, err := f.svc.Exchange(tok)
	wantCode(t, err, CrmCodeInvalidToken)

	// Un iat en el futuro también tolera 30 s de desfase de reloj.
	tok = f.sign(t, "crm-prod-2026-09", f.claims("vendedora@oberstaff.com", oberstaffID))
	f.svc.now = func() time.Time { return f.now.Add(-20 * time.Second) }
	if _, err := f.svc.Exchange(tok); err != nil {
		t.Fatalf("iat 20 s en el futuro debe entrar con el leeway: %v", err)
	}
}

func TestCrmExchangeTokenUsed(t *testing.T) {
	f := newCrmFixture(t)
	tok := f.sign(t, "crm-prod-2026-09", f.claims("vendedora@oberstaff.com", oberstaffID))
	if _, err := f.svc.Exchange(tok); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Exchange(tok)
	wantCode(t, err, CrmCodeTokenUsed)
}

// El jti se consume aunque el canje termine en user_not_found: el mismo token
// no se puede volver a intentar.
func TestCrmExchangeConsumesJTIBeforeUserCheck(t *testing.T) {
	f := newCrmFixture(t)
	tok := f.sign(t, "crm-prod-2026-09", f.claims("nadie@oberstaff.com", oberstaffID))
	_, err := f.svc.Exchange(tok)
	wantCode(t, err, CrmCodeUserNotFound)
	_, err = f.svc.Exchange(tok)
	wantCode(t, err, CrmCodeTokenUsed)
}

func TestCrmExchangeCompanyNotFound(t *testing.T) {
	f := newCrmFixture(t)
	for name, company := range map[string]uint{
		"no existe":          999,
		"es un profesional":  sellerID,
		"empresa suspendida": suspendedCo,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims("vendedora@oberstaff.com", company)))
			wantCode(t, err, CrmCodeCompanyNotFound)
		})
	}
}

// §8: la pertenencia es la barrera real, y superadmin y CS se rechazan
// siempre aunque tengan empleo en la empresa.
func TestCrmExchangeUserNotFound(t *testing.T) {
	f := newCrmFixture(t)
	cases := map[string]struct {
		email   string
		company uint
	}{
		"correo inexistente":            {"nadie@oberstaff.com", oberstaffID},
		"sin empleo activo":             {"ex@oberstaff.com", oberstaffID},
		"vendedora contra otra empresa": {"vendedora@oberstaff.com", otherCompany},
		"cuenta de otra empresa":        {"cliente@otra.com", oberstaffID},
		"profesional de otra parte":     {"persona@x.com", oberstaffID},
		"superadmin con empleo":         {"root@oberstaff.com", oberstaffID},
		"customer success con empleo":   {"cs@oberstaff.com", oberstaffID},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims(tc.email, tc.company)))
			wantCode(t, err, CrmCodeUserNotFound)
		})
	}
}

func TestCrmExchangeAccessSuspended(t *testing.T) {
	f := newCrmFixture(t)
	_, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", f.claims("nueva@oberstaff.com", oberstaffID)))
	wantCode(t, err, CrmCodeAccessSuspended)
}

func TestParseCrmPublicKeysFailClosed(t *testing.T) {
	good := publicPEM(t, testKey(t, "crm-prod-2026-09"))
	small, _ := rsa.GenerateKey(rand.Reader, 1024)
	cases := map[string]string{
		"vacía":           "",
		"no es JSON":      "-----BEGIN PUBLIC KEY-----",
		"objeto vacío":    "{}",
		"un PEM inválido": `{"a":` + mustJSON(good) + `,"b":"basura"}`,
		"clave de 1024":   `{"a":` + mustJSON(publicPEM(t, small)) + `}`,
		"kid vacío":       `{"":` + mustJSON(good) + `}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCrmPublicKeys(raw); err == nil {
				t.Fatal("debió fallar")
			}
			svc := NewCrmSSOService(raw, nil, nil, nil, nil, nil)
			if svc.Enabled() {
				t.Fatal("con claves inválidas el canje debe quedar deshabilitado")
			}
		})
	}
}

// Claim scope (contrato v1.3): elige la vista. Sin él, "tasks", como los
// tokens anteriores; cualquier valor fuera de la lista es invalid_token.
func TestCrmExchangeScope(t *testing.T) {
	cases := []struct {
		name  string
		scope any // nil = sin claim
		want  string
	}{
		{"sin claim → tasks", nil, middleware.ScopeTasks},
		{"tasks", "tasks", middleware.ScopeTasks},
		{"hours", "hours", middleware.ScopeHours},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCrmFixture(t)
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			if tc.scope != nil {
				c["scope"] = tc.scope
			}
			res, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", c))
			if err != nil {
				t.Fatalf("canje rechazado: %v", err)
			}
			if res.Scope != tc.want || f.sessions.scope != tc.want {
				t.Fatalf("scope = %q (sesión %q), esperaba %q", res.Scope, f.sessions.scope, tc.want)
			}
		})
	}

	for name, bad := range map[string]any{
		"desconocido": "admin",
		"vacío":       "",
		"mayúsculas":  "HOURS",
		"no es texto": 7,
		"lista":       []string{"tasks", "hours"},
	} {
		t.Run("inválido: "+name, func(t *testing.T) {
			f := newCrmFixture(t)
			c := f.claims("vendedora@oberstaff.com", oberstaffID)
			c["scope"] = bad
			_, err := f.svc.Exchange(f.sign(t, "crm-prod-2026-09", c))
			wantCode(t, err, CrmCodeInvalidToken)
		})
	}
}

// v1.3, §7 scope_not_allowed: el rol tiene que dar acceso al módulo de la
// vista, con las reglas de RequirePermission.
func TestCrmExchangeScopeNotAllowed(t *testing.T) {
	sign := func(f *crmFixture, email string, scope string) string {
		c := f.claims(email, oberstaffID)
		c["scope"] = scope
		return f.sign(t, "crm-prod-2026-09", c)
	}

	f := newCrmFixture(t)
	_, err := f.svc.Exchange(sign(f, "solo.tareas@oberstaff.com", "hours"))
	wantCode(t, err, CrmCodeScopeNotAllowed)

	if _, err := f.svc.Exchange(sign(f, "solo.tareas@oberstaff.com", "tasks")); err != nil {
		t.Fatalf("con Tareas en edit, la vista de Tareas debe abrir: %v", err)
	}
	if _, err := f.svc.Exchange(sign(f, "vendedora@oberstaff.com", "hours")); err != nil {
		t.Fatalf("sin roles asignados no se restringe, como en la app: %v", err)
	}

	// Un rol sin la clave del módulo tampoco da acceso.
	f.perms.roles[sellerID] = map[string]string{"tasks": models.PermissionView}
	_, err = f.svc.Exchange(sign(f, "vendedora@oberstaff.com", "hours"))
	wantCode(t, err, CrmCodeScopeNotAllowed)

	// La cuenta de empresa no pasa por el RBAC, igual que en RequirePermission.
	f.perms.roles[oberstaffID] = map[string]string{"hours": models.PermissionNone}
	if _, err := f.svc.Exchange(sign(f, "admin@oberstaff.com", "hours")); err != nil {
		t.Fatalf("la cuenta de empresa debe abrir Horas: %v", err)
	}

	// Si no se pueden leer los permisos, no se abre (fallo interno, no un código).
	_, err = f.svc.Exchange(sign(f, "roto@oberstaff.com", "hours"))
	var ssoErr *CrmSSOError
	if err == nil || errors.As(err, &ssoErr) {
		t.Fatalf("un fallo al leer permisos debe ser un error interno, got %v", err)
	}
}
