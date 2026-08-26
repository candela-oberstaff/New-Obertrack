package service

import (
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
const (
	EmailKindChatDigest         = "chat_digest"
	EmailKindInactivityAlert    = "inactivity_alert"
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
	ManagedElsewhere string `json:"managed_elsewhere,omitempty"`
	Enabled          bool   `json:"enabled"`
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
		Key: EmailKindChatDigest, Category: EmailCategoryAutomatic,
		Name:        "Mensajes de chat sin leer",
		Description: "«Tienes N mensajes esperando en Obertrack», con enlace al chat.",
		Trigger:     "Cada 30 min: a quien lleva 2 h o más con mensajes sin leer y no está conectado. Máximo uno al día por persona.",
		Recipient:   "El usuario con mensajes pendientes",
	},
	{
		Key: EmailKindInactivityAlert, Category: EmailCategoryAutomatic,
		Name:        "Alerta de inactividad",
		Description: "Profesionales que llevan días sin registrar horas.",
		Trigger:     "Chequeo diario: 2+ días sin registrar. No repite la misma persona en 7 días.",
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
}

// EmailSettingsService resuelve si un tipo de correo está activo y expone el
// catálogo al panel. La consulta ocurre en CADA envío, así que el estado vive
// en un caché en memoria que se invalida al guardar.
type EmailSettingsService struct {
	repo  repository.EmailSettingRepository
	brevo *BrevoService

	mu      sync.RWMutex
	cache   map[string]bool
	loaded  bool
	expires time.Time
}

// emailSettingsTTL refresca el caché aunque la escritura venga de otra
// instancia del backend (despliegues con más de una réplica).
const emailSettingsTTL = 60 * time.Second

func NewEmailSettingsService(repo repository.EmailSettingRepository, brevo *BrevoService) *EmailSettingsService {
	return &EmailSettingsService{repo: repo, cache: map[string]bool{}, brevo: brevo}
}

// Enabled dice si un tipo de correo puede salir.
//
// Sin fila guardada responde true: la ausencia significa "nunca se tocó", y por
// defecto todos los correos están encendidos.
//
// Ante un FALLO DE BASE el criterio cambia según el tipo. Antes se respondía
// siempre true —"el silencio nunca debe nacer de un error"—, que es correcto
// para recuperar contraseña: preferimos un correo de más antes que dejar a
// alguien sin poder entrar. Pero aplicado a un correo automático que alguien
// apagó a propósito, convertía el interruptor en algo que funciona CASI
// siempre, y en silencio: el correo salía y no quedaba constancia de que se
// había ignorado el apagado. Peor que no tener interruptor, porque nadie
// desconfía de él.
//
// Ahora solo los tipos Essential se dejan pasar ante un error; el resto se
// frena. Y en ambos casos queda un log, para que la próxima vez esto se
// responda leyendo los logs en vez de deduciéndolo.
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

// List devuelve el catálogo con el estado actual de cada correo.
func (s *EmailSettingsService) List() []EmailType {
	saved := map[string]bool{}
	if rows, err := s.repo.List(); err == nil {
		for _, r := range rows {
			saved[r.Key] = r.Enabled
		}
	}
	out := make([]EmailType, 0, len(emailCatalog))
	for _, t := range emailCatalog {
		enabled, ok := saved[t.Key]
		t.Enabled = !ok || enabled
		out = append(out, t)
	}
	return out
}

// SetEnabled guarda el interruptor de un tipo y refresca el caché.
func (s *EmailSettingsService) SetEnabled(kind string, enabled bool, userID uint) error {
	if !isKnownEmailKind(kind) {
		return fmt.Errorf("tipo de correo desconocido: %s", kind)
	}
	if err := s.repo.Upsert(&models.EmailSetting{Key: kind, Enabled: enabled, UpdatedBy: userID}); err != nil {
		return err
	}
	s.mu.Lock()
	s.loaded = false // fuerza recarga en la próxima consulta
	s.mu.Unlock()
	return nil
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
