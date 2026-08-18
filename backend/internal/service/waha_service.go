package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
)

type WahaService struct {
	apiURL  string
	apiKey  string
	session string
	// client is shared across all requests and carries a timeout so a slow or
	// hung WAHA server can never block a request handler (or the ContactSync
	// goroutine) indefinitely.
	client *http.Client
	// readClient sirve las lecturas pesadas (historial de chats, libreta de
	// contactos). Va aparte porque los plazos útiles son opuestos: un envío tiene
	// que fallar rápido —hay un agente esperando— mientras que chats/overview
	// contra una instancia remota tarda decenas de segundos. Con los 10s del
	// cliente de envío el import del historial se caía en cada vuelta y la
	// bandeja se quedaba congelada en la última sincronización que sí entró.
	readClient *http.Client

	// --- Outbound anti-ban throttle ---
	// WhatsApp bans numbers that behave like bots (bursts, no pauses). All sends
	// funnel through a single serialized gate that (a) enforces a minimum spacing
	// between consecutive messages and (b) caps the number of messages per rolling
	// minute. Combined with the human-typing sequence it makes traffic look manual.
	sendMu         sync.Mutex
	lastSendAt     time.Time
	sendWindow     []time.Time   // timestamps of sends in the last minute (sliding window)
	maxPerMin      int           // hard ceiling per rolling minute
	minInterval    time.Duration // minimum gap between two consecutive sends
	humanTyping    bool          // send "seen" + "typing…" with a proportional delay before sending
	requireInbound bool          // only allow sending to contacts that messaged first (anti cold-outreach)
}

// RequireInboundBeforeSend reports whether cold-outreach protection is enabled:
// outbound WhatsApp messages are only allowed to contacts that wrote first.
func (s *WahaService) RequireInboundBeforeSend() bool { return s.requireInbound }

func NewWahaService() *WahaService {
	return &WahaService{
		apiURL:  getEnvOrDefault("WAHA_API_URL", "http://localhost:3000"), // Default WAHA port
		apiKey:  getEnvOrDefault("WAHA_API_KEY", ""),                      // Optional API Key
		session: getEnvOrDefault("WAHA_SESSION", "default"),               // Session name (e.g. 'default')
		client:  &http.Client{Timeout: 10 * time.Second},
		readClient: &http.Client{
			Timeout: time.Duration(envInt("WAHA_READ_TIMEOUT_S", 45)) * time.Second,
		},

		maxPerMin:      envInt("WAHA_MAX_MSGS_PER_MIN", 20),
		minInterval:    time.Duration(envInt("WAHA_MIN_SEND_INTERVAL_MS", 1500)) * time.Millisecond,
		humanTyping:    envBool("WAHA_HUMAN_TYPING", true),
		requireInbound: envBool("WAHA_REQUIRE_INBOUND", true),
	}
}

