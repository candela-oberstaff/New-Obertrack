package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// BrevoService handles email dispatch via the Brevo (Sendinblue) Transactional API.
type BrevoService struct {
	apiKey  string
	apiURL  string
	from    BrevoContact
}

type BrevoContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BrevoEmailRequest struct {
	Sender      BrevoContact   `json:"sender"`
	To          []BrevoContact `json:"to"`
	Subject     string         `json:"subject"`
	HTMLContent string         `json:"htmlContent"`
}

type BrevoErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewBrevoService() *BrevoService {
	return &BrevoService{
		apiKey: os.Getenv("BREVO_API_KEY"),
		apiURL: "https://api.brevo.com/v3/smtp/email",
		from: BrevoContact{
			Name:  getEnvOrDefault("BREVO_SENDER_NAME", "Obertrack"),
			Email: getEnvOrDefault("BREVO_SENDER_EMAIL", "noreply@obertrack.com"),
		},
	}
}

// SendEmail sends a single transactional email via Brevo.
func (s *BrevoService) SendEmail(toEmail, toName, subject, htmlContent string) error {
	if s.apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY is not configured")
	}

	payload := BrevoEmailRequest{
		Sender: s.from,
		To: []BrevoContact{
			{Name: toName, Email: toEmail},
		},
		Subject:     subject,
		HTMLContent: htmlContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", s.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Brevo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var brevoErr BrevoErrorResponse
		json.NewDecoder(resp.Body).Decode(&brevoErr)
		return fmt.Errorf("brevo API error [%d]: %s - %s", resp.StatusCode, brevoErr.Code, brevoErr.Message)
	}

	return nil
}

// SendBulk sends the same email to a list of recipients, one by one.
// For high-volume sending, consider using Brevo's campaign/batch API instead.
func (s *BrevoService) SendBulk(recipients []BrevoContact, subject, htmlContent string) []error {
	var errs []error
	for _, r := range recipients {
		if err := s.SendEmail(r.Email, r.Name, subject, htmlContent); err != nil {
			errs = append(errs, fmt.Errorf("failed to send to %s: %w", r.Email, err))
		}
	}
	return errs
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
