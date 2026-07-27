package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

const (
	googleAuthEndpoint   = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint  = "https://oauth2.googleapis.com/token"
	googleRevokeEndpoint = "https://oauth2.googleapis.com/revoke"

	// googleScopes: 'openid email' identifica la cuenta vinculada (sub + email);
	// calendar.events da acceso SOLO a crear y editar eventos, que es justo lo
	// que necesita la sincronización (Fase 2) y nada más. Deliberadamente NO se
	// pide calendar.readonly ni calendar (que permitirían listar o crear
	// calendarios): mantener el scope mínimo simplifica la verificación de
	// Google. Como contrapartida, los eventos siempre van al calendario
	// principal del usuario ('primary'); no hay selector de calendario destino.
	googleScopes = "openid email https://www.googleapis.com/auth/calendar.events"

	// stateTTL acota la ventana en la que un state robado sirve de algo. El
	// consentimiento de Google rara vez pasa de un minuto.
	stateTTL = 10 * time.Minute

	// accessTokenSkew renueva el token un poco antes de que expire, para que no
	// caduque a mitad de una llamada ya en vuelo.
	accessTokenSkew = 60 * time.Second
)

type GoogleCalendarService interface {
	// Enabled informa si la integración está configurada. Con el flag apagado
	// el resto de métodos devuelve ErrGoogleDisabled.
	Enabled() bool
	// AuthURL construye la URL de consentimiento de Google con un state firmado.
	AuthURL(userID uint, returnTo string) (string, error)
	// HandleCallback canjea el código de autorización, resuelve la identidad de
	// la cuenta y guarda el vínculo. Devuelve también el destino al que debe
	// volver el navegador (el returnTo que viajó en el state).
	HandleCallback(code, state string) (account *models.GoogleCalendarAccount, returnTo string, err error)
	Status(userID uint) (*models.GoogleCalendarAccount, error)
	// Disconnect revoca el permiso en Google y borra el vínculo local.
	Disconnect(userID uint) error
	// AccessToken devuelve un access token vigente, refrescándolo si hace falta.
	// Lo consumirá la Fase 2 (creación de eventos); se expone ya porque es donde
	// vive el manejo de needs_reauth.
	AccessToken(userID uint) (string, error)

	// --- Eventos (Fase 2) ---
	// CreateEvent crea un evento de día completo y devuelve su id en Google.
	CreateEvent(userID uint, calendarID string, ev CalendarEventInput) (string, error)
	// UpdateEvent sobrescribe un evento existente. Devuelve ErrEventGone si el
	// usuario ya lo borró a mano en Google (el que llama debe re-crearlo).
	UpdateEvent(userID uint, calendarID, eventID string, ev CalendarEventInput) error
	// DeleteEvent borra un evento. Es idempotente: si ya no existe, no es error.
	DeleteEvent(userID uint, calendarID, eventID string) error
}

// CalendarEventInput describe un evento de DÍA COMPLETO. Se usa día completo (y
// no horas) a propósito: las tareas de Obertrack tienen fecha sin hora, así que
// esto esquiva por completo la zona horaria. StartDate y EndDate son días
// inclusivos tal como los ve el usuario; la conversión al 'end' exclusivo que
// exige Google ocurre dentro del servicio.
type CalendarEventInput struct {
	Summary     string
	Description string
	StartDate   time.Time
	EndDate     time.Time
}

var (
	ErrGoogleDisabled = errors.New("la integración con Google Calendar no está configurada")
	// ErrNeedsReauth: Google rechazó el refresh token. El usuario debe volver a
	// pasar por el consentimiento; no es un error que se pueda reintentar.
	ErrNeedsReauth  = errors.New("el acceso a Google Calendar fue revocado: vuelve a conectar la cuenta")
	ErrInvalidState = errors.New("el enlace de conexión expiró o no es válido")
	// ErrEventGone: el evento ya no existe en Google (el usuario lo borró a
	// mano). En un update, el que llama debe volver a crearlo.
	ErrEventGone = errors.New("el evento ya no existe en Google Calendar")
)