// envInt reads an integer env var with a fallback (invalid/empty -> fallback).
func envInt(key string, fallback int) int {
	if v := getEnvOrDefault(key, ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// envBool reads a boolean env var with a fallback.
func envBool(key string, fallback bool) bool {
	if v := getEnvOrDefault(key, ""); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func (s *WahaService) GetSession() string {
	return s.session
}

// reader devuelve el cliente para lecturas pesadas, cayendo al de envío cuando el
// servicio se construyó a mano (tests) y no lo tiene.
func (s *WahaService) reader() *http.Client {
	if s.readClient != nil {
		return s.readClient
	}
	return s.client
}

type WahaSendTextRequest struct {
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
	Session string `json:"session"`
}

// SendMessage delivers a text message and returns the WAHA message ID assigned to
// it. That ID is persisted as the message's external_id so a later history import
// recognises our own outbound message instead of inserting it a second time.
func (s *WahaService) SendMessage(session string, to string, text string) (string, error) {
	chatID := to
	if !strings.Contains(chatID, "@") {
		chatID = fmt.Sprintf("%s@c.us", chatID)
	}

	backoff := 500 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= sendMaxAttempts; attempt++ {
		// 1) Anti-ban gate, en CADA intento. Reservar turno solo antes del primero
		// dejaba que los reintentos salieran pegados al anterior y sin contar en la
		// ventana por minuto: justo la ráfaga que el gate existe para evitar.
		if err := s.awaitSendSlot(); err != nil {
			if lastErr != nil {
				// Ya se tocó WAHA y falló. Ese fallo es la causa real y merece gastar
				// un intento; devolver "sin turno" lo reprogramaría a los 5s en bucle.
				return "", lastErr
			}
			return "", err
		}

		// 2) Human-like pre-send: mark the chat as seen and show "typing…" for a delay
		// proportional to the message length. Best-effort — failures don't block the send.
		// Solo antes del primer envío: repetirlo en cada reintento no aporta realismo
		// y retrasa la entrega varios segundos más.
		if attempt == 1 && s.humanTyping {
			s.sendSeen(session, chatID)
			s.startTyping(session, chatID)
			time.Sleep(typingDelay(text))
			s.stopTyping(session, chatID)
		}

		// 3) Actual send, retrying transient failures so a blip in the WAHA container
		// doesn't silently drop an agent's reply.
		msgID, err := s.postSendText(session, chatID, text)
		if err == nil {
			return msgID, nil
		}
		if !isRetryableSendErr(err) {
			return "", err
		}
		lastErr = err
		log.Printf("[WAHA] send attempt %d/%d failed: %v", attempt, sendMaxAttempts, err)
		if attempt < sendMaxAttempts {
			time.Sleep(backoff)
			backoff *= 3
		}
	}
	return "", lastErr
}

// awaitSendSlot reserva el próximo turno de envío y espera a que llegue. La
// espera ocurre fuera del lock (ver reserveSlot), así que varios remitentes hacen
// cola en paralelo en vez de serializarse detrás del mutex.
func (s *WahaService) awaitSendSlot() error {
	sendAt, err := s.reserveSlot()
	if err != nil {
		return err
	}
	if wait := time.Until(sendAt); wait > 0 {
		time.Sleep(wait)
	}
	return nil
}

// reserveSlot books the timestamp at which the caller is allowed to send. It
// enforces a rolling per-minute cap (returning apperrors.ErrRateLimited when
// exceeded) and a minimum spacing between consecutive messages.
//
// The reservation is made under the lock but the waiting happens outside of it:
// concurrent senders each get their own slot and queue up in parallel instead of
// serializing whole HTTP handlers behind a mutex held during a sleep.
func (s *WahaService) reserveSlot() (time.Time, error) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)
	kept := s.sendWindow[:0]
	for _, t := range s.sendWindow {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.sendWindow = kept

	if s.maxPerMin > 0 && len(s.sendWindow) >= s.maxPerMin {
		log.Printf("[WAHA] outbound rate limit hit (%d/min) — rejecting send", s.maxPerMin)
		return time.Time{}, apperrors.ErrRateLimited
	}

	// The slot is the later of "now" and "one interval after the last reserved
	// slot", so a burst of callers spreads out instead of stacking on the clock.
	sendAt := now
	if s.minInterval > 0 && !s.lastSendAt.IsZero() {
		if earliest := s.lastSendAt.Add(s.minInterval); earliest.After(sendAt) {
			sendAt = earliest
		}
	}

	s.lastSendAt = sendAt
	s.sendWindow = append(s.sendWindow, sendAt)
	return sendAt, nil
}

// typingDelay returns a human-like "typing…" duration: a base plus time
// proportional to the message length, capped, with random jitter.
func typingDelay(text string) time.Duration {
	d := 700*time.Millisecond + time.Duration(len([]rune(text)))*35*time.Millisecond
	if d > 4*time.Second {
		d = 4 * time.Second
	}
	return d + time.Duration(rand.Intn(700))*time.Millisecond
}

// wahaHTTPError is a non-2xx response from WAHA. It carries the status code so
// the retry logic can tell a transient server-side failure from a permanent one.
type wahaHTTPError struct{ status int }

func (e *wahaHTTPError) Error() string { return fmt.Sprintf("waha API error: status %d", e.status) }

// isRetryableSendErr reports whether a failed send is worth repeating: transport
// errors (WAHA restarting, connection reset) and 5xx/429 responses. A 4xx such as
// 422 "session does not exist" is permanent — retrying only delays the error.
func isRetryableSendErr(err error) bool {
	// Un envío de resultado desconocido NO se reintenta: el motor WEBJS tarda de
	// sobra en responder y para cuando vence el plazo el mensaje suele estar ya
	// entregado, así que repetirlo se lo deja duplicado al contacto en el teléfono.
	// Lo resuelve el llamador reconciliando contra el chat (FindDeliveredMessage).
	if errors.Is(err, apperrors.ErrSendUncertain) {
		return false
	}
	var httpErr *wahaHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status >= 500 || httpErr.status == http.StatusTooManyRequests
	}
	return true // transport-level failure
}

const sendMaxAttempts = 3

// postSendText performs the raw sendText call to WAHA and returns the ID WAHA
// assigned to the message (empty if the response carries none).
func (s *WahaService) postSendText(session, chatID, text string) (string, error) {
	payload := WahaSendTextRequest{ChatID: chatID, Text: text, Session: session}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal waha payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/sendText", s.apiURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		// Un timeout no significa "no se envió", significa "no sé": WAHA pudo haber
		// entregado el mensaje y habérsenos agotado el plazo esperando su respuesta.
		// Se marca aparte para que el llamador lo compruebe antes de reenviar.
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "", fmt.Errorf("%w: %v", apperrors.ErrSendUncertain, err)
		}
		return "", fmt.Errorf("failed to send request to WAHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &wahaHTTPError{status: resp.StatusCode}
	}

	// The message ID is best-effort: a send that succeeded must not be reported as
	// failed just because the response shape was unexpected.
	var sent wahaSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&sent); err != nil {
		log.Printf("[WAHA] could not decode sendText response (message was sent): %v", err)
		return "", nil
	}
	return sent.messageID(), nil
}

