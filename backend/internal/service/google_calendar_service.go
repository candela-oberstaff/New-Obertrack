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
	//
	// meetings.space.readonly lee quién está conectado a una sala. Es el scope
	// correcto —y no meetings.space.created— porque ese último solo alcanza a las
	// salas que la app creó A TRAVÉS de la API de Meet, y las nuestras las crea
	// Calendar con conferenceData.createRequest.
	//
	// OJO: es un scope SENSIBLE. En modo testing funciona con los usuarios de
	// prueba, pero publicar la app exigirá pasar la verificación de Google.
	googleScopes = "openid email " +
		"https://www.googleapis.com/auth/calendar.events " +
		"https://www.googleapis.com/auth/meetings.space.readonly"

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
	// SetDisconnectHook cablea la limpieza que otros módulos necesitan al
	// desvincular una cuenta (hoy: borrar los enlaces tarea↔evento de la Fase 2).
	// Es un callback inyectado —mismo patrón que taskService.SetCalendarSync—
	// porque la dependencia ya va calendarSync→googleCalendar y llamarlo al revés
	// crearía un ciclo. Sin cablear (nil): no se limpia nada.
	SetDisconnectHook(fn func(userID uint))
	// AccessToken devuelve un access token vigente, refrescándolo si hace falta.
	// Lo consumirá la Fase 2 (creación de eventos); se expone ya porque es donde
	// vive el manejo de needs_reauth.
	AccessToken(userID uint) (string, error)

	// --- Eventos ---
	// CreateEvent crea el evento y devuelve lo que Google respondió (id, enlace
	// y, si se pidió, la sala de Meet).
	CreateEvent(userID uint, calendarID string, ev CalendarEventInput) (*CalendarEvent, error)
	// UpdateEvent sobrescribe un evento existente. Devuelve ErrEventGone si el
	// usuario ya lo borró a mano en Google (el que llama debe re-crearlo).
	UpdateEvent(userID uint, calendarID, eventID string, ev CalendarEventInput) error
	// GetEvent consulta un evento. Se usa sobre todo para resolver el enlace de
	// Meet cuando la conferencia quedó ConferencePending al crear.
	GetEvent(userID uint, calendarID, eventID string) (*CalendarEvent, error)
	// DeleteEvent borra un evento. Es idempotente: si ya no existe, no es error.
	DeleteEvent(userID uint, calendarID, eventID string) error

	// --- Meet ---
	// MeetPresence dice cuánta gente hay AHORA en la sala de un enlace de Meet.
	// Usa la API de Meet (otra API, misma credencial) y exige el scope
	// meetings.space.readonly: con un token emitido antes de pedirlo devuelve
	// ErrMeetScopeMissing.
	MeetPresence(userID uint, meetURL string) (*MeetPresence, error)
}

// CalendarEventInput describe un evento a crear o actualizar. Soporta las dos
// formas que necesita Obertrack, y el par de campos que se rellene decide cuál:
//
//   - DÍA COMPLETO (tareas): StartDate/EndDate. Son días inclusivos tal como los
//     ve el usuario; la conversión al 'end' exclusivo que exige Google ocurre en
//     buildEventPayload. Al no llevar hora, esquiva por completo la zona horaria.
//   - CON HORA (sesiones de Meet): StartAt/EndAt + TimeZone.
//
// Rellenar los dos pares es un error de programación: gana el de hora.
type CalendarEventInput struct {
	Summary     string
	Description string

	// Día completo.
	StartDate time.Time
	EndDate   time.Time

	// Con hora. TimeZone es obligatorio si StartAt no es cero, y debe ser un
	// identificador IANA ("America/Bogota"), NO un offset: en un evento
	// recurrente el offset se rompe con el horario de verano y toda la serie se
	// desplaza una hora.
	StartAt  time.Time
	EndAt    time.Time
	TimeZone string

	// Attendees son los correos invitados. Cuando hay al menos uno, Google manda
	// las invitaciones por correo (sendUpdates=all).
	Attendees []string
	// CreateConference pide a Google que genere una sala de Meet para el evento.
	// Solo tiene efecto al crear: en las actualizaciones la conferencia existente
	// se conserva (ver UpdateEvent).
	CreateConference bool
	// Recurrence son reglas RRULE de la serie. Vacío = evento único.
	Recurrence []string
}

// CalendarEvent es lo que Google devuelve tras crear o consultar un evento.
type CalendarEvent struct {
	ID       string
	HTMLLink string
	// MeetURL es el enlace de la videollamada. Vacío si el evento no tiene
	// conferencia o si Google todavía la está creando (ver ConferencePending).
	MeetURL string
	// ConferencePending: Google aceptó el evento pero aún no terminó de crear la
	// sala. Hay que volver a consultar el evento para obtener el enlace.
	ConferencePending bool
}

