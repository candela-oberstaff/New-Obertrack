package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Claves del catálogo de correos. Son el contrato entre el emisor (que llama a
// SendEmailKind con su clave) y el panel de Configuración → Correos.
// NOTA: existió EmailKindChatDigest ("chat_digest"), el correo de respaldo
// "tienes mensajes sin leer". Se retiró por completo por decisión del equipo
// (2026-08-27): llevaba semanas generando envíos fantasma y consumiendo el
// plan de Brevo. Su tabla de registro y su interruptor se limpian en la
// migración 202608271200_drop_chat_digest.
const (
	EmailKindInactivityAlert    = "inactivity_alert"
	EmailKindStaleCompany       = "stale_company"
	EmailKindWorkHourReport     = "workhour_report"
	EmailKindSupportTicket      = "support_ticket"
	EmailKindPasswordReset      = "password_reset"
	EmailKindAccountSetup       = "account_setup"
	EmailKindAccessCredentials  = "access_credentials"
	EmailKindInductionInvite    = "induction_invite"
	EmailKindTestimonialRequest = "testimonial_request"
	EmailKindIncidentBroadcast  = "incident_broadcast"
	EmailKindSurveyInvite       = "survey_invite"
	EmailKindTicketReply        = "ticket_reply"
	EmailKindManualComposer     = "manual_composer"
	EmailKindCampaign           = "campaign"
	// EmailKindWorkflow se enviaba con la cadena "workflow" escrita a mano en el
	// sitio del envío. Como no estaba en el catálogo, no salía en la pantalla, no se
	// podía crear su fila —SetEnabled rechaza claves desconocidas— y una clave sin
	// fila se considera ENCENDIDA: era el único correo del sistema imposible de
	// apagar, y encima automático.
	EmailKindWorkflow = "workflow"
	// EmailKindObervoiceCredentials envía al usuario sus datos de telefonía SIP.
	EmailKindObervoiceCredentials = "obervoice_credentials"
)

// EmailCategory agrupa los correos en el panel.
const (
	EmailCategoryAutomatic = "automatic" // los dispara un watcher, sin intervención
	EmailCategoryEvent     = "event"     // los dispara una acción del sistema
	EmailCategoryManual    = "manual"    // los dispara una persona
)

// EmailType describe un correo del sistema para el panel de Configuración.
type EmailType struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Trigger explica CUÁNDO sale (lo que el equipo necesita saber para decidir).
	Trigger   string `json:"trigger"`
	Recipient string `json:"recipient"`
	Category  string `json:"category"`
	// Essential marca los correos que dejan gente fuera de la plataforma si se
	// apagan (recuperar/crear contraseña). Se pueden apagar igual, pero la
	// interfaz avisa antes.
	Essential bool `json:"essential"`
	// ManagedElsewhere: su encendido vive en otra parte del panel (el reporte
	// de jornadas se gobierna con "Envío automático de reportes"), así que la
	// fila se muestra sin toggle propio para no tener dos mandos.
	ManagedElsewhere string                  `json:"managed_elsewhere,omitempty"`
	Enabled          bool                    `json:"enabled"`
	Frequency        string                  `json:"frequency"`
	DayOfMonth       int                     `json:"day_of_month"`
	Weekday          int                     `json:"weekday"`
	Hour             int                     `json:"hour"`
	Minute           int                     `json:"minute"`
	Timezone         string                  `json:"timezone"`
	Recipients       []models.EmailRecipient `json:"recipients"`
}