// wahaSendResponse captures the sendText reply. Depending on the engine, `id` is
// either a plain string ("true_1234@c.us_ABC") or an object whose `_serialized`
// field holds that same string, so it is decoded lazily.
type wahaSendResponse struct {
	ID json.RawMessage `json:"id"`
}

func (r *wahaSendResponse) messageID() string {
	if len(r.ID) == 0 {
		return ""
	}
	var asString string
	if json.Unmarshal(r.ID, &asString) == nil {
		return asString
	}
	var asObject struct {
		Serialized string `json:"_serialized"`
	}
	if json.Unmarshal(r.ID, &asObject) == nil {
		return asObject.Serialized
	}
	return ""
}

// postChatAction fires a best-effort presence/read action (sendSeen, startTyping,
// stopTyping) for a chat. Errors are swallowed: these must never block a send.
func (s *WahaService) postChatAction(endpoint, session, chatID string) {
	payload := map[string]string{"chatId": chatID, "session": session}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/%s", s.apiURL, endpoint), bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (s *WahaService) sendSeen(session, chatID string) { s.postChatAction("sendSeen", session, chatID) }
func (s *WahaService) startTyping(session, chatID string) {
	s.postChatAction("startTyping", session, chatID)
}
func (s *WahaService) stopTyping(session, chatID string) {
	s.postChatAction("stopTyping", session, chatID)
}

type WahaContactResponse struct {
	ID        string `json:"id"`
	Number    string `json:"number"`
	Name      string `json:"name"`
	Pushname  string `json:"pushname"`
	ShortName string `json:"shortName"`
	Phone     string `json:"phone"`
}

func (m *WahaContactResponse) BestName() string {
	for _, v := range []string{m.Name, m.Pushname, m.ShortName} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (m *WahaContactResponse) RealPhone() string {
	if i := strings.IndexByte(m.ID, '@'); i > 0 && strings.HasSuffix(m.ID, "@c.us") {
		return m.ID[:i]
	}
	return strings.TrimSpace(m.Phone)
}

func (m *WahaContactResponse) GetDisplayName() string { return m.BestName() }

func (m *WahaContactResponse) GetPhone() string { return m.RealPhone() }

func (s *WahaService) GetContact(session string, contactID string) (*WahaContactResponse, error) {
	if !strings.Contains(contactID, "@") {
		contactID = fmt.Sprintf("%s@c.us", contactID)
	}

	reqURL := fmt.Sprintf("%s/api/contacts?session=%s&contactId=%s",
		s.apiURL, url.QueryEscape(session), url.QueryEscape(contactID))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.reader().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contact from WAHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}

	var contact WahaContactResponse
	if err := json.NewDecoder(resp.Body).Decode(&contact); err != nil {
		return nil, fmt.Errorf("failed to decode contact: %w", err)
	}
	if contact.ID != "" || contact.BestName() != "" {
		return &contact, nil
	}

	return nil, fmt.Errorf("contact not found")
}

// GetAllContacts returns the full contact book of a session.
//
// Note the URL shape: WAHA exposes the *contacts* endpoints with the session as a
// query parameter (/api/contacts/all?session=X), unlike the *chats* endpoints
// which take it in the path (/api/{session}/chats/...). Using the path form here
// returns HTTP 500 and made ContactSync fail silently.
func (s *WahaService) GetAllContacts(session string) ([]WahaContactResponse, error) {
	reqURL := fmt.Sprintf("%s/api/contacts/all?session=%s", s.apiURL, url.QueryEscape(session))
	var contacts []WahaContactResponse
	if err := s.getJSON(reqURL, &contacts); err != nil {
		return nil, err
	}
	return contacts, nil
}

type WahaSessionStatusResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "CONNECTED", "STOPPED", etc.
	QR     struct {
		Raw   string `json:"raw"`
		Image string `json:"image"`
	} `json:"qr"`
}

