package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// SlackService publica mensajes en un canal vía Incoming Webhook.
// Sin SLACK_WEBHOOK_URL configurada queda deshabilitado silenciosamente.
type SlackService struct {
	webhookURL string
	client     *http.Client
}

func NewSlackService() *SlackService {
	return &SlackService{
		webhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackService) Enabled() bool {
	return s.webhookURL != ""
}

// Notify envía un mensaje de texto (admite el markdown básico de Slack).
func (s *SlackService) Notify(text string) error {
	if !s.Enabled() {
		return nil
	}
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook respondió %d", resp.StatusCode)
	}
	return nil
}
