package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type WahaService struct {
	apiURL string
	apiKey string
}

func NewWahaService() *WahaService {
	return &WahaService{
		apiURL: getEnvOrDefault("WAHA_API_URL", "http://localhost:3000"), // Default WAHA port
		apiKey: getEnvOrDefault("WAHA_API_KEY", ""),                      // Optional API Key
	}
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
