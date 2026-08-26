package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// ErrEmailKindDisabled lo devuelve un envío cuyo TIPO está apagado en
// Configuración → Correos.
//
// Antes estos envíos devolvían nil, con la idea de "no romper el flujo que lo
// disparó". Para los watchers está bien —no hay nadie mirando—, pero los flujos
// que le reportan el resultado a una persona interpretaban ese nil como
// "enviado": una campaña con el interruptor cerrado respondía "enviada a 330",
// se marcaba como enviada y quedaba bloqueada para reenviar, sin haber salido
// un solo correo.
//
// Quien no le importe lo descarta con errors.Is; quien tenga a alguien delante
// lo muestra.
var ErrEmailKindDisabled = errors.New("el tipo de correo está desactivado en Configuración → Correos")

// BrevoService handles email dispatch via the Brevo (Sendinblue) Transactional API.
type BrevoService struct {
	apiKey string
	apiURL string
	from   BrevoContact
	// kindGate decide si un TIPO de correo puede salir (Configuración →
	// Correos). Se inyecta en el wiring para no acoplar Brevo al servicio de
	// ajustes; si es nil, todo sale (comportamiento histórico).
	kindGate func(kind string) bool
}

// SetKindGate cablea el interruptor por tipo de correo. Lo llama routes/deps.
func (s *BrevoService) SetKindGate(gate func(kind string) bool) { s.kindGate = gate }

// AllowsKind informa si ese tipo de correo está encendido.
func (s *BrevoService) AllowsKind(kind string) bool {
	return s.kindGate == nil || s.kindGate(kind)
}

// SendEmailKind es SendEmail respetando el interruptor del tipo. Es el camino
// que deben usar TODOS los envíos del sistema: así el panel de Configuración
// los gobierna sin tocar código.
//
// Un correo apagado devuelve ErrEmailKindDisabled: no es un fallo, pero tampoco
// un envío. Quien no tenga a nadie delante lo descarta con errors.Is.
func (s *BrevoService) SendEmailKind(kind, toEmail, toName, subject, htmlContent string) error {
	if !s.AllowsKind(kind) {
		log.Printf("[Brevo] correo %q omitido: está desactivado en Configuración → Correos", kind)
		return ErrEmailKindDisabled
	}
	return s.SendEmail(toEmail, toName, subject, htmlContent)
}

// SendEmailKindTagged es SendEmailKind estampando etiquetas en el envío. Lo usan
// las campañas para que sus eventos vuelvan del webhook identificados.
func (s *BrevoService) SendEmailKindTagged(kind, toEmail, toName, subject, htmlContent string, tags []string) error {
	if !s.AllowsKind(kind) {
		log.Printf("[Brevo] correo %q omitido: está desactivado en Configuración → Correos", kind)
		return ErrEmailKindDisabled
	}
	return s.SendEmailTagged(toEmail, toName, subject, htmlContent, tags)
}

// SendEmailKindWithAttachments es la variante con adjuntos (reporte de jornadas).
func (s *BrevoService) SendEmailKindWithAttachments(kind, toEmail, toName, subject, htmlContent string, attachments []BrevoAttachment) error {
	if !s.AllowsKind(kind) {
		log.Printf("[Brevo] correo %q omitido: está desactivado en Configuración → Correos", kind)
		return ErrEmailKindDisabled
	}
	return s.SendEmailWithAttachments(toEmail, toName, subject, htmlContent, attachments)
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
	// Tags viajan de ida en el envío y vuelven en cada evento del webhook. Son
	// el ÚNICO hilo que ata una apertura o un clic a la campaña que lo provocó:
	// ver CampaignTag más abajo.
	Tags []string `json:"tags,omitempty"`
}

// BrevoMessageVersion es una copia personalizada dentro de un envío por lotes: su
// destinatario y, si hace falta, su propio asunto y cuerpo.
type BrevoMessageVersion struct {
	To []BrevoContact `json:"to"`
	// Subject y HTMLContent sólo se mandan cuando difieren del global. La API exige
	// que el global exista igualmente: por eso el lote siempre lleva los dos.
	Subject     string `json:"subject,omitempty"`
	HTMLContent string `json:"htmlContent,omitempty"`
}