// GetSessionStatusAndQR gets the session status and active QR code if not authenticated
func (s *WahaService) GetSessionStatusAndQR(session string) (*WahaSessionStatusResponse, error) {
	url := fmt.Sprintf("%s/api/sessions/%s", s.apiURL, session)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	client := s.client
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch session status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}

	var status WahaSessionStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode session status: %w", err)
	}

	// WAHA v4 returns the status in "status". Standard states are: "SCAN_QR", "WORKING", "FAILED", etc.
	// If the state is not "WORKING" (or "CONNECTED" depending on version), fetch the QR image from the auth endpoint
	if status.Status != "WORKING" && status.Status != "CONNECTED" {
		qrUrl := fmt.Sprintf("%s/api/%s/auth/qr", s.apiURL, session)
		qrReq, err := http.NewRequest("GET", qrUrl, nil)
		if err == nil {
			qrReq.Header.Set("accept", "application/json")
			if s.apiKey != "" {
				qrReq.Header.Set("X-Api-Key", s.apiKey)
			}
			respQR, errQR := client.Do(qrReq)
			if errQR == nil && respQR.StatusCode == 200 {
				var qrData struct {
					Raw   string `json:"raw"`
					Image string `json:"image"`
				}
				if json.NewDecoder(respQR.Body).Decode(&qrData) == nil {
					status.QR.Raw = qrData.Raw
					status.QR.Image = qrData.Image
				}
				respQR.Body.Close()
			}
		}
	}

	return &status, nil
}

// WahaChatOverview is one entry from /chats/overview: a chat and its last message.
type WahaChatOverview struct {
	ID   string `json:"id"`   // e.g. "1234@c.us" or "1234@g.us" (group)
	Name string `json:"name"` // display name, may be empty
}

// WahaChatMessage is one message from /chats/{chatId}/messages.
type WahaChatMessage struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"` // unix seconds
	Body      string `json:"body"`
	FromMe    bool   `json:"fromMe"`
	Type      string `json:"type"`
	From      string `json:"from"`
	// HasMedia is the only reliable media signal on this endpoint: the WEBJS
	// engine returns `type: null` for history messages, so Type is often empty.
	HasMedia bool `json:"hasMedia"`
	Media    *struct {
		Mimetype string `json:"mimetype"`
		Filename string `json:"filename"`
		URL      string `json:"url"`
	} `json:"media"`
}

