package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
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

type BrevoAttachment struct {
	Name    string `json:"name"`
	Content string `json:"content"` // base64 encoded bytes
}

type BrevoEmailRequest struct {
	Sender      BrevoContact      `json:"sender"`
	To          []BrevoContact    `json:"to"`
	Subject     string            `json:"subject"`
	HTMLContent string            `json:"htmlContent"`
	Attachment  []BrevoAttachment `json:"attachment,omitempty"`
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

func (s *BrevoService) SendEmailWithAttachments(toEmail, toName, subject, htmlContent string, attachments []BrevoAttachment) error {
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
		Attachment:  attachments,
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

// SendTicketUpdateEmail sends a ticket update email using a styled HTML template.
func (s *BrevoService) SendTicketUpdateEmail(toEmail, toName string, ticketID uint, ticketTitle, content string) error {
	if s.apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY is not configured")
	}

	subject := fmt.Sprintf("[Obertrack - Ticket #%d] %s", ticketID, ticketTitle)

	// Clean up content to preserve linebreaks in HTML
	escapedContent := html.EscapeString(content)
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: #f3f4f6;
      color: #1f2937;
      margin: 0;
      padding: 0;
    }
    .container {
      max-width: 600px;
      margin: 40px auto;
      background-color: #ffffff;
      border-radius: 12px;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
      overflow: hidden;
      border: 1px solid #e5e7eb;
    }
    .header {
      background: linear-gradient(135deg, #6366f1, #4f46e5);
      padding: 24px 32px;
      color: #ffffff;
    }
    .header h2 {
      margin: 0;
      font-size: 20px;
      font-weight: 700;
    }
    .header p {
      margin: 4px 0 0 0;
      font-size: 13px;
      opacity: 0.9;
    }
    .content {
      padding: 32px;
      line-height: 1.6;
      font-size: 15px;
    }
    .message-box {
      background-color: #f9fafb;
      border-left: 4px solid #6366f1;
      padding: 16px 20px;
      margin: 20px 0;
      border-radius: 4px;
      font-size: 15px;
      color: #374151;
      white-space: pre-wrap;
    }
    .footer {
      background-color: #f9fafb;
      padding: 20px 32px;
      text-align: center;
      font-size: 12px;
      color: #6b7280;
      border-top: 1px solid #e5e7eb;
    }
    .footer p {
      margin: 4px 0;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h2>Obertrack Soporte</h2>
      <p>Actualización del Ticket #%d - %s</p>
    </div>
    <div class="content">
      <p>Estimado/a %s,</p>
      <p>Hemos enviado una actualización a tu consulta. A continuación puedes ver el mensaje:</p>
      
      <div class="message-box">%s</div>
      
      <p>Si tienes alguna duda o deseas agregar más información, puedes responder directamente a este correo.</p>
    </div>
    <div class="footer">
      <p>Este es un correo automático enviado por <strong>Obertrack</strong>.</p>
      <p>&copy; 2026 Obertrack. Todos los derechos reservados.</p>
    </div>
  </div>
</body>
</html>`, ticketID, ticketTitle, toName, escapedContent)

	return s.SendEmail(toEmail, toName, subject, htmlContent)
}