// emailCatalog es la lista COMPLETA de correos que salen de Obertrack. Al
// agregar un envío nuevo al sistema, agrégalo aquí y usa SendEmailKind con su
// clave: así aparece en el panel y respeta su interruptor.
var emailCatalog = []EmailType{
	{
		Key: EmailKindWorkHourReport, Category: EmailCategoryAutomatic,
		Name:             "Reporte de jornadas",
		Description:      "Resumen de actividades del período con PDF y Excel adjuntos.",
		Trigger:          "Según la programación de arriba (diaria, semanal o mensual). En la frecuencia mensual también cierra el mes aprobando las jornadas pendientes.",
		Recipient:        "Cada empresa",
		ManagedElsewhere: "Se enciende y programa en «Envío automático de reportes».",
	},
	{
		Key: EmailKindWorkflow, Category: EmailCategoryAutomatic,
		Name:        "Correos de automatizaciones",
		Description: "El correo que manda una regla del tablero cuando su receta lo incluye.",
		Trigger:     "Cuando se cumple el disparador de una automatización con acción de correo.",
		Recipient:   "Quien indique la regla: responsables, managers o el líder del proyecto.",
	},
	{
		Key: EmailKindInactivityAlert, Category: EmailCategoryAutomatic,
		Name:        "Alerta de inactividad",
		Description: "Profesionales que llevan días sin registrar horas.",
		Trigger:     "Chequeo diario: 2+ días sin registrar. No repite la misma persona en 7 días.",
		Recipient:   "Equipo de Customer Success",
	},
	{
		Key: EmailKindStaleCompany, Category: EmailCategoryAutomatic,
		Name:        "Empresa sin usar la app",
		Description: "Empresas donde nadie ha abierto la app en dos semanas.",
		Trigger:     "Chequeo diario: 14 días sin que nadie de la empresa entre. No repite la misma empresa en 14 días.",
		Recipient:   "Equipo de Customer Success",
	},
	{
		Key: EmailKindSupportTicket, Category: EmailCategoryEvent,
		Name:        "Nuevo ticket de soporte",
		Description: "Aviso de solicitudes nuevas, agrupadas en un digest.",
		Trigger:     "Al crearse una solicitud. La primera sale al instante y las siguientes 15 min se agrupan en un solo correo.",
		Recipient:   "Customer Success y Analistas de IT",
	},
	{
		Key: EmailKindPasswordReset, Category: EmailCategoryEvent, Essential: true,
		Name:        "Recuperar contraseña",
		Description: "Enlace para restablecer la contraseña.",
		Trigger:     "Cuando alguien usa «Olvidé mi contraseña».",
		Recipient:   "El usuario que lo solicita",
	},
	{
		Key: EmailKindAccountSetup, Category: EmailCategoryEvent, Essential: true,
		Name:        "Crea tu contraseña (alta)",
		Description: "Enlace de alta para estrenar la cuenta.",
		Trigger:     "Al entregar el acceso desde el panel de Usuarios.",
		Recipient:   "El usuario nuevo",
	},
	{
		Key: EmailKindAccessCredentials, Category: EmailCategoryEvent,
		Name:        "Datos de acceso",
		Description: "Usuario y contraseña temporal.",
		Trigger:     "Al entregar credenciales desde el panel de Usuarios.",
		Recipient:   "El usuario",
	},
	{
		Key: EmailKindInductionInvite, Category: EmailCategoryEvent,
		Name:        "Invitación a inducción",
		Description: "Invita a completar el proceso de inducción.",
		Trigger:     "Al asignar una inducción a alguien.",
		Recipient:   "El profesional",
	},
	{
		Key: EmailKindTestimonialRequest, Category: EmailCategoryEvent,
		Name:        "Solicitud de testimonio",
		Description: "Invita a escribir y firmar un testimonio.",
		Trigger:     "Al pedir un testimonio desde el panel de Testimonios.",
		Recipient:   "El profesional o la empresa",
	},
	{
		Key: EmailKindIncidentBroadcast, Category: EmailCategoryEvent,
		Name:        "Broadcast de incidente",
		Description: "Comunicado masivo a los afectados por un incidente.",
		Trigger:     "Al pulsar «Broadcast a afectados» en un incidente.",
		Recipient:   "Profesionales del incidente",
	},
	{
		Key: EmailKindSurveyInvite, Category: EmailCategoryEvent,
		Name:        "Invitación a encuesta",
		Description: "Invita a responder una encuesta.",
		Trigger:     "Al publicar una encuesta.",
		Recipient:   "Los destinatarios de la encuesta",
	},
	{
		Key: EmailKindTicketReply, Category: EmailCategoryEvent,
		Name:        "Respuesta de ticket por correo",
		Description: "La respuesta del agente al contacto, por el canal email.",
		Trigger:     "Al responder un ticket cuyo canal es correo.",
		Recipient:   "El contacto del ticket",
	},
	{
		Key: EmailKindManualComposer, Category: EmailCategoryManual,
		Name:        "Correos del redactor",
		Description: "Envíos uno a uno o masivos desde fichas, Mapa e Incidentes.",
		Trigger:     "Cuando una persona del equipo lo envía a mano.",
		Recipient:   "Quien se elija",
	},
	{
		Key: EmailKindCampaign, Category: EmailCategoryManual,
		Name:        "Campañas de Email Marketing",
		Description: "Campañas del constructor de correos (incluye las programadas).",
		Trigger:     "Al enviar o programar una campaña.",
		Recipient:   "La lista de la campaña",
	},
	{
		Key:         EmailKindObervoiceCredentials,
		Category:    EmailCategoryEvent,
		Name:        "Credenciales Obervoice",
		Description: "Datos de telefonía SIP enviados al usuario desde el panel de Obervoice.",
		Trigger:     "Al hacer clic en \"Enviar credenciales\" desde el panel de Obervoice.",
		Recipient:   "El usuario cuyos datos se configuraron.",
	},
}

