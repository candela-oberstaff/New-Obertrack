package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type WahaService struct {
	apiURL  string
	apiKey  string
	session string
}

func NewWahaService() *WahaService {
	return &WahaService{
		apiURL:  getEnvOrDefault("WAHA_API_URL", "http://localhost:3000"), // Default WAHA port
		apiKey:  getEnvOrDefault("WAHA_API_KEY", ""),                      // Optional API Key
		session: getEnvOrDefault("WAHA_SESSION", "default"),               // Session name (e.g. 'default')
	}
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

	payload := WahaSendTextRequest{
		ChatID:  chatID,
		Text:    text,
		Session: session,
	}

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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to WAHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}

	return nil
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

	client := &http.Client{}
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

	client := &http.Client{}
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

	client := &http.Client{}
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

