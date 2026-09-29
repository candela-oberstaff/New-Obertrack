package service

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

// Acceso embebido a Tareas desde el CRM (Obersuite). El contrato vive en
// docs/integracion-crm-tareas.md; los números de sección de los comentarios
// son los suyos.

// Códigos de error del canje (§7). Son contrato: el CRM decide qué mostrar y si
// reintenta según el código, así que no se renombran sin subir la versión.
const (
	CrmCodeInvalidToken    = "invalid_token"
	CrmCodeTokenUsed       = "token_used"
	CrmCodeCompanyNotFound = "company_not_found"
	CrmCodeUserNotFound    = "user_not_found"
	CrmCodeAccessSuspended = "access_suspended"
)

const (
	crmIssuer   = "oberstaff-crm"
	crmAudience = "obertrack"
	// crmLeeway se aplica a exp, iat y nbf (§2).
	crmLeeway = 30 * time.Second
	// crmMaxLifetime: el CRM emite exp = iat + 60 y todo lo que dure más se
	// rechaza, aunque la firma sea buena (§2).
	crmMaxLifetime = 60 * time.Second
	// CrmSessionTTL es la duración de la sesión embebida, sin refresh (§4).
	CrmSessionTTL = 60 * time.Minute
)

// uuidV4 valida el formato del jti (§2): versión 4 y variante RFC 4122.
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// CrmSSOError es un canje rechazado. Code es lo que ve el CRM; Reason, el
// motivo real, que solo va a la auditoría (§7: user_not_found no distingue
// entre "no existe" y "no pertenece", pero la auditoría sí).
type CrmSSOError struct {
	Code      string
	Reason    string
	Kid       string
	JTI       string
	Email     string
	CompanyID uint
}

func (e *CrmSSOError) Error() string { return e.Code + ": " + e.Reason }

// CrmSSOResult es un canje aceptado.
type CrmSSOResult struct {
	AccessToken string
	User        *models.User
	CompanyID   uint
	Kid         string
	JTI         string
}

type crmUserLookup interface {
	GetByID(id uint) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
}

type crmEmploymentLookup interface {
	GetActive(userID, companyID uint) (*models.Employment, error)
}

type crmNonceStore interface {
	Insert(n *models.CrmLoginNonce) error
	DeleteExpired(before time.Time) error
}

type crmSessionIssuer interface {
	AssertCanSignIn(user *models.User) error
	IssueScopedAccess(user *models.User, tenantID uint, scope string, ttl time.Duration) (string, error)
}

type crmClaims struct {
	// Puntero para distinguir "ausente" de 0. Un valor no entero (12.5, "12")
	// hace fallar el parseo y cae en invalid_token.
	ObertrackCompanyID *int64 `json:"obertrack_company_id"`
	jwt.RegisteredClaims
}

// CrmSSOService canjea el token de un solo uso del CRM por una sesión de
// Obertrack acotada a Tareas.
type CrmSSOService struct {
	keys        map[string]*rsa.PublicKey
	users       crmUserLookup
	employments crmEmploymentLookup
	nonces      crmNonceStore
	sessions    crmSessionIssuer
	now         func() time.Time
}