type BrevoBatchRequest struct {
	Sender          BrevoContact          `json:"sender"`
	Subject         string                `json:"subject"`
	HTMLContent     string                `json:"htmlContent"`
	MessageVersions []BrevoMessageVersion `json:"messageVersions"`
	Tags            []string              `json:"tags,omitempty"`
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
func logAccepted(resp *http.Response, que string) {
	var out BrevoSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.MessageID == "" {
		log.Printf("[brevo] aceptado: %s (sin messageId en la respuesta)", que)
		return
	}
	log.Printf("[brevo] aceptado: %s messageId=%s", que, out.MessageID)
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

// campaignTagPrefix marca las etiquetas que pone Obertrack. El prefijo evita
// confundirlas con etiquetas puestas a mano en el panel de Brevo.
const campaignTagPrefix = "obertrack-campaign-"

// CampaignTag es la etiqueta con la que sale cada correo de una campaña.
//
// Las campañas de Obertrack se envían por la API TRANSACCIONAL, un correo por
// persona: Brevo no sabe que existe la campaña, ve 330 envíos sueltos. Por eso
// sus eventos (`opened`, `click`, `hard_bounce`) NO traen `campaign_id` —ese
// campo solo existe en las campañas de marketing de Brevo— y sin esta etiqueta
// no hay forma de saber a qué campaña pertenece una apertura. Era exactamente
// el motivo por el que el panel mostraba 0.0% en todo.
func CampaignTag(id uint) string {
	return fmt.Sprintf("%s%d", campaignTagPrefix, id)
}

// CampaignIDFromTags recupera el ID de campaña de las etiquetas de un evento.
// Devuelve 0 si ninguna es nuestra (correo suelto, prueba, notificación).
func CampaignIDFromTags(tags []string) uint {
	for _, t := range tags {
		rest := strings.TrimPrefix(strings.TrimSpace(t), campaignTagPrefix)
		if rest == t {
			continue
		}
		if id, err := strconv.ParseUint(rest, 10, 32); err == nil {
			return uint(id)
		}
	}
	return 0
}

// ParseBrevoTags normaliza las etiquetas tal como llegan en el webhook, que no
// tiene un formato único: según el evento, Brevo manda `tags` como arreglo
// (["obertrack-campaign-15"]), como cadena con JSON dentro, o solo el campo
// `tag` con una sola etiqueta. Se aceptan las tres para que ningún evento se
// pierda por la forma en que vino.
func ParseBrevoTags(raw json.RawMessage, single string) []string {
	var tags []string

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tags); err != nil {
			var asString string
			if json.Unmarshal(raw, &asString) == nil && asString != "" {
				if json.Unmarshal([]byte(asString), &tags) != nil {
					tags = []string{asString}
				}
			}
		}
	}

	if single != "" {
		tags = append(tags, single)
	}
	return tags
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
	return s.SendEmailTagged(toEmail, toName, subject, htmlContent, nil)
}

// SendEmailTagged es SendEmail adjuntando etiquetas al envío. Las etiquetas no
// se ven en el buzón: viajan con el correo y Brevo las devuelve en cada evento
// del webhook, que es como se atribuye una apertura a su campaña.
func (s *BrevoService) SendEmailTagged(toEmail, toName, subject, htmlContent string, tags []string) error {
	if s.apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY is not configured")
	}

	payload := BrevoEmailRequest{
		Sender:      s.from,
		To:          []BrevoContact{recipient(toEmail, toName)},
		Subject:     subject,
		HTMLContent: wrapBrevoHTML(htmlContent),
		Tags:        tags,
	}

	return s.post(payload, toEmail+" ("+subject+")")
}

// wrapBrevoHTML mete el contenido en la plantilla de marca, salvo que ya venga
// envuelto. Se extrajo de SendEmailTagged para que el envío por lotes vista cada
// versión exactamente igual que el envío individual: dos envolturas distintas es como
// acaban divergiendo el correo de prueba y el de verdad.
func wrapBrevoHTML(htmlContent string) string {
	if strings.Contains(htmlContent, "<!-- Obertrack Logo -->") ||
		strings.Contains(htmlContent, "<!-- Oberstaff Logo -->") ||
		strings.Contains(htmlContent, "<html") {
		return htmlContent
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
</head>
<body style="margin: 0; padding: 0; background-color: #f6f8fa;">
	<div style="max-width: 600px; margin: 24px auto; background: #ffffff; border: 1px solid #ddd9ef; border-radius: 16px; overflow: hidden; box-shadow: 0 4px 12px rgba(6, 11, 35, 0.05); font-family: sans-serif;">
		<!-- Banner con Logo. Dos cosas que hay que respetar aquí:
		     1) El degradado de marca es indispensable: el logo es la versión
		        BLANCA y sobre el fondo blanco anterior quedaba invisible, dejando
		        una franja vacía en la cabecera de casi todos los correos.
		     2) El archivo es logo-oberstaff.png (921x225) y NO Horizontal_Blanco.png
		        (5000x1058): el proxy de imágenes de Gmail no sirve la versión
		        grande y el correo llega con la imagen rota. Cualquier logo que se
		        ponga aquí debe venir ya redimensionado para correo (~600 px). -->
		<div style="background: linear-gradient(135deg, #060b23 0%%, #cc33cc 100%%); padding: 32px 24px; text-align: center;">
			<img src="https://obertrack.com/logos/logo-oberstaff.png" alt="Oberstaff" height="40" style="display: block; margin: 0 auto; height: 40px; max-width: 260px; border: 0; outline: none;" />
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