var (
	ErrGoogleDisabled = errors.New("la integración con Google no está configurada")
	// ErrNeedsReauth: Google rechazó el refresh token. El usuario debe volver a
	// pasar por el consentimiento; no es un error que se pueda reintentar.
	ErrNeedsReauth  = errors.New("el acceso a tu cuenta de Google fue revocado: vuelve a conectarla")
	ErrInvalidState = errors.New("el enlace de conexión expiró o no es válido")
	// ErrEventGone: el evento ya no existe en Google (el usuario lo borró a
	// mano). En un update, el que llama debe volver a crearlo.
	ErrEventGone = errors.New("el evento ya no existe en Google Calendar")
	// ErrGooglePermanent envuelve un rechazo que no mejora esperando (datos
	// inválidos, permiso denegado). El worker lo usa para no gastar la ventana de
	// reintentos en algo que va a fallar igual dentro de dos horas.
	ErrGooglePermanent = errors.New("Google rechazó la petición de forma permanente")
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

	// onDisconnect lo inyecta SetDisconnectHook; nil = sin limpieza extra.
	onDisconnect func(userID uint)
}

func (s *googleCalendarService) SetDisconnectHook(fn func(userID uint)) {
	s.onDisconnect = fn
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

// googleEventPayload es el cuerpo JSON de un evento.
type googleEventPayload struct {
	Summary     string           `json:"summary"`
	Description string           `json:"description,omitempty"`
	Start       googleEventTime  `json:"start"`
	End         googleEventTime  `json:"end"`
	Attendees   []googleAttendee `json:"attendees,omitempty"`
	Recurrence  []string         `json:"recurrence,omitempty"`
	// ConferenceData solo viaja al crear, y solo si se pidió sala. Google exige
	// además conferenceDataVersion=1 en la query para tenerlo en cuenta.
	ConferenceData *googleConferenceData `json:"conferenceData,omitempty"`
}

// googleEventTime cubre las dos formas de fechar un evento. Ambos campos llevan
// omitempty porque Google RECHAZA un evento que traiga `date` y `dateTime` a la
// vez: exactamente uno de los dos debe estar presente.
type googleEventTime struct {
	// Date lo usa el evento de día completo (sin zona horaria).
	Date string `json:"date,omitempty"`
	// DateTime es RFC3339 y va acompañado de TimeZone.
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type googleAttendee struct {
	Email string `json:"email"`
}

type googleConferenceData struct {
	CreateRequest *googleConferenceCreateRequest `json:"createRequest,omitempty"`
}

type googleConferenceCreateRequest struct {
	// RequestID identifica la petición: Google deduplica por él, así que dos
	// llamadas con el mismo id no generan dos salas.
	RequestID             string                      `json:"requestId"`
	ConferenceSolutionKey googleConferenceSolutionKey `json:"conferenceSolutionKey"`
}

type googleConferenceSolutionKey struct {
	Type string `json:"type"`
}

const (
	googleEventDateLayout = "2006-01-02"
	// hangoutsMeet es el tipo de conferencia de Google Meet. Es el único que se
	// puede crear con el scope calendar.events.
	hangoutsMeet = "hangoutsMeet"
)

// buildEventPayload traduce el input a la forma que espera Google.
//
// Con hora (sesiones): `dateTime` RFC3339 + `timeZone` IANA, y el fin es el
// instante real de fin. Con día completo (tareas): `date`, y el fin es
// EXCLUSIVO, así que una tarea de un solo día (start==end) va de ese día al
// siguiente. Si no hay EndDate, dura un día desde StartDate.
func buildEventPayload(ev CalendarEventInput) googleEventPayload {
	payload := googleEventPayload{
		Summary:     ev.Summary,
		Description: ev.Description,
		Recurrence:  ev.Recurrence,
	}
	for _, email := range ev.Attendees {
		payload.Attendees = append(payload.Attendees, googleAttendee{Email: email})
	}
	if ev.CreateConference {
		payload.ConferenceData = &googleConferenceData{
			CreateRequest: &googleConferenceCreateRequest{
				RequestID:             newConferenceRequestID(),
				ConferenceSolutionKey: googleConferenceSolutionKey{Type: hangoutsMeet},
			},
		}
	}

	if !ev.StartAt.IsZero() {
		end := ev.EndAt
		if end.IsZero() || !end.After(ev.StartAt) {
			// Un fin inválido produciría un 400 de Google; una hora es un valor
			// por defecto razonable para una reunión.
			end = ev.StartAt.Add(time.Hour)
		}
		payload.Start = googleEventTime{DateTime: ev.StartAt.Format(time.RFC3339), TimeZone: ev.TimeZone}
		payload.End = googleEventTime{DateTime: end.Format(time.RFC3339), TimeZone: ev.TimeZone}
		return payload
	}

	start := ev.StartDate
	end := ev.EndDate
	if end.IsZero() || end.Before(start) {
		end = start
	}
	// +1 día para el fin exclusivo de Google.
	endExclusive := end.AddDate(0, 0, 1)
	payload.Start = googleEventTime{Date: start.Format(googleEventDateLayout)}
	payload.End = googleEventTime{Date: endExclusive.Format(googleEventDateLayout)}
	return payload
}

// newConferenceRequestID genera el id que deduplica la creación de la sala. No
// hace falta que sea criptográfico, pero sí único por petición; si rand falla se
// cae a una marca de tiempo antes que devolver un id vacío (que Google rechaza).
func newConferenceRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("obertrack-%d", time.Now().UnixNano())
	}
	return "obertrack-" + base64.RawURLEncoding.EncodeToString(buf)
}