// EmailSettingsService resuelve si un tipo de correo está activo y expone el
// catálogo al panel. La consulta ocurre en CADA envío, así que el estado vive
// en un caché en memoria que se invalida al guardar.
type EmailSettingsService struct {
	repo     repository.EmailSettingRepository
	userRepo repository.UserRepository
	brevo    *BrevoService

	mu      sync.RWMutex
	cache   map[string]bool
	loaded  bool
	expires time.Time
}

// emailSettingsTTL refresca el caché aunque la escritura venga de otra
// instancia del backend (despliegues con más de una réplica).
const emailSettingsTTL = 60 * time.Second

func NewEmailSettingsService(repo repository.EmailSettingRepository, userRepo repository.UserRepository, brevo *BrevoService) *EmailSettingsService {
	return &EmailSettingsService{repo: repo, userRepo: userRepo, cache: map[string]bool{}, brevo: brevo}
}

// Enabled dice si un tipo de correo puede salir.
func (s *EmailSettingsService) Enabled(kind string) bool {
	s.mu.RLock()
	fresh := s.loaded && time.Now().Before(s.expires)
	if fresh {
		enabled, ok := s.cache[kind]
		s.mu.RUnlock()
		if !ok {
			return true
		}
		return enabled
	}
	s.mu.RUnlock()

	rows, err := s.repo.List()
	if err != nil {
		allow := isEssentialEmailKind(kind)
		log.Printf(
			"[Correos] no se pudieron leer los interruptores (%v): %q se %s por ser %s",
			err, kind,
			map[bool]string{true: "DEJA PASAR", false: "FRENA"}[allow],
			map[bool]string{true: "esencial", false: "no esencial"}[allow],
		)
		return allow
	}
	next := make(map[string]bool, len(rows))
	for _, r := range rows {
		next[r.Key] = r.Enabled
	}
	s.mu.Lock()
	s.cache, s.loaded, s.expires = next, true, time.Now().Add(emailSettingsTTL)
	s.mu.Unlock()

	if enabled, ok := next[kind]; ok {
		return enabled
	}
	return true
}

func defaultFrequency(key string) string {
	if key == EmailKindWorkHourReport {
		return "mensual"
	}
	return "diaria"
}