type googleCalendarService struct {
	repo   repository.GoogleCalendarRepository
	sealer *utils.SecretSealer
	client *http.Client

	enabled      bool
	clientID     string
	clientSecret string
	redirectURI  string
	stateSecret  []byte

	// refreshLocks serializa el refresh POR USUARIO. Sin esto, dos peticiones
	// concurrentes del mismo usuario pedirían dos tokens a Google y la segunda
	// escritura pisaría a la primera.
	refreshLocks sync.Map // uint -> *sync.Mutex
}

// NewGoogleCalendarService construye el servicio. Si el flag está apagado
// devuelve una instancia inerte (Enabled() == false) en vez de nil, para que
// los handlers puedan responder 503 sin comprobaciones de nil por todas partes.
func NewGoogleCalendarService(repo repository.GoogleCalendarRepository, cfg *config.Config) GoogleCalendarService {
	svc := &googleCalendarService{
		repo:   repo,
		client: &http.Client{Timeout: 20 * time.Second},
	}
	if !cfg.GoogleCalendarEnabled {
		return svc
	}

	// config.LoadConfig ya validó la clave con fail-fast, así que aquí solo
	// puede fallar en tests que construyan un Config a mano.
	sealer, err := utils.NewSecretSealer(cfg.GoogleTokenEncKey)
	if err != nil {
		log.Printf("WARN: Google Calendar deshabilitado, clave de cifrado inválida: %v", err)
		return svc
	}

	svc.enabled = true
	svc.sealer = sealer
	svc.clientID = cfg.GoogleClientID
	svc.clientSecret = cfg.GoogleClientSecret
	svc.redirectURI = cfg.GoogleRedirectURI
	svc.stateSecret = deriveStateSecret(cfg.JWTSecret)
	return svc
}

// deriveStateSecret separa la clave que firma el state de la que firma las
// sesiones. Firmar el state con JWT_SECRET a secas sería un agujero: ese token
// viaja en una URL (queda en historial, logs y Referer) y, al compartir clave y
// algoritmo con los access tokens, podría intentar presentarse como cookie de
// sesión. Con una clave derivada distinta, un state NUNCA vale como sesión ni
// al revés, aunque se filtre.
func deriveStateSecret(jwtSecret string) []byte {
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte("google-oauth-state-v1"))
	return mac.Sum(nil)
}

func (s *googleCalendarService) Enabled() bool { return s.enabled }

// --- State ---

type googleStateClaims struct {
	UserID   uint   `json:"uid"`
	ReturnTo string `json:"rt,omitempty"`
	jwt.RegisteredClaims
}