// NewCrmSSOService lee las claves públicas de CRM_SSO_PUBLIC_KEYS (§3). Si
// faltan o alguna no sirve, el servicio queda deshabilitado (fail-closed) y el
// canje responde 503; el resto de Obertrack arranca igual.
func NewCrmSSOService(rawKeys string, users crmUserLookup, employments crmEmploymentLookup, nonces crmNonceStore, sessions crmSessionIssuer) *CrmSSOService {
	s := &CrmSSOService{users: users, employments: employments, nonces: nonces, sessions: sessions, now: time.Now}
	keys, err := ParseCrmPublicKeys(rawKeys)
	if err != nil {
		log.Printf("[CRM SSO] deshabilitado: %v", err)
		return s
	}
	s.keys = keys
	kids := make([]string, 0, len(keys))
	for kid := range keys {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	log.Printf("[CRM SSO] habilitado con kid: %s", strings.Join(kids, ", "))
	return s
}

// ParseCrmPublicKeys interpreta {"<kid>": "<PEM>", ...}. Es todo o nada: un
// solo PEM inválido deshabilita el canje, en vez de arrancar con un juego de
// claves que no es el que se configuró.
func ParseCrmPublicKeys(raw string) (map[string]*rsa.PublicKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("CRM_SSO_PUBLIC_KEYS no está configurada")
	}
	var pems map[string]string
	if err := json.Unmarshal([]byte(raw), &pems); err != nil {
		return nil, fmt.Errorf("CRM_SSO_PUBLIC_KEYS no es un JSON {kid: PEM}: %w", err)
	}
	if len(pems) == 0 {
		return nil, errors.New("CRM_SSO_PUBLIC_KEYS no tiene claves")
	}
	keys := make(map[string]*rsa.PublicKey, len(pems))
	for kid, pem := range pems {
		if strings.TrimSpace(kid) == "" {
			return nil, errors.New("CRM_SSO_PUBLIC_KEYS tiene un kid vacío")
		}
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pem))
		if err != nil {
			return nil, fmt.Errorf("la clave %q no es un PEM RSA válido: %w", kid, err)
		}
		if key.N.BitLen() < 2048 {
			return nil, fmt.Errorf("la clave %q tiene menos de 2048 bits", kid)
		}
		keys[kid] = key
	}
	return keys, nil
}

// Enabled dice si hay claves cargadas.
func (s *CrmSSOService) Enabled() bool { return len(s.keys) > 0 }

