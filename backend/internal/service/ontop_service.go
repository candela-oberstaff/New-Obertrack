package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/obertrack/backend/internal/config"
)

// OntopService es el cliente HTTP de bajo nivel contra la API de Ontop. Encapsula
// la autenticación (login + caché del JWT con expiración) y expone métodos
// tipados por cada endpoint que consume el módulo Wallet.
//
// Es un CLIENTE de integración (como WahaService/BrevoService), no un servicio de
// dominio: no toca la base de datos ni conoce el modelo local. Toda la lógica de
// negocio/reconciliación vive en WalletService, que lo envuelve.
type OntopService struct {
	baseURL  string
	email    string
	password string
	clientID string
	client   *http.Client

	// La renovación del token se serializa para evitar múltiples logins en
	// paralelo bajo carga concurrente.
	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

// tokenSkew renueva el token un poco antes de su expiración real para no usar
// nunca uno a punto de caducar en pleno request.
const tokenSkew = 60 * time.Second

func NewOntopService(cfg *config.Config) *OntopService {
	return &OntopService{
		baseURL:  strings.TrimRight(cfg.OntopAPIURL, "/"),
		email:    cfg.OntopEmail,
		password: cfg.OntopPassword,
		clientID: cfg.OntopClientID,
		// Pagos: timeout holgado pero acotado para no colgar el handler si Ontop
		// tarda o no responde.
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Configured indica si hay credenciales suficientes para operar contra Ontop.
func (s *OntopService) Configured() bool {
	return s.email != "" && s.password != "" && s.clientID != ""
}

func (s *OntopService) ClientID() string { return s.clientID }

// ---------------------------------------------------------------------------
// Autenticación
// ---------------------------------------------------------------------------

type ontopLoginRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	RememberMe     bool   `json:"rememberMe"`
	RecaptchaToken string `json:"recaptchaToken"`
	User           string `json:"user"`
	IP             string `json:"ip"`
	WebDeviceInfo  string `json:"webDeviceInfo"`
}

// login pide un token nuevo. No usa el cliente autenticado (aún no hay token).
func (s *OntopService) login() (string, time.Time, error) {
	if s.email == "" || s.password == "" {
		return "", time.Time{}, errors.New("Ontop no está configurado (faltan credenciales)")
	}
	body, _ := json.Marshal(ontopLoginRequest{
		Email:         s.email,
		Password:      s.password,
		RememberMe:    false,
		User:          s.email,
		WebDeviceInfo: "Backend",
	})

	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/login/login/v2", bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("Ontop login: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("Ontop login falló (HTTP %d): %s", resp.StatusCode, truncate(string(raw), 300))
	}

	token := extractToken(raw)
	if token == "" {
		return "", time.Time{}, errors.New("Ontop login: no se encontró token en la respuesta")
	}
	exp := tokenExpiry(token)
	return token, exp, nil
}

// extractToken localiza el JWT en la respuesta de login, tolerando las formas
// más comunes (token/access_token/jwt en la raíz o bajo "data").
func extractToken(raw []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	candidates := []string{"token", "access_token", "accessToken", "jwt", "id_token", "idToken"}
	if t := findStringKey(m, candidates); t != "" {
		return t
	}
	// Buscar un nivel anidado (p.ej. {"data": {"token": "..."}}).
	for _, nestKey := range []string{"data", "result", "payload"} {
		if nested, ok := m[nestKey].(map[string]interface{}); ok {
			if t := findStringKey(nested, candidates); t != "" {
				return t
			}
		}
	}
	return ""
}

func findStringKey(m map[string]interface{}, keys []string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// tokenExpiry lee (sin verificar la firma) el claim "exp" del JWT para saber
// cuándo renovarlo. Si no se puede parsear, asume una vida corta conservadora.
func tokenExpiry(token string) time.Time {
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err == nil {
		if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
			if exp, err := claims.GetExpirationTime(); err == nil && exp != nil {
				return exp.Time
			}
		}
	}
	// Fallback: 15 minutos sin poder inferir exp real.
	return time.Now().Add(15 * time.Minute)
}

// ensureToken devuelve un token válido, renovándolo si expiró o está por expirar.
func (s *OntopService) ensureToken() (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if s.token != "" && time.Now().Before(s.tokenExp.Add(-tokenSkew)) {
		return s.token, nil
	}
	token, exp, err := s.login()
	if err != nil {
		return "", err
	}
	s.token = token
	s.tokenExp = exp
	return token, nil
}

// ---------------------------------------------------------------------------
// Helper de requests autenticados
// ---------------------------------------------------------------------------

// doJSON ejecuta una petición autenticada y deserializa la respuesta JSON en out
// (si out != nil). Reintenta una vez re-autenticando si Ontop responde 401.
func (s *OntopService) doJSON(method, path string, payload interface{}, out interface{}) error {
	token, err := s.ensureToken()
	if err != nil {
		return err
	}
	resp, raw, err := s.rawRequest(method, path, payload, token)
	if err != nil {
		return err
	}
	// Token rechazado: fuerza un re-login y reintenta una vez.
	if resp.StatusCode == http.StatusUnauthorized {
		s.invalidateToken()
		token, err = s.ensureToken()
		if err != nil {
			return err
		}
		resp, raw, err = s.rawRequest(method, path, payload, token)
		if err != nil {
			return err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Ontop %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(string(raw), 300))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("Ontop %s %s: respuesta ilegible: %w", method, path, err)
		}
	}
	return nil
}