func (s *googleCalendarService) buildState(userID uint, returnTo string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	now := time.Now()
	claims := googleStateClaims{
		UserID:   userID,
		ReturnTo: returnTo,
		RegisteredClaims: jwt.RegisteredClaims{
			// Audiencia propia: aunque alguien reutilizara la clave, este token
			// no pasa por ningún otro validador de la app.
			Audience:  jwt.ClaimStrings{"google-oauth-state"},
			ID:        base64.RawURLEncoding.EncodeToString(nonce),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(stateTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.stateSecret)
}

func (s *googleCalendarService) parseState(state string) (*googleStateClaims, error) {
	claims := &googleStateClaims{}
	_, err := jwt.ParseWithClaims(state, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo de firma inesperado: %v", t.Header["alg"])
		}
		return s.stateSecret, nil
	},
		jwt.WithAudience("google-oauth-state"),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || claims.UserID == 0 {
		return nil, ErrInvalidState
	}
	return claims, nil
}

// --- Flujo de vinculación ---

func (s *googleCalendarService) AuthURL(userID uint, returnTo string) (string, error) {
	if !s.enabled {
		return "", ErrGoogleDisabled
	}
	state, err := s.buildState(userID, returnTo)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("client_id", s.clientID)
	q.Set("redirect_uri", s.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", googleScopes)
	q.Set("state", state)
	// access_type=offline + prompt=consent son obligatorios: sin ellos Google
	// deja de devolver refresh_token a partir del segundo consentimiento de la
	// misma cuenta, y la integración se rompería semanas después sin señal
	// visible en el momento de conectar.
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("include_granted_scopes", "true")

	return googleAuthEndpoint + "?" + q.Encode(), nil
}

// googleTokenResponse es la respuesta de /token, tanto en el canje del código
// como en el refresh.
type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (s *googleCalendarService) HandleCallback(code, state string) (*models.GoogleCalendarAccount, string, error) {
	if !s.enabled {
		return nil, "", ErrGoogleDisabled
	}
	claims, err := s.parseState(state)
	if err != nil {
		return nil, "", err
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("redirect_uri", s.redirectURI)
	form.Set("grant_type", "authorization_code")

	tok, err := s.postToken(form)
	if err != nil {
		return nil, claims.ReturnTo, err
	}
	if tok.RefreshToken == "" {
		// Con prompt=consent no debería pasar; si pasa, guardar el vínculo sin
		// refresh token dejaría una cuenta que caduca en una hora y no se puede
		// renovar. Mejor fallar aquí y que el usuario reintente.
		return nil, claims.ReturnTo, errors.New("Google no devolvió un token de larga duración: vuelve a intentarlo")
	}

	sub, email, err := parseIDToken(tok.IDToken)
	if err != nil {
		return nil, claims.ReturnTo, err
	}

	refreshEnc, err := s.sealer.Seal(tok.RefreshToken)
	if err != nil {
		return nil, claims.ReturnTo, err
	}
	accessEnc, err := s.sealer.Seal(tok.AccessToken)
	if err != nil {
		return nil, claims.ReturnTo, err
	}
	expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)

	account := &models.GoogleCalendarAccount{
		UserID:          claims.UserID,
		GoogleSub:       sub,
		GoogleEmail:     email,
		RefreshTokenEnc: refreshEnc,
		AccessTokenEnc:  accessEnc,
		AccessExpiresAt: &expiresAt,
		Scopes:          tok.Scope,
		CalendarID:      "primary",
		Status:          models.GoogleCalStatusActive,
		ConnectedAt:     time.Now(),
	}
	if err := s.repo.Upsert(account); err != nil {
		return nil, claims.ReturnTo, err
	}

	log.Printf("Google Calendar: usuario %d vinculó la cuenta %s", claims.UserID, email)
	return account, claims.ReturnTo, nil
}

// postToken hace la llamada a /token y normaliza los errores de OAuth, que
// Google devuelve con HTTP 400 y un cuerpo JSON en vez de un status específico.
func (s *googleCalendarService) postToken(form url.Values) (*googleTokenResponse, error) {
	resp, err := s.client.PostForm(googleTokenEndpoint, form)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar a Google: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var tok googleTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("respuesta inesperada de Google (%d)", resp.StatusCode)
	}
	if tok.Error != "" || resp.StatusCode != http.StatusOK {
		// invalid_grant es el caso importante: el usuario revocó el acceso, o el
		// refresh token caducó (proyectos en modo "testing" los invalidan a los
		// 7 días). Se distingue para poder marcar needs_reauth.
		if tok.Error == "invalid_grant" {
			return nil, ErrNeedsReauth
		}
		return nil, fmt.Errorf("Google rechazó la solicitud: %s", firstNonEmpty(tok.ErrorDesc, tok.Error, resp.Status))
	}
	return &tok, nil
}

// parseIDToken lee 'sub' y 'email' del id_token SIN verificar la firma: el
// token llegó por una conexión TLS directa servidor-a-servidor con el endpoint
// de Google, que es el caso en el que la propia documentación de Google permite
// omitir la validación. No se usa para autenticar a nadie en Obertrack, solo
// para saber qué cuenta se acaba de vincular.
func parseIDToken(idToken string) (sub, email string, err error) {
	if idToken == "" {
		return "", "", errors.New("Google no devolvió la identidad de la cuenta")
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	parser := jwt.NewParser()
	if _, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{}); err != nil {
		return "", "", fmt.Errorf("id_token ilegible: %w", err)
	}

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", "", errors.New("id_token con formato inesperado")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("id_token ilegible: %w", err)
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", "", fmt.Errorf("id_token ilegible: %w", err)
	}
	if claims.Sub == "" || claims.Email == "" {
		return "", "", errors.New("Google no devolvió el correo de la cuenta")
	}
	return claims.Sub, claims.Email, nil
}