// MimeType returns the message's media mimetype when WAHA provided one.
func (m *WahaChatMessage) MimeType() string {
	if m.Media == nil {
		return ""
	}
	return m.Media.Mimetype
}

// IsGroupChat reports whether a chat/JID belongs to a group conversation.
func IsGroupChat(chatID string) bool { return strings.HasSuffix(chatID, "@g.us") }

// IsIndividualChat reports whether a chat/JID is a 1:1 conversation.
//
// WhatsApp now addresses individual chats by LID ("1128288...@lid") rather than
// by phone JID ("17873491050@c.us"), and both forms appear side by side. Matching
// only "@c.us" — as the history import used to — skips every modern chat.
func IsIndividualChat(chatID string) bool {
	return strings.HasSuffix(chatID, "@c.us") || strings.HasSuffix(chatID, "@lid")
}

// MediaPlaceholder returns the text stored for a message that carries an
// attachment instead of text, so the conversation shows that *something* arrived
// rather than dropping it silently. Returns "" when the message is not media.
func MediaPlaceholder(msgType, mimetype string, hasMedia bool) string {
	kind := strings.ToLower(strings.TrimSpace(msgType))
	if kind == "" && mimetype != "" {
		kind, _, _ = strings.Cut(mimetype, "/")
	}
	switch kind {
	case "image":
		return "📷 Imagen recibida"
	case "video":
		return "🎥 Video recibido"
	case "audio", "ptt", "voice":
		return "🎤 Nota de voz recibida"
	case "document", "application":
		return "📄 Documento recibido"
	case "sticker":
		return "🏷️ Sticker recibido"
	case "location":
		return "📍 Ubicación recibida"
	case "vcard", "contact_card", "contact":
		return "👤 Contacto recibido"
	}
	if hasMedia {
		return "📎 Archivo adjunto recibido"
	}
	return ""
}

// GetChatsOverview returns the most recent chats of a session (with their last
// message). Used to import existing conversations when the session connects.
func (s *WahaService) GetChatsOverview(session string, limit int) ([]WahaChatOverview, error) {
	reqURL := fmt.Sprintf("%s/api/%s/chats/overview?limit=%d", s.apiURL, session, limit)
	var chats []WahaChatOverview
	if err := s.getJSON(reqURL, &chats); err != nil {
		return nil, err
	}
	return chats, nil
}