// Exchange valida el token y emite la sesión. Devuelve *CrmSSOError si el
// canje se rechaza por el token o por el usuario; cualquier otro error es un
// fallo interno (base de datos, firma de la sesión).
func (s *CrmSSOService) Exchange(raw string) (*CrmSSOResult, error) {
	if !s.Enabled() {
		return nil, errors.New("canje del CRM deshabilitado")
	}

	var kid string
	claims := &crmClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(crmIssuer),
		jwt.WithAudience(crmAudience),
		jwt.WithLeeway(crmLeeway),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(s.now),
	)
	_, err := parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		k, _ := t.Header["kid"].(string)
		kid = k
		if k == "" {
			return nil, errors.New("falta kid en la cabecera")
		}
		key, ok := s.keys[k]
		if !ok {
			return nil, fmt.Errorf("kid desconocido %q", k)
		}
		return key, nil
	})
	fail := func(code, reason string) *CrmSSOError {
		e := &CrmSSOError{Code: code, Reason: reason, Kid: kid, JTI: claims.ID, Email: claims.Subject}
		if claims.ObertrackCompanyID != nil && *claims.ObertrackCompanyID > 0 {
			e.CompanyID = uint(*claims.ObertrackCompanyID)
		}
		return e
	}
	if err != nil {
		return nil, fail(CrmCodeInvalidToken, err.Error())
	}

	// Claims obligatorios (§2) que el parser no exige por sí solo.
	switch {
	case claims.IssuedAt == nil:
		return nil, fail(CrmCodeInvalidToken, "falta iat")
	case claims.ExpiresAt.Sub(claims.IssuedAt.Time) > crmMaxLifetime:
		return nil, fail(CrmCodeInvalidToken, fmt.Sprintf("exp - iat = %s, supera %s", claims.ExpiresAt.Sub(claims.IssuedAt.Time), crmMaxLifetime))
	case strings.TrimSpace(claims.Subject) == "":
		return nil, fail(CrmCodeInvalidToken, "falta sub")
	case !uuidV4.MatchString(claims.ID):
		return nil, fail(CrmCodeInvalidToken, "jti ausente o no es un UUID v4")
	case claims.ObertrackCompanyID == nil || *claims.ObertrackCompanyID <= 0 || *claims.ObertrackCompanyID > int64(^uint32(0)):
		return nil, fail(CrmCodeInvalidToken, "obertrack_company_id ausente o fuera de rango")
	}
	companyID := uint(*claims.ObertrackCompanyID)
	email := strings.TrimSpace(claims.Subject)

	// El jti se consume ANTES de mirar empresa y usuario: un token válido que
	// termina en user_not_found tampoco se puede volver a intentar.
	now := s.now()
	nonce := &models.CrmLoginNonce{
		JTI:       claims.ID,
		Kid:       kid,
		Email:     email,
		CompanyID: companyID,
		UsedAt:    now,
		ExpiresAt: claims.ExpiresAt.Add(crmLeeway),
	}
	if err := s.nonces.Insert(nonce); err != nil {
		if isUniqueViolation(err) {
			return nil, fail(CrmCodeTokenUsed, "jti ya consumido")
		}
		return nil, fmt.Errorf("no se pudo registrar el jti: %w", err)
	}
	// Limpieza oportunista: el volumen es de unos pocos canjes al día, así
	// que no hace falta un worker. Un fallo aquí no afecta al canje.
	if err := s.nonces.DeleteExpired(now); err != nil {
		log.Printf("[CRM SSO] no se pudieron borrar los jti vencidos: %v", err)
	}

	// Empresa (§7 company_not_found): existe, es una empresa y está activa.
	company, err := s.users.GetByID(companyID)
	if err != nil || company == nil {
		return nil, fail(CrmCodeCompanyNotFound, "la empresa no existe")
	}
	if company.UserType != models.UserTypeEmployer {
		return nil, fail(CrmCodeCompanyNotFound, fmt.Sprintf("el id es de un usuario %s, no de una empresa", company.UserType))
	}
	if !company.IsActive {
		return nil, fail(CrmCodeCompanyNotFound, "la empresa está suspendida")
	}

	// Usuario y pertenencia (§8). Es la barrera real: un enlace mal hecho en
	// el CRM no debe dar acceso a otra empresa.
	user, err := s.findUser(email)
	if err != nil {
		return nil, fail(CrmCodeUserNotFound, "el correo no existe en Obertrack")
	}
	if user.IsSuperadmin || user.UserType == models.UserTypeSuperadmin || user.UserType == models.UserTypeCustomerSuccess {
		return nil, fail(CrmCodeUserNotFound, fmt.Sprintf("cuenta de plataforma (%s): rechazada siempre", user.UserType))
	}
	if user.UserType == models.UserTypeEmployer {
		if user.ID != companyID {
			return nil, fail(CrmCodeUserNotFound, "es la cuenta de otra empresa")
		}
	} else if _, err := s.employments.GetActive(user.ID, companyID); err != nil {
		return nil, fail(CrmCodeUserNotFound, "sin empleo activo en la empresa")
	}

	if err := s.sessions.AssertCanSignIn(user); err != nil {
		return nil, fail(CrmCodeAccessSuspended, err.Error())
	}

	access, err := s.sessions.IssueScopedAccess(user, companyID, middleware.ScopeTasks, CrmSessionTTL)
	if err != nil {
		return nil, fmt.Errorf("no se pudo emitir la sesión: %w", err)
	}
	return &CrmSSOResult{AccessToken: access, User: user, CompanyID: companyID, Kid: kid, JTI: claims.ID}, nil
}

// findUser busca por el correo tal cual llega y, si no aparece, en minúsculas:
// el CRM lee el correo de su propia base y no tiene por qué conservar las
// mayúsculas con que se dio de alta aquí.
func (s *CrmSSOService) findUser(email string) (*models.User, error) {
	user, err := s.users.GetByEmail(email)
	if err == nil && user != nil {
		return user, nil
	}
	if lower := strings.ToLower(email); lower != email {
		if user, err := s.users.GetByEmail(lower); err == nil && user != nil {
			return user, nil
		}
	}
	return nil, errors.New("no encontrado")
}