// googleEventResponse es la parte de un evento que nos interesa al leerlo.
type googleEventResponse struct {
	ID       string `json:"id"`
	HTMLLink string `json:"htmlLink"`
	// HangoutLink es el atajo que Google rellena con la URL de Meet.
	HangoutLink    string                        `json:"hangoutLink"`
	ConferenceData *googleConferenceDataResponse `json:"conferenceData"`
}

type googleConferenceDataResponse struct {
	EntryPoints   []googleEntryPoint             `json:"entryPoints"`
	CreateRequest *googleCreateRequestStatusWrap `json:"createRequest"`
}

// googleEntryPoint es cada forma de entrar a la conferencia: el enlace de vídeo,
// el teléfono de marcación, etc.
type googleEntryPoint struct {
	EntryPointType string `json:"entryPointType"`
	URI            string `json:"uri"`
}

type googleCreateRequestStatusWrap struct {
	Status googleCreateRequestStatus `json:"status"`
}

type googleCreateRequestStatus struct {
	StatusCode string `json:"statusCode"`
}

// toCalendarEvent normaliza la respuesta. El enlace de Meet se busca primero en
// hangoutLink (que Google rellena en el caso normal) y si no en los entryPoints
// de vídeo, que es donde vive cuando la conferencia se creó con createRequest.
func (r *googleEventResponse) toCalendarEvent() *CalendarEvent {
	ev := &CalendarEvent{ID: r.ID, HTMLLink: r.HTMLLink, MeetURL: r.HangoutLink}
	if r.ConferenceData == nil {
		return ev
	}
	if ev.MeetURL == "" {
		for _, ep := range r.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && ep.URI != "" {
				ev.MeetURL = ep.URI
				break
			}
		}
	}
	// 'pending' significa que Google todavía está creando la sala: el enlace no
	// existe aún y hay que volver a consultar el evento.
	if cr := r.ConferenceData.CreateRequest; cr != nil && cr.Status.StatusCode == "pending" {
		ev.ConferencePending = true
	}
	return ev
}

func calendarEventsURL(calendarID string) string {
	if calendarID == "" {
		calendarID = "primary"
	}
	return "https://www.googleapis.com/calendar/v3/calendars/" + url.PathEscape(calendarID) + "/events"
}