// post manda un payload ya armado. Lo comparten el envío individual y el de lotes.
// `que` describe el envío para el log (a quién, o cuántos en un lote).
func (s *BrevoService) post(payload any, que string) error {
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

	logAccepted(resp, que)
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

	logAccepted(resp, toEmail+" ("+subject+")")
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

// BatchRecipient es una persona dentro de un envío masivo, con el asunto y el cuerpo
// ya personalizados si la plantilla llevaba variables.
type BatchRecipient struct {
	Email   string
	Name    string
	Subject string
	HTML    string
}

// Tamaño de cada petición. La API admite hasta 1000 versiones, pero cuando cada una
// lleva su propio HTML el cuerpo de la petición crece con la plantilla entera por
// persona: a 60 KB de plantilla, 1000 versiones serían 60 MB. Con contenido idéntico
// para todos ese problema no existe y se puede ir mucho más arriba.
const (
	brevoBatchSizePersonalized = 50
	brevoBatchSizeUniform      = 500
)

// SendBatchKind manda el mismo correo a mucha gente en POCAS peticiones, una copia
// separada por persona.
//
// Antes se mandaba de una en una: quinientos destinatarios eran quinientas llamadas
// seguidas, con quien pulsó "enviar" esperando delante de la pantalla. No consume
// menos créditos —Brevo cobra por correo entregado, no por llamada— pero convierte
// minutos en segundos y deja de exponerse al límite de peticiones por segundo.
//
// Cada persona va en su propia "versión", nunca en un mismo `to`: así nadie ve las
// direcciones de los demás, igual que con el envío individual.
//
// Devuelve cuántos se aceptaron y los errores por destinatario. Un lote que falla se
// cuenta como fallo de TODOS los suyos: la API acepta o rechaza la petición entera, y
// dar por enviados a los de un lote rechazado sería mentir en el informe.
func (s *BrevoService) SendBatchKind(kind string, recipients []BatchRecipient, subject, htmlContent string, tags []string) (int, []string) {
	if len(recipients) == 0 {
		return 0, nil
	}
	if !s.AllowsKind(kind) {
		return 0, nil
	}
	if s.apiKey == "" {
		return 0, []string{"BREVO_API_KEY is not configured"}
	}

	// ¿Alguien lleva contenido propio? Eso decide el tamaño del lote.
	personalizado := false
	for _, r := range recipients {
		if (r.HTML != "" && r.HTML != htmlContent) || (r.Subject != "" && r.Subject != subject) {
			personalizado = true
			break
		}
	}
	tam := brevoBatchSizeUniform
	if personalizado {
		tam = brevoBatchSizePersonalized
	}

	enviados := 0
	var fallos []string
	for inicio := 0; inicio < len(recipients); inicio += tam {
		fin := inicio + tam
		if fin > len(recipients) {
			fin = len(recipients)
		}
		lote := recipients[inicio:fin]

		versiones := make([]BrevoMessageVersion, 0, len(lote))
		for _, r := range lote {
			v := BrevoMessageVersion{To: []BrevoContact{recipient(r.Email, r.Name)}}
			if r.Subject != "" && r.Subject != subject {
				v.Subject = r.Subject
			}
			if r.HTML != "" && r.HTML != htmlContent {
				v.HTMLContent = wrapBrevoHTML(r.HTML)
			}
			versiones = append(versiones, v)
		}

		payload := BrevoBatchRequest{
			Sender:  s.from,
			Subject: subject,
			// El global es obligatorio aunque cada versión traiga el suyo: sin él la
			// API rechaza el htmlContent por versión.
			HTMLContent:     wrapBrevoHTML(htmlContent),
			MessageVersions: versiones,
			Tags:            tags,
		}
		if err := s.post(payload, fmt.Sprintf("lote de %d (%q)", len(lote), subject)); err != nil {
			for _, r := range lote {
				fallos = append(fallos, fmt.Sprintf("%s: %s", r.Email, err.Error()))
			}
			continue
		}
		enviados += len(lote)
	}
	return enviados, fallos
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
