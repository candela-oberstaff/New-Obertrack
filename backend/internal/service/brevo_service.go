package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// BrevoService handles email dispatch via the Brevo (Sendinblue) Transactional API.
type BrevoService struct {
	apiKey string
	apiURL string
	from   BrevoContact
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

// BrevoSendResponse es lo que devuelve Brevo al aceptar un envío.
type BrevoSendResponse struct {
	MessageID string `json:"messageId"`
}

// logAccepted deja constancia de que Brevo ACEPTÓ el correo, con su messageId.
//
// Aceptar no es entregar: Brevo responde 201 y luego descarta en silencio los
// envíos a contactos bloqueados (los que rebotaron antes, se dieron de baja o
// marcaron spam). Sin este identificador, un "no me llegó el correo" no se
// puede investigar —no hay forma de saber si salió, rebotó o se bloqueó— y era
// justo lo que pasaba: la aplicación decía "enviado" y ahí se acababa el rastro.
//
// Con el messageId se busca el envío en el panel de Brevo (Transactional >
// Logs) y se ve qué le pasó de verdad.
func logAccepted(resp *http.Response, toEmail, subject string) {
	var out BrevoSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.MessageID == "" {
		log.Printf("[brevo] aceptado para %s (%q) pero sin messageId en la respuesta", toEmail, subject)
		return
	}
	log.Printf("[brevo] aceptado para %s (%q) messageId=%s", toEmail, subject, out.MessageID)
}

// recipient arma el destinatario para Brevo, que RECHAZA la petición entera con
// "missing_parameter - name is missing in to" si el nombre va vacío.
//
// No es hipotético: hay destinatarios sin nombre por toda la base (contactos de
// WhatsApp recién creados, importaciones a medias, direcciones que se escriben a
// mano). Antes reventaba el envío; ahora se cae a la parte local de la dirección,
// que es fea pero llega. El nombre aquí es solo lo que se ve en el "Para" del
// buzón: el saludo del correo lo resuelven las variables {{nombre}}, así que
// esto no cambia el contenido.
func recipient(email, name string) BrevoContact {
	name = strings.TrimSpace(name)
	if name == "" {
		if at := strings.Index(email, "@"); at > 0 {
			name = email[:at]
		} else {
			name = email
		}
	}
	return BrevoContact{Name: name, Email: email}
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

	// Wrap HTML with Obertrack header/logo and footer if it is not already wrapped
	wrappedHTML := htmlContent
	if !strings.Contains(htmlContent, "<!-- Obertrack Logo -->") && !strings.Contains(htmlContent, "<!-- Oberstaff Logo -->") && !strings.Contains(htmlContent, "<html") {
		wrappedHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
</head>
<body style="margin: 0; padding: 0; background-color: #f6f8fa;">
	<div style="max-width: 600px; margin: 24px auto; background: #ffffff; border: 1px solid #ddd9ef; border-radius: 16px; overflow: hidden; box-shadow: 0 4px 12px rgba(6, 11, 35, 0.05); font-family: sans-serif;">
		<!-- Banner con Logo -->
		<div style="background: #ffffff; padding: 32px 24px; text-align: center; border-bottom: 1px solid #ddd9ef;">
			<img src="https://obertrack.com/logos/logo-oberstaff.png" alt="Oberstaff Logo" height="40" style="display: block; margin: 0 auto 12px auto; height: 40px; border: 0; outline: none;" />
			<!-- Oberstaff Logo -->
		</div>

		<!-- Contenido -->
		<div style="padding: 32px 24px; color: #060b23; font-size: 15px; line-height: 1.6;">
			%s
		</div>

		<!-- Footer -->
		<div style="background: #f5f2fb; padding: 24px; text-align: center; font-size: 12px; color: #8880a8; border-top: 1px solid #ddd9ef;">
			Este es un correo enviado de forma segura por la plataforma <strong>Obertrack</strong>.<br>
			&copy; 2026 Obertrack. Todos los derechos reservados.
		</div>
	</div>
</body>
</html>`, htmlContent)
	}

	payload := BrevoEmailRequest{
		Sender:      s.from,
		To:          []BrevoContact{recipient(toEmail, toName)},
		Subject:     subject,
		HTMLContent: wrappedHTML,
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

	logAccepted(resp, toEmail, subject)
	return nil
}

func (s *BrevoService) SendEmailWithAttachments(toEmail, toName, subject, htmlContent string, attachments []BrevoAttachment) error {
	if s.apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY is not configured")
	}

	payload := BrevoEmailRequest{
		Sender:      s.from,
		To:          []BrevoContact{recipient(toEmail, toName)},
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

	logAccepted(resp, toEmail, subject)
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