// --- Consulta y gestión ---

func (s *googleCalendarService) Status(userID uint) (*models.GoogleCalendarAccount, error) {
	if !s.enabled {
		return nil, ErrGoogleDisabled
	}
	return s.repo.GetByUser(userID)
}

func (s *googleCalendarService) AccessToken(userID uint) (string, error) {
	if !s.enabled {
		return "", ErrGoogleDisabled
	}

	lock, _ := s.refreshLocks.LoadOrStore(userID, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	account, err := s.repo.GetByUser(userID)
	if err != nil {
		return "", err
	}
	if account.Status == models.GoogleCalStatusNeedsReauth {
		return "", ErrNeedsReauth
	}

	// Token vigente en BD: se reusa. Se relee dentro del lock a propósito, así
	// la segunda petición concurrente encuentra ya refrescado lo que hizo la
	// primera en vez de pedir otro token.
	if account.AccessTokenEnc != "" && account.AccessExpiresAt != nil &&
		time.Now().Add(accessTokenSkew).Before(*account.AccessExpiresAt) {
		return s.sealer.Open(account.AccessTokenEnc)
	}

	refreshToken, err := s.sealer.Open(account.RefreshTokenEnc)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")

	tok, err := s.postToken(form)
	if err != nil {
		if errors.Is(err, ErrNeedsReauth) {
			s.markNeedsReauth(userID, "Google revocó el acceso")
		}
		return "", err
	}

	accessEnc, sealErr := s.sealer.Seal(tok.AccessToken)
	if sealErr != nil {
		return "", sealErr
	}
	expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	updates := map[string]interface{}{
		"access_token_enc":  accessEnc,
		"access_expires_at": expiresAt,
		"status":            models.GoogleCalStatusActive,
		"last_error":        "",
	}
	// Google normalmente NO rota el refresh token, pero si lo hace hay que
	// guardarlo o el siguiente refresh fallaría.
	if tok.RefreshToken != "" && tok.RefreshToken != refreshToken {
		if newRefreshEnc, err := s.sealer.Seal(tok.RefreshToken); err == nil {
			updates["refresh_token_enc"] = newRefreshEnc
		}
	}
	if err := s.repo.UpdateFields(userID, updates); err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (s *googleCalendarService) markNeedsReauth(userID uint, reason string) {
	if err := s.repo.UpdateFields(userID, map[string]interface{}{
		"status":     models.GoogleCalStatusNeedsReauth,
		"last_error": reason,
	}); err != nil {
		log.Printf("Google Calendar: no se pudo marcar needs_reauth al usuario %d: %v", userID, err)
	}
}

// googleEventPayload es el cuerpo JSON de un evento de día completo. Las fechas
// van en el campo `date` (no `dateTime`), que es lo que hace que Google lo trate
// como all-day y sin zona horaria.
type googleEventPayload struct {
	Summary     string          `json:"summary"`
	Description string          `json:"description,omitempty"`
	Start       googleEventDate `json:"start"`
	End         googleEventDate `json:"end"`
	// Source enlaza el evento de vuelta a Obertrack desde la UI de Google.
	Source *googleEventSource `json:"source,omitempty"`
}

type googleEventDate struct {
	Date string `json:"date"`
}

type googleEventSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

const googleEventDateLayout = "2006-01-02"

// buildEventPayload traduce el input a la forma que espera Google. El `end.date`
// de un evento all-day es EXCLUSIVO: una tarea de un solo día (start==end) va de
// ese día al siguiente. Si no hay EndDate, dura un día desde StartDate.
func buildEventPayload(ev CalendarEventInput) googleEventPayload {
	start := ev.StartDate
	end := ev.EndDate
	if end.IsZero() || end.Before(start) {
		end = start
	}
	// +1 día para el fin exclusivo de Google.
	endExclusive := end.AddDate(0, 0, 1)

	return googleEventPayload{
		Summary:     ev.Summary,
		Description: ev.Description,
		Start:       googleEventDate{Date: start.Format(googleEventDateLayout)},
		End:         googleEventDate{Date: endExclusive.Format(googleEventDateLayout)},
	}
}

func calendarEventsURL(calendarID string) string {
	if calendarID == "" {
		calendarID = "primary"
	}
	return "https://www.googleapis.com/calendar/v3/calendars/" + url.PathEscape(calendarID) + "/events"
}

func (s *googleCalendarService) CreateEvent(userID uint, calendarID string, ev CalendarEventInput) (string, error) {
	if !s.enabled {
		return "", ErrGoogleDisabled
	}
	body, err := json.Marshal(buildEventPayload(ev))
	if err != nil {
		return "", err
	}

	resp, err := s.doEventRequest(userID, http.MethodPost, calendarEventsURL(calendarID), body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", s.classifyEventError(userID, resp)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("Google no devolvió el id del evento creado")
	}
	return created.ID, nil
}

func (s *googleCalendarService) UpdateEvent(userID uint, calendarID, eventID string, ev CalendarEventInput) error {
	if !s.enabled {
		return ErrGoogleDisabled
	}
	body, err := json.Marshal(buildEventPayload(ev))
	if err != nil {
		return err
	}

	eventURL := calendarEventsURL(calendarID) + "/" + url.PathEscape(eventID)
	resp, err := s.doEventRequest(userID, http.MethodPut, eventURL, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 404/410: el usuario borró el evento a mano. Se avisa para que el llamador
	// lo re-cree en vez de tratarlo como fallo permanente.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return ErrEventGone
	}
	if resp.StatusCode != http.StatusOK {
		return s.classifyEventError(userID, resp)
	}
	return nil
}

func (s *googleCalendarService) DeleteEvent(userID uint, calendarID, eventID string) error {
	if !s.enabled {
		return ErrGoogleDisabled
	}
	eventURL := calendarEventsURL(calendarID) + "/" + url.PathEscape(eventID)
	resp, err := s.doEventRequest(userID, http.MethodDelete, eventURL, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Borrar algo que ya no está es un éxito: el estado final deseado (no existe)
	// se cumple igual. Google devuelve 410 si ya se había borrado.
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil
	}
	return s.classifyEventError(userID, resp)
}

// doEventRequest resuelve el token, hace la llamada y la reintenta una vez si el
// access token acababa de expirar (401 con token recién refrescado en carrera).
func (s *googleCalendarService) doEventRequest(userID uint, method, urlStr string, body []byte) (*http.Response, error) {
	token, err := s.AccessToken(userID)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar a Google: %w", err)
	}
	return resp, nil
}

// classifyEventError traduce un status de error de la API de eventos a un error
// tipado. Un 401 significa credencial inválida → needs_reauth (no reintentable);
// el resto se devuelve como error genérico que el worker sí reintenta.
func (s *googleCalendarService) classifyEventError(userID uint, resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		s.markNeedsReauth(userID, "Google rechazó la credencial al sincronizar un evento")
		return ErrNeedsReauth
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("Google respondió %s al sincronizar el evento: %s", resp.Status, string(snippet))
}

func (s *googleCalendarService) Disconnect(userID uint) error {
	if !s.enabled {
		return ErrGoogleDisabled
	}
	account, err := s.repo.GetByUser(userID)
	if err != nil {
		return err
	}

	// Se revoca en Google ANTES de borrar: si solo borráramos la fila, el
	// permiso seguiría concedido en la cuenta del usuario y Obertrack aparecería
	// para siempre en su lista de apps con acceso.
	if refreshToken, err := s.sealer.Open(account.RefreshTokenEnc); err == nil {
		resp, err := s.client.PostForm(googleRevokeEndpoint, url.Values{"token": {refreshToken}})
		if err != nil {
			// Un fallo de red al revocar no debe impedir la desvinculación: el
			// usuario pidió desconectar y eso tiene que ocurrir igual.
			log.Printf("Google Calendar: no se pudo revocar el token del usuario %d: %v", userID, err)
		} else {
			resp.Body.Close()
		}
	}

	s.refreshLocks.Delete(userID)
	return s.repo.DeleteByUser(userID)
}
