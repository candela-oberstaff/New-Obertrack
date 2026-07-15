package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
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

	// --- Outbound anti-ban throttle ---
	// WhatsApp bans numbers that behave like bots (bursts, no pauses). All sends
	// funnel through a single serialized gate that (a) enforces a minimum spacing
	// between consecutive messages and (b) caps the number of messages per rolling
	// minute. Combined with the human-typing sequence it makes traffic look manual.
	sendMu      sync.Mutex
	lastSendAt  time.Time
	sendWindow  []time.Time   // timestamps of sends in the last minute (sliding window)
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

type WahaSendTextRequest struct {
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
	Session string `json:"session"`
}

func (s *WahaService) SendMessage(session string, to string, text string) error {
	chatID := to
	if !strings.Contains(chatID, "@") {
		chatID = fmt.Sprintf("%s@c.us", chatID)
	}

	// 1) Anti-ban gate: rejects when over the per-minute cap, otherwise blocks just
	// long enough to honor the minimum spacing between messages.
	if err := s.throttleGate(); err != nil {
		return err
	}

	// 2) Human-like pre-send: mark the chat as seen and show "typing…" for a delay
	// proportional to the message length. Best-effort — failures don't block the send.
	if s.humanTyping {
		s.sendSeen(session, chatID)
		s.startTyping(session, chatID)
		time.Sleep(typingDelay(text))
		s.stopTyping(session, chatID)
	}

	// 3) Actual send.
	return s.postSendText(session, chatID, text)
}

// throttleGate serializes all outbound sends. It enforces a rolling per-minute
// cap (returning apperrors.ErrRateLimited when exceeded) and a minimum spacing
// between consecutive messages (sleeping while holding the lock, which naturally
// prevents parallel bursts).
func (s *WahaService) throttleGate() error {
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
		return apperrors.ErrRateLimited
	}

	if s.minInterval > 0 && !s.lastSendAt.IsZero() {
		if gap := now.Sub(s.lastSendAt); gap < s.minInterval {
			time.Sleep(s.minInterval - gap)
			now = time.Now()
		}
	}

	s.lastSendAt = now
	s.sendWindow = append(s.sendWindow, now)
	return nil
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

// postSendText performs the raw sendText call to WAHA.
func (s *WahaService) postSendText(session, chatID, text string) error {
	payload := WahaSendTextRequest{ChatID: chatID, Text: text, Session: session}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal waha payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/sendText", s.apiURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to WAHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}
	return nil
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

func (s *WahaService) sendSeen(session, chatID string)    { s.postChatAction("sendSeen", session, chatID) }
func (s *WahaService) startTyping(session, chatID string) { s.postChatAction("startTyping", session, chatID) }
func (s *WahaService) stopTyping(session, chatID string)  { s.postChatAction("stopTyping", session, chatID) }

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

	client := s.client
	resp, err := client.Do(req)
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

func (s *WahaService) GetAllContacts(session string) ([]WahaContactResponse, error) {
	url := fmt.Sprintf("%s/api/%s/contacts/all", s.apiURL, session)
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
		return nil, fmt.Errorf("failed to fetch contacts from WAHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}

	var contacts []WahaContactResponse
	if err := json.NewDecoder(resp.Body).Decode(&contacts); err != nil {
		return nil, fmt.Errorf("failed to decode contacts: %w", err)
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
	resp, err := s.client.Do(req)
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

