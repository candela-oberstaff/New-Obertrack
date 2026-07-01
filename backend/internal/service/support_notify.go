package service

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// SupportNotifier envía un correo (best-effort) al equipo de soporte cada vez que
// se crea un ticket. Los destinatarios son los agentes activos de Customer Success
// y Analista de IT (soporte técnico), más, si está configurado, el buzón fijo SUPPORT_EMAIL.
type SupportNotifier struct {
	brevoSvc     *BrevoService
	userRepo     repository.UserRepository
	supportEmail string
}

func NewSupportNotifier(brevoSvc *BrevoService, userRepo repository.UserRepository, supportEmail string) *SupportNotifier {
	return &SupportNotifier{brevoSvc: brevoSvc, userRepo: userRepo, supportEmail: strings.TrimSpace(supportEmail)}
}

// SupportTicketInfo describe el ticket recién creado para el cuerpo del correo.
type SupportTicketInfo struct {
	Type        string
	Requester   string
	Company     string
	Subject     string
	Description string
	Reason      string
	Link        string
}

// recipients resuelve la lista de correos (agentes CS activos + SUPPORT_EMAIL),
// en minúsculas, sin blancos y sin duplicados.
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
		// Equipo de soporte: Customer Success + Analista de IT (soporte técnico).
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

// Notify envía el correo en segundo plano. Nunca falla la creación del ticket:
// los errores se registran y se descartan.
func (n *SupportNotifier) Notify(info SupportTicketInfo) {
	if n == nil || n.brevoSvc == nil {
		return
	}
	go func() {
		recipients := n.recipients()
		if len(recipients) == 0 {
			return
		}
		subject := "🎫 Nuevo ticket de soporte"
		html := n.buildHTML(info)
		for _, r := range recipients {
			if err := n.brevoSvc.SendEmail(r.Email, r.Name, subject, html); err != nil {
				log.Printf("[SupportNotifier] no se pudo enviar a %s: %v", r.Email, err)
			}
		}
	}()
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