func (s *OntopService) rawRequest(method, path string, payload interface{}, token string) (*http.Response, []byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Ontop %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, raw, nil
}

func (s *OntopService) invalidateToken() {
	s.tokenMu.Lock()
	s.token = ""
	s.tokenExp = time.Time{}
	s.tokenMu.Unlock()
}

// ---------------------------------------------------------------------------
// Endpoints tipados
// ---------------------------------------------------------------------------

// OntopWallet es la vista de la billetera del cliente (saldo disponible).
type OntopWallet struct {
	ID       int64   `json:"id"`
	ClientID int64   `json:"client_id"`
	Enabled  bool    `json:"enabled"`
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

func (s *OntopService) GetWallet() (*OntopWallet, error) {
	var w OntopWallet
	path := fmt.Sprintf("/client-wallet/clients/%s/wallet", url.PathEscape(s.clientID))
	if err := s.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// OntopWorker es un trabajador vinculado al cliente.
type OntopWorker struct {
	AgreementID int64  `json:"agreementId"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email"`
	Status      string `json:"status"`
}

type ontopWorkersListResponse struct {
	Content       []OntopWorker `json:"content"`
	TotalElements int64         `json:"totalElements"`
}

// ListWorkers lista trabajadores. statusId < 0 omite el filtro por estado.
func (s *OntopService) ListWorkers(statusID int) ([]OntopWorker, int64, error) {
	path := "/contract/workers/list"
	if statusID >= 0 {
		path += "?statusId=" + fmt.Sprint(statusID)
	}
	var resp ontopWorkersListResponse
	if err := s.doJSON(http.MethodGet, path, nil, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Content, resp.TotalElements, nil
}

// GetWorker obtiene un trabajador por email.
func (s *OntopService) GetWorker(email string) (map[string]interface{}, error) {
	path := "/contract/workers?email=" + url.QueryEscape(email)
	var out map[string]interface{}
	if err := s.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OntopPaymentInput es un pago dentro del payload de creación de paylist.
type OntopPaymentInput struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	WorkerEmail string  `json:"worker_email"`
}

type ontopCreatePaylistRequest struct {
	Description    string              `json:"description"`
	IdempotenceKey string              `json:"idempotence_key"`
	Payments       []OntopPaymentInput `json:"payments"`
	ClientID       interface{}         `json:"client_id"`
}

// OntopPaylistResponse es la respuesta de crear/consultar una paylist.
type OntopPaylistResponse struct {
	ID             int64  `json:"id"`
	Description    string `json:"description"`
	ClientID       int64  `json:"client_id"`
	IdempotenceKey string `json:"idempotence_key"`
}

// CreatePaylist crea un lote de pagos en Ontop. clientIDNumeric se pasa tal cual
// espera Ontop (numérico); idempotenceKey debe ser único de 32 chars.
func (s *OntopService) CreatePaylist(description, idempotenceKey string, payments []OntopPaymentInput, clientIDNumeric interface{}) (*OntopPaylistResponse, error) {
	reqBody := ontopCreatePaylistRequest{
		Description:    description,
		IdempotenceKey: idempotenceKey,
		Payments:       payments,
		ClientID:       clientIDNumeric,
	}
	var out OntopPaylistResponse
	if err := s.doJSON(http.MethodPost, "/payment-agent/paylists", reqBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OntopPaymentDetail es un pago individual tal como lo devuelve Ontop.
// NOTA: el nombre exacto del campo de fecha de Ontop no está documentado en la
// guía; se intentan las variantes más comunes. Ajustar cuando se confirme.
type OntopPaymentDetail struct {
	ID          int64   `json:"id"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	WorkerEmail string  `json:"worker_email"`
	PaylistID   int64   `json:"paylist_id"`
	Status      string  `json:"status"`
	Cause       *string `json:"cause"`
	CreatedAt   string  `json:"created_at"`
	Date        string  `json:"date"`
}

// PaymentDate devuelve la primera fecha disponible del pago (ISO), o "".
func (p OntopPaymentDetail) PaymentDate() string {
	if p.CreatedAt != "" {
		return p.CreatedAt
	}
	return p.Date
}

type ontopPaymentsListResponse struct {
	Content []OntopPaymentDetail `json:"content"`
}

// GetPaylistPayments lista los pagos de una paylist en Ontop.
func (s *OntopService) GetPaylistPayments(ontopPaylistID int64) ([]OntopPaymentDetail, error) {
	path := fmt.Sprintf("/payment-agent/paylists/%d/payments?page=1&size=200", ontopPaylistID)
	var resp ontopPaymentsListResponse
	if err := s.doJSON(http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Content, nil
}

// GetPayment obtiene un pago individual por ID.
func (s *OntopService) GetPayment(ontopPaymentID int64) (*OntopPaymentDetail, error) {
	var out OntopPaymentDetail
	path := fmt.Sprintf("/payment-agent/payments/%d", ontopPaymentID)
	if err := s.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type ontopPaymentsPage struct {
	Content    []OntopPaymentDetail `json:"content"`
	Last       *bool                `json:"last"`
	TotalPages *int                 `json:"totalPages"`
}

// ListPayments lista pagos individuales del cliente en un rango de fechas
// (paginado). Devuelve además si es la última página. Se usa para armar la vista
// personal filtrando por el email del trabajador en el backend (el profesional
// nunca ve pagos de otros).
func (s *OntopService) ListPayments(startISO, endISO string, page, size int) ([]OntopPaymentDetail, bool, error) {
	if size <= 0 {
		size = 200
	}
	path := fmt.Sprintf("/payment-agent/payments?startDate=%s&endDate=%s&page=%d&size=%d",
		url.QueryEscape(startISO), url.QueryEscape(endISO), page, size)
	var resp ontopPaymentsPage
	if err := s.doJSON(http.MethodGet, path, nil, &resp); err != nil {
		return nil, false, err
	}
	last := len(resp.Content) < size
	if resp.Last != nil {
		last = *resp.Last
	}
	return resp.Content, last, nil
}

// GetTransactions expone el libro contable del cliente (paginado).
func (s *OntopService) GetTransactions(page, size int) (map[string]interface{}, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 50
	}
	path := fmt.Sprintf("/client-wallet/clients/%s/transactions?page=%d&size=%d", url.PathEscape(s.clientID), page, size)
	var out map[string]interface{}
	if err := s.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// truncate acota mensajes de error de Ontop para no volcar respuestas enormes al log.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
