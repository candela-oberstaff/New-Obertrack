package service

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// supportDigestWindow es la ventana del digest de correo: el PRIMER ticket
// tras un periodo tranquilo se avisa al instante; los que lleguen dentro de la
// ventana se agrupan en UN solo correo al cerrarse. Antes cada solicitud
// generaba un correo a TODOS los agentes CS+IT — con volumen real, spam puro.
const supportDigestWindow = 15 * time.Minute

// El encendido de este correo ya NO vive en una constante: se gobierna desde
// Configuración → Correos (clave EmailKindSupportTicket) y lo aplica
// BrevoService.SendEmailKind, sin necesidad de redeploy.

type SupportNotifier struct {
	brevoSvc     *BrevoService
	userRepo     repository.UserRepository
	supportEmail string

	// Estado del digest. En memoria a propósito: si el proceso se reinicia se
	// pierde a lo sumo un lote de correos de cortesía — la campanita y el
	// tablero siguen siendo el aviso principal.
	mu       sync.Mutex
	pending  []SupportTicketInfo
	timer    *time.Timer
	lastSent time.Time
}

func NewSupportNotifier(brevoSvc *BrevoService, userRepo repository.UserRepository, supportEmail string) *SupportNotifier {
	return &SupportNotifier{brevoSvc: brevoSvc, userRepo: userRepo, supportEmail: strings.TrimSpace(supportEmail)}
}

type SupportTicketInfo struct {
	Type        string
	Requester   string
	Company     string
	Subject     string
	Description string
	Reason      string
	Link        string
}

func (n *SupportNotifier) recipients() []BrevoContact {
	seen := make(map[string]bool)
	out := make([]BrevoContact, 0, 8)

	add := func(email, name string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		out = append(out, BrevoContact{Name: name, Email: email})
	}

	if n.userRepo != nil {
		for _, role := range []models.UserType{models.UserTypeCustomerSuccess, models.UserTypeITAnalyst} {
			users, _, err := n.userRepo.GetAll(string(role), "", "", 0, 0, 1000)
			if err != nil {
				log.Printf("[SupportNotifier] no se pudo listar agentes (%s): %v", role, err)
				continue
			}
			for _, u := range users {
				if u.IsActive {
					add(u.Email, u.Name)
				}
			}
		}
	}
	add(n.supportEmail, "Soporte")
	return out
}

// Notify encola el aviso en el digest. Tras un periodo tranquilo el primer
// ticket sale al instante (la urgencia no espera 15 minutos); los siguientes
// dentro de la ventana se agrupan en un solo correo.
func (n *SupportNotifier) Notify(info SupportTicketInfo) {
	if n == nil || n.brevoSvc == nil {
		return
	}
	// Si el tipo está apagado en Configuración → Correos, no se arma el digest:
	// así no se acumula un lote que nadie va a recibir.
	if !n.brevoSvc.AllowsKind(EmailKindSupportTicket) {
		return
	}
	n.mu.Lock()
	n.pending = append(n.pending, info)

	if time.Since(n.lastSent) >= supportDigestWindow {
		batch := n.pending
		n.pending = nil
		n.lastSent = time.Now()
		n.mu.Unlock()
		go n.send(batch)
		return
	}

	// Dentro de la ventana: programa (una sola vez) el envío del lote al
	// cerrarse. Los tickets que sigan llegando se suman al mismo lote.
	if n.timer == nil {
		delay := supportDigestWindow - time.Since(n.lastSent)
		n.timer = time.AfterFunc(delay, func() {
			n.mu.Lock()
			batch := n.pending
			n.pending = nil
			n.timer = nil
			if len(batch) > 0 {
				n.lastSent = time.Now()
			}
			n.mu.Unlock()
			if len(batch) > 0 {
				n.send(batch)
			}
		})
	}
	n.mu.Unlock()
}

