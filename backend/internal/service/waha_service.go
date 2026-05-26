package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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

// SendMessage sends a text message via WAHA to a specific WhatsApp number
func (s *WahaService) SendMessage(session string, toPhone string, text string) error {
	// Format to WhatsApp chat ID
	chatID := toPhone
	if !strings.HasSuffix(chatID, "@c.us") {
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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// GetContact fetches contact details from WAHA
func (s *WahaService) GetContact(session string, contactID string) (*WahaContactResponse, error) {
	if !strings.Contains(contactID, "@") {
		contactID = fmt.Sprintf("%s@c.us", contactID)
	}

	url := fmt.Sprintf("%s/api/%s/contacts/all?id=%s", s.apiURL, session, contactID)
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
		return nil, fmt.Errorf("failed to fetch contact from WAHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("waha API error: status %d", resp.StatusCode)
	}

	var contacts []WahaContactResponse
	if err := json.NewDecoder(resp.Body).Decode(&contacts); err != nil {
		return nil, fmt.Errorf("failed to decode contact: %w", err)
	}

	if len(contacts) > 0 {
		return &contacts[0], nil
	}

	return nil, fmt.Errorf("contact not found")
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

	return &status, nil
}