// eventQuery arma los parámetros comunes. sendUpdates=all solo se pide cuando
// hay invitados: es lo que hace que Google les mande el correo de invitación (y
// sin invitados no tendría a quién notificar).
func eventQuery(hasAttendees, withConference bool) string {
	q := url.Values{}
	if hasAttendees {
		q.Set("sendUpdates", "all")
	}
	if withConference {
		q.Set("conferenceDataVersion", "1")
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func (s *googleCalendarService) CreateEvent(userID uint, calendarID string, ev CalendarEventInput) (*CalendarEvent, error) {
	if !s.enabled {
		return nil, ErrGoogleDisabled
	}
	body, err := json.Marshal(buildEventPayload(ev))
	if err != nil {
		return nil, err
	}

	eventsURL := calendarEventsURL(calendarID) + eventQuery(len(ev.Attendees) > 0, ev.CreateConference)
	resp, err := s.doGoogleRequest(userID, http.MethodPost, eventsURL, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, s.classifyEventError(userID, resp)
	}
	var created googleEventResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&created); err != nil {
		return nil, err
	}
	if created.ID == "" {
		return nil, errors.New("Google no devolvió el id del evento creado")
	}
	return created.toCalendarEvent(), nil
}

// UpdateEvent reemplaza el evento (PUT). Deliberadamente NO manda
// conferenceDataVersion=1: con la versión 0 Google ignora la conferencia del
// cuerpo y CONSERVA la que ya tenía el evento, que es justo lo que queremos —el
// enlace de Meet de una sesión no debe cambiar al mover la hora—. Pedir la
// versión 1 sin incluir la conferencia en el cuerpo la borraría.
func (s *googleCalendarService) UpdateEvent(userID uint, calendarID, eventID string, ev CalendarEventInput) error {
	if !s.enabled {
		return ErrGoogleDisabled
	}
	body, err := json.Marshal(buildEventPayload(ev))
	if err != nil {
		return err
	}

	eventURL := calendarEventsURL(calendarID) + "/" + url.PathEscape(eventID) +
		eventQuery(len(ev.Attendees) > 0, false)
	resp, err := s.doGoogleRequest(userID, http.MethodPut, eventURL, body)
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

func (s *googleCalendarService) GetEvent(userID uint, calendarID, eventID string) (*CalendarEvent, error) {
	if !s.enabled {
		return nil, ErrGoogleDisabled
	}
	eventURL := calendarEventsURL(calendarID) + "/" + url.PathEscape(eventID)
	resp, err := s.doGoogleRequest(userID, http.MethodGet, eventURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, ErrEventGone
	}
	if resp.StatusCode != http.StatusOK {
		return nil, s.classifyEventError(userID, resp)
	}
	var event googleEventResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&event); err != nil {
		return nil, err
	}
	return event.toCalendarEvent(), nil
}

func (s *googleCalendarService) DeleteEvent(userID uint, calendarID, eventID string) error {
	if !s.enabled {
		return ErrGoogleDisabled
	}
	// sendUpdates=all para que Google avise a los invitados de que la reunión se
	// canceló. En los eventos de tarea no hay invitados, así que no cambia nada.
	eventURL := calendarEventsURL(calendarID) + "/" + url.PathEscape(eventID) + "?sendUpdates=all"
	resp, err := s.doGoogleRequest(userID, http.MethodDelete, eventURL, nil)
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

// doGoogleRequest resuelve el token y hace la llamada. No reintenta: AccessToken
// renueva con 60s de margen (accessTokenSkew), así que un 401 aquí no es un token
// recién caducado sino una credencial revocada, y repetirla daría otro 401.
// classifyEventError lo traduce a needs_reauth.
func (s *googleCalendarService) doGoogleRequest(userID uint, method, urlStr string, body []byte) (*http.Response, error) {
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
// tipado, para que el worker sepa si tiene sentido reintentar:
//   - 401 → needs_reauth: credencial inválida, la arregla el usuario reconectando.
//   - 4xx que no sea 408/429 → ErrGooglePermanent: la petición está mal (datos
//     inválidos, permiso denegado) y repetirla da la misma respuesta.
//   - el resto → error genérico, que el worker reintenta con backoff.
func (s *googleCalendarService) classifyEventError(userID uint, resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		s.markNeedsReauth(userID, "Google rechazó la credencial al sincronizar un evento")
		return ErrNeedsReauth
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if isTransientStatus(resp.StatusCode) {
		return fmt.Errorf("Google respondió %s al sincronizar el evento: %s", resp.Status, string(snippet))
	}
	return fmt.Errorf("%w (%s): %s", ErrGooglePermanent, resp.Status, string(snippet))
}

// isTransientStatus distingue lo que se arregla solo con el tiempo. 429 es cuota
// agotada (el caso más probable en esta integración: la API de Calendar limita
// por usuario y minuto) y 5xx es un incidente del lado de Google; ambos se
// resuelven esperando. 408 es un timeout que Google se atribuye. Cualquier otro
// 4xx es un problema de la petición, y reintentarlo solo quema cuota.
func isTransientStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusRequestTimeout ||
		code >= 500
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
	if err := s.repo.DeleteByUser(userID); err != nil {
		return err
	}

	// Limpieza de los módulos que colgaban del vínculo (Fase 2: los enlaces
	// tarea↔evento). Va DESPUÉS de borrar la fila para que un fallo aquí no deje
	// al usuario conectado a medias: lo que pidió —desvincular— ya ocurrió.
	if s.onDisconnect != nil {
		s.onDisconnect(userID)
	}
	return nil
}