func (s *EmailSettingsService) getDefaultRecipients(key string) []models.EmailRecipient {
	if s.userRepo != nil {
		switch key {
		case EmailKindInactivityAlert, EmailKindStaleCompany:
			users, err := s.userRepo.ListActiveByTypes([]models.UserType{
				models.UserTypeCustomerSuccess,
				models.UserTypeSuperadmin,
			})
			if err == nil && len(users) > 0 {
				seen := map[string]bool{}
				var out []models.EmailRecipient
				for _, u := range users {
					if u.IsSystem || strings.TrimSpace(u.Email) == "" {
						continue
					}
					email := strings.ToLower(strings.TrimSpace(u.Email))
					if !seen[email] {
						seen[email] = true
						out = append(out, models.EmailRecipient{
							Email:   email,
							Name:    u.Name,
							Enabled: true,
						})
					}
				}
				if len(out) > 0 {
					return out
				}
			}

		case EmailKindWorkflow:
			users, err := s.userRepo.ListActiveByTypes([]models.UserType{
				models.UserTypeCustomerSuccess,
				models.UserTypeSuperadmin,
				models.UserTypeProfessional,
			})
			if err == nil && len(users) > 0 {
				seen := map[string]bool{}
				var out []models.EmailRecipient
				for _, u := range users {
					if u.IsSystem || strings.TrimSpace(u.Email) == "" {
						continue
					}
					if u.IsManager || u.IsSupervisor || u.UserType == models.UserTypeCustomerSuccess || u.UserType == models.UserTypeSuperadmin {
						email := strings.ToLower(strings.TrimSpace(u.Email))
						if !seen[email] {
							seen[email] = true
							out = append(out, models.EmailRecipient{
								Email:   email,
								Name:    u.Name,
								Enabled: true,
							})
						}
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}

	return []models.EmailRecipient{
		{Email: "lorena@oberstaff.com", Name: "Lorena Moujalli", Enabled: true},
	}
}

// List devuelve el catálogo con el estado actual de cada correo.
func (s *EmailSettingsService) List() []EmailType {
	saved := map[string]models.EmailSetting{}
	if rows, err := s.repo.List(); err == nil {
		for _, r := range rows {
			saved[r.Key] = r
		}
	}
	out := make([]EmailType, 0, len(emailCatalog))
	for _, t := range emailCatalog {
		if r, ok := saved[t.Key]; ok {
			t.Enabled = r.Enabled
			if r.Frequency != "" {
				t.Frequency = r.Frequency
			} else {
				t.Frequency = defaultFrequency(t.Key)
			}
			if r.DayOfMonth > 0 {
				t.DayOfMonth = r.DayOfMonth
			} else {
				t.DayOfMonth = 1
			}
			if r.Weekday > 0 {
				t.Weekday = r.Weekday
			} else {
				t.Weekday = 1
			}
			t.Hour = r.Hour
			t.Minute = r.Minute
			if r.Timezone != "" {
				t.Timezone = r.Timezone
			} else {
				t.Timezone = "America/Santiago"
			}
			var recs []models.EmailRecipient
			if r.Recipients != "" {
				_ = json.Unmarshal([]byte(r.Recipients), &recs)
			}
			if len(recs) == 0 || (len(recs) == 1 && (recs[0].Email == "cs@oberstaff.com" || recs[0].Email == "responsables@oberstaff.com" || recs[0].Email == "soporte@oberstaff.com" || recs[0].Email == "reportes@empresa.com")) {
				recs = s.getDefaultRecipients(t.Key)
			}
			t.Recipients = recs
		} else {
			t.Enabled = true
			t.Frequency = defaultFrequency(t.Key)
			t.DayOfMonth = 1
			t.Weekday = 1
			t.Hour = 8
			t.Minute = 0
			t.Timezone = "America/Santiago"
			t.Recipients = s.getDefaultRecipients(t.Key)
		}
		out = append(out, t)
	}
	return out
}

type UpdateEmailSettingReq struct {
	Enabled    *bool                   `json:"enabled"`
	Frequency  *string                 `json:"frequency"`
	DayOfMonth *int                    `json:"day_of_month"`
	Weekday    *int                    `json:"weekday"`
	Hour       *int                    `json:"hour"`
	Minute     *int                    `json:"minute"`
	Timezone   *string                 `json:"timezone"`
	Recipients []models.EmailRecipient `json:"recipients"`
}

// UpdateSetting guarda los ajustes (interruptor, fecha, destinatarios) de un tipo y refresca el caché.
func (s *EmailSettingsService) UpdateSetting(kind string, req UpdateEmailSettingReq, userID uint) error {
	if !isKnownEmailKind(kind) {
		return fmt.Errorf("tipo de correo desconocido: %s", kind)
	}

	current, err := s.repo.Get(kind)
	setting := models.EmailSetting{
		Key:        kind,
		Enabled:    true,
		Frequency:  defaultFrequency(kind),
		DayOfMonth: 1,
		Weekday:    1,
		Hour:       8,
		Minute:     0,
		Timezone:   "America/Santiago",
		UpdatedBy:  userID,
		UpdatedAt:  time.Now(),
	}
	if err == nil && current != nil {
		setting = *current
		setting.UpdatedBy = userID
		setting.UpdatedAt = time.Now()
	}

	if req.Enabled != nil {
		setting.Enabled = *req.Enabled
	}
	if req.Frequency != nil {
		setting.Frequency = *req.Frequency
	}
	if req.DayOfMonth != nil {
		setting.DayOfMonth = *req.DayOfMonth
	}
	if req.Weekday != nil {
		setting.Weekday = *req.Weekday
	}
	if req.Hour != nil {
		setting.Hour = *req.Hour
	}
	if req.Minute != nil {
		setting.Minute = *req.Minute
	}
	if req.Timezone != nil {
		setting.Timezone = *req.Timezone
	}
	if req.Recipients != nil {
		bytes, _ := json.Marshal(req.Recipients)
		setting.Recipients = string(bytes)
	}

	if err := s.repo.Upsert(&setting); err != nil {
		return err
	}

	s.mu.Lock()
	s.loaded = false
	s.mu.Unlock()
	return nil
}

// SetEnabled guarda el interruptor de un tipo y refresca el caché.
func (s *EmailSettingsService) SetEnabled(kind string, enabled bool, userID uint) error {
	return s.UpdateSetting(kind, UpdateEmailSettingReq{Enabled: &enabled}, userID)
}

// NOTA: no hay un SetAll (apagar/encender todo de una vez) a propósito. Existió
// y se quitó: un único clic capaz de tumbar TODOS los correos —incluidos los de
// recuperar y crear contraseña, que dejan a la gente sin poder entrar— es un
// riesgo que no compensa la comodidad. El apagado se hace tipo por tipo.

// isEssentialEmailKind marca los correos sin los cuales alguien queda fuera de
// la plataforma (crear y recuperar contraseña). Son los únicos que se dejan
// pasar cuando no se puede consultar el estado de los interruptores.
func isEssentialEmailKind(kind string) bool {
	for _, t := range emailCatalog {
		if t.Key == kind {
			return t.Essential
		}
	}
	// Un tipo desconocido no llega aquí desde el sistema (el catálogo es el
	// contrato), así que ante la duda no se envía.
	return false
}

func isKnownEmailKind(kind string) bool {
	for _, t := range emailCatalog {
		if t.Key == kind {
			return true
		}
	}
	return false
}

// SendTest manda una MUESTRA del correo indicado al destinatario dado, para
// revisar el formato sin esperar a que ocurra el disparador real. Ignora el
// interruptor a propósito: se prueba también un correo apagado antes de
// encenderlo. El contenido lleva datos de ejemplo y un aviso de prueba.
func (s *EmailSettingsService) SendTest(kind, toEmail, toName string) error {
	if !isKnownEmailKind(kind) {
		return fmt.Errorf("tipo de correo desconocido: %s", kind)
	}
	if s.brevo == nil {
		return fmt.Errorf("el envío de correo no está configurado")
	}
	if strings.TrimSpace(toEmail) == "" {
		return fmt.Errorf("hace falta un correo de destino")
	}
	if toName == "" {
		toName = "Equipo Obertrack"
	}

	subject, body := sampleEmail(kind, toName)
	notice := `<div style="background:#fef9c3;border:1px solid #fde047;border-radius:10px;padding:12px 16px;margin-bottom:20px;font-size:13px;color:#854d0e;">
		<strong>Correo de prueba.</strong> Es una muestra con datos de ejemplo para revisar el formato; no corresponde a actividad real.
	</div>`

	return s.brevo.SendEmail(toEmail, toName, "[Prueba] "+subject, notice+body)
}

// GetPreview devuelve el asunto y cuerpo HTML de la muestra del correo para la vista previa.
func (s *EmailSettingsService) GetPreview(kind, toName string) (string, string, error) {
	if !isKnownEmailKind(kind) {
		return "", "", fmt.Errorf("tipo de correo desconocido: %s", kind)
	}
	if toName == "" {
		toName = "Lorena Moujalli"
	}
	subject, body := sampleEmail(kind, toName)
	return subject, body, nil
}