// GetChatMessages returns the most recent messages of a chat (newest first as
// WAHA returns them). downloadMedia is disabled to keep the import light.
func (s *WahaService) GetChatMessages(session, chatID string, limit int) ([]WahaChatMessage, error) {
	reqURL := fmt.Sprintf("%s/api/%s/chats/%s/messages?limit=%d&downloadMedia=false",
		s.apiURL, session, url.PathEscape(chatID), limit)
	var msgs []WahaChatMessage
	if err := s.getJSON(reqURL, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// reconcileScanLimit acota cuántos mensajes recientes del chat se miran al
// reconciliar un envío incierto. La copia que se busca, si existe, se entregó
// hace segundos: mirar más lejos solo aumenta el riesgo de casar con otra cosa.
const reconcileScanLimit = 10

// reconcileClockSkew tolera que el reloj del contenedor de WAHA y el nuestro no
// vayan exactamente iguales al comparar el timestamp del mensaje con `since`.
const reconcileClockSkew = 10 * time.Second

// FindDeliveredMessage busca en el chat un mensaje propio con este texto enviado
// después de `since`, y devuelve el ID que WAHA le asignó.
//
// Resuelve el envío que terminó en timeout: reenviarlo a ciegas le dejaría al
// contacto el mensaje repetido en el teléfono, y darlo por fallido perdería una
// respuesta que sí llegó. Preguntarle al chat es lo único que distingue los dos
// casos. El filtro por fecha evita casar con un "ok" idéntico de ayer.
func (s *WahaService) FindDeliveredMessage(session, chatID, text string, since time.Time) (string, bool) {
	if !strings.Contains(chatID, "@") {
		chatID = fmt.Sprintf("%s@c.us", chatID)
	}
	msgs, err := s.GetChatMessages(session, chatID, reconcileScanLimit)
	if err != nil {
		// Sin poder comprobarlo se responde "no encontrado": el llamador reintentará,
		// que es preferible a cerrar como enviado algo que quizá nunca salió.
		log.Printf("[WAHA] no se pudo reconciliar el envío a %s: %v", chatID, err)
		return "", false
	}
	want := strings.TrimSpace(text)
	floor := since.Add(-reconcileClockSkew).Unix()
	for _, m := range msgs {
		if !m.FromMe || m.Timestamp < floor {
			continue
		}
		if strings.TrimSpace(m.Body) == want {
			return m.ID, true
		}
	}
	return "", false
}

// getJSON performs an authenticated GET and decodes the JSON body into out.
func (s *WahaService) getJSON(url string, out interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("accept", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}
	resp, err := s.reader().Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch from WAHA: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// StartSession asks WAHA to (re)start a session. Powers the "force connection"
// action so operators can bring a dropped/failed session back up from the app
// without opening the WAHA dashboard. Treats "already started" (422) as success.
func (s *WahaService) StartSession(session string) error {
	url := fmt.Sprintf("%s/api/sessions/%s/start", s.apiURL, session)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start WAHA session: %w", err)
	}
	defer resp.Body.Close()

	// 2xx = started; 422 = already started (idempotent for a force button).
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}
	return nil
}

// DownloadMessageMedia proxies the download request to WAHA and returns the response body (stream) and content type.
// The caller is responsible for closing the io.ReadCloser.
func (s *WahaService) DownloadMessageMedia(session, chatID, messageID string) (io.ReadCloser, string, error) {
	// 1. Pide a WAHA que recupere el mensaje y descargue el adjunto (downloadMedia=true)
	//    Los IDs de WAHA contienen '@' que debe codificarse en la URL.
	urlInfo := fmt.Sprintf("%s/api/%s/chats/%s/messages/%s?downloadMedia=true",
		s.apiURL, session,
		url.PathEscape(chatID),
		url.PathEscape(messageID),
	)
	reqInfo, err := http.NewRequest("GET", urlInfo, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create info request: %w", err)
	}
	if s.apiKey != "" {
		reqInfo.Header.Set("X-Api-Key", s.apiKey)
	}

	respInfo, err := s.reader().Do(reqInfo)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch message info from WAHA: %w", err)
	}
	defer respInfo.Body.Close()

	if respInfo.StatusCode < 200 || respInfo.StatusCode >= 300 {
		return nil, "", fmt.Errorf("waha info error: status %d", respInfo.StatusCode)
	}

	var msg WahaChatMessage
	if err := json.NewDecoder(respInfo.Body).Decode(&msg); err != nil {
		return nil, "", fmt.Errorf("waha decode error: %w", err)
	}

	if msg.Media == nil {
		return nil, "", fmt.Errorf("el mensaje no contiene adjuntos")
	}

	// 2. Extraer la URL del archivo.
	//    WAHA devuelve el URL con http://localhost:PORT (su propio contenedor).
	//    Hay que reemplazar esa base por la URL remota real.
	fileURL := msg.Media.URL
	if fileURL == "" {
		return nil, "", fmt.Errorf("WAHA no devolvió un enlace al archivo")
	}
	if strings.HasPrefix(fileURL, "http://localhost") || strings.HasPrefix(fileURL, "http://127.") {
		// Extraer solo el path (/api/files/...) y reemplazar el host
		if parsed, err := url.Parse(fileURL); err == nil {
			fileURL = s.apiURL + parsed.Path
		}
	}

	// 3. Descargar y transmitir el archivo
	reqFile, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create file request: %w", err)
	}
	if s.apiKey != "" {
		reqFile.Header.Set("X-Api-Key", s.apiKey)
	}

	respFile, err := s.reader().Do(reqFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch file from WAHA: %w", err)
	}
	if respFile.StatusCode < 200 || respFile.StatusCode >= 300 {
		respFile.Body.Close()
		return nil, "", fmt.Errorf("waha file error: status %d", respFile.StatusCode)
	}

	return respFile.Body, msg.Media.Mimetype, nil
}