// send entrega un lote (1..N tickets) a todos los agentes. Con un solo ticket
// el correo es el de siempre; con varios, un digest con la lista.
func (n *SupportNotifier) send(batch []SupportTicketInfo) {
	recipients := n.recipients()
	if len(recipients) == 0 {
		return
	}
	subject := "🎫 Nuevo ticket de soporte"
	html := ""
	if len(batch) == 1 {
		html = n.buildHTML(batch[0])
	} else {
		subject = fmt.Sprintf("🎫 %d tickets de soporte nuevos", len(batch))
		html = n.buildDigestHTML(batch)
	}
	for _, r := range recipients {
		if err := n.brevoSvc.SendEmailKind(EmailKindSupportTicket, r.Email, r.Name, subject, html); err != nil {
			log.Printf("[SupportNotifier] no se pudo enviar a %s: %v", r.Email, err)
		}
	}
}

// buildDigestHTML arma el correo de un lote: una fila por ticket con lo
// esencial y un solo botón al tablero.
func (n *SupportNotifier) buildDigestHTML(batch []SupportTicketInfo) string {
	link := "/tickets/soporte"
	if base := frontendBaseURL(); base != "" {
		link = base + link
	}

	var rows strings.Builder
	for _, info := range batch {
		who := strings.TrimSpace(info.Requester)
		if info.Company != "" {
			who = fmt.Sprintf("%s (%s)", who, info.Company)
		}
		subject := strings.TrimSpace(info.Subject)
		if subject == "" {
			subject = strings.TrimSpace(info.Type)
		}
		rows.WriteString(fmt.Sprintf(
			`<tr><td style="padding:8px 12px 8px 0;color:#060b23;font-weight:600;vertical-align:top;white-space:nowrap;">%s</td><td style="padding:8px 0;color:#060b23;">%s<div style="color:#8880a8;font-size:12px;">%s</div></td></tr>`,
			who, subject, strings.TrimSpace(info.Type)))
	}

	return fmt.Sprintf(`<h2 style="margin:0 0 16px 0;color:#060b23;">🎫 %d tickets de soporte nuevos</h2>
<p style="margin:0 0 16px 0;">Llegaron varias solicitudes en los últimos minutos:</p>
<table style="width:100%%;border-collapse:collapse;font-size:14px;">%s</table>
<div style="margin-top:24px;">
	<a href="%s" style="display:inline-block;background:#cc33cc;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:8px;font-weight:600;">Abrir el tablero de soporte</a>
</div>`, len(batch), rows.String(), link)
}

// frontendBaseURL resuelve la base pública del frontend para armar enlaces.
func frontendBaseURL() string {
	base := os.Getenv("FRONTEND_URL")
	if base == "" {
		base = os.Getenv("SERVICE_URL_FRONTEND")
	}
	return strings.TrimRight(base, "/")
}

func (n *SupportNotifier) buildHTML(info SupportTicketInfo) string {
	link := info.Link
	if link == "" {
		link = "/tickets"
	}
	if !strings.HasPrefix(link, "http") {
		base := os.Getenv("FRONTEND_URL")
		if base == "" {
			base = os.Getenv("SERVICE_URL_FRONTEND")
		}
		if base != "" {
			link = strings.TrimRight(base, "/") + link
		}
	}

	var rows strings.Builder
	row := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		rows.WriteString(fmt.Sprintf(
			`<tr><td style="padding:6px 12px 6px 0;color:#8880a8;font-weight:600;white-space:nowrap;vertical-align:top;">%s</td><td style="padding:6px 0;color:#060b23;">%s</td></tr>`,
			label, value))
	}
	row("Solicitante", info.Requester)
	row("Empresa", info.Company)
	row("Tipo", info.Type)
	row("Asunto", info.Subject)
	row("Descripción", info.Description)
	row("Motivo", info.Reason)

	return fmt.Sprintf(`<h2 style="margin:0 0 16px 0;color:#060b23;">🎫 Nuevo ticket de soporte</h2>
<p style="margin:0 0 16px 0;">Se ha creado un nuevo ticket de soporte. Estos son los detalles:</p>
<table style="width:100%%;border-collapse:collapse;font-size:14px;">%s</table>
<div style="margin-top:24px;">
	<a href="%s" style="display:inline-block;background:#cc33cc;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:8px;font-weight:600;">Abrir en Obertrack</a>
</div>`, rows.String(), link)
}
