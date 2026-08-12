package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
	"github.com/obertrack/backend/internal/websocket"
)

// TicketService holds all business logic for the support inbox. Handlers and
// webhook ingestion both go through it, so DB access stays in the repository
// layer (audit: handlers must not touch *gorm.DB directly).
type TicketService interface {
	List(requesterID uint, userType string) ([]models.Ticket, error)
	Get(id, requesterID uint, userType string) (*models.Ticket, error)
	Update(id, requesterID uint, userType string, stage models.TicketStage, status string, assignedTo *uint) (*models.Ticket, error)
	SendAgentMessage(id, agentID uint, userType string, content string, channel models.MessageChannel) (*models.TicketMessage, error)

	IngestWhatsApp(session, peer, body, externalID string, fromMe bool) error
	IngestEmail(fromEmail, fromName, subject, textBody, messageID string) error

	// ListInternal returns Obertrack-generated alert tickets (origin = internal).
	ListInternal() ([]models.Ticket, error)
	ListWhatsApp() ([]models.Ticket, error)
	GetWhatsAppTicket(id uint) (*models.Ticket, error)
	// LookupWhatsAppChat dice si un teléfono tiene ya conversación de WhatsApp
	// con nosotros, para poder abrirla en vez de intentar escribir en frío.
	LookupWhatsAppChat(phone string) (*WhatsAppChatLookup, error)
	// OpenWhatsAppChat devuelve la conversación del teléfono, creándola si no
	// existe, para poder abrirla siempre en nuestra bandeja.
	OpenWhatsAppChat(phone, name string) (*WhatsAppChatLookup, error)
	// CanReplyWhatsApp dice si se puede enviar en esa conversación. La interfaz
	// lo usa para bloquear el cuadro de texto con el motivo, en vez de dejar
	// escribir un mensaje que el envío va a rechazar.
	CanReplyWhatsApp(ticketID uint) bool
	// LoadOlderMessages pide a WAHA más mensajes históricos de una conversación
	// y guarda los que falten en la base de datos.
	LoadOlderMessages(ticketID uint) (int, error)
	// DownloadWaMedia baja un adjunto de WAHA transmitiéndolo en tiempo real.
	DownloadWaMedia(ticketID uint, externalID string) (io.ReadCloser, string, error)
	// ImportWhatsAppHistory pulls recent chats + messages from the connected WAHA
	// session and imports them as tickets/messages (idempotent). Returns the count
	// of newly imported messages.
	ImportWhatsAppHistory() (int, error)
	// ListWhatsAppSessions inventaría lo guardado por sesión, marcando cuál es la
	// activa. Es lo que se le muestra a quien va a borrar, para que sepa qué se
	// lleva por delante antes de confirmar.
	ListWhatsAppSessions() ([]WhatsAppSessionInfo, error)
	// PurgeWhatsAppSession borra definitivamente las conversaciones de una sesión.
	PurgeWhatsAppSession(session string) (repository.WhatsAppPurgeCounts, error)
	SendWhatsAppReply(id, agentID uint, content string) (*models.TicketMessage, error)
	// WhatsAppAction ejecuta claim/resolve/reopen. El claim es atómico: si otro
	// agente ya atiende la conversación devuelve ErrConflict, salvo que el actor
	// sea superadmin (retomar deliberado, el único caso que la UI ofrece).
	WhatsAppAction(id, agentID uint, action string, isSuperadmin bool) (*models.Ticket, error)
	// WhatsAppAssign traspasa un chat de WhatsApp a otro agente, dejando la
	// misma bitácora que un ticket interno.
	WhatsAppAssign(id, actorID, assigneeID uint, isSuperadmin bool, reason string) (*models.Ticket, error)
	// GetInternal returns a single internal alert ticket (with notes).
	GetInternal(id uint) (*models.Ticket, error)
	// ListInternalReport returns internal alerts created within [start, end].
	ListInternalReport(start, end time.Time) ([]models.Ticket, error)
	// CreateWorkHourRejectionAlert opens an internal support alert when a
	// professional's work hours are rejected.
	CreateWorkHourRejectionAlert(in RejectionAlertInput) error
	// CreateInductionFailureAlert abre una alerta interna cuando un profesional
	// recién contratado agota sus intentos de inducción sin aprobar, para que
	// Soporte lo contacte.
	CreateInductionFailureAlert(in InductionAlertInput) error
	// CreateObersuiteHireAlert abre el ticket de una incorporación llegada
	// desde Obersuite, para que la bandeja de soporte la vea.
	CreateObersuiteHireAlert(in ObersuiteHireInput) error
	// CloseObersuiteHireAlert cierra esa incorporación al aprobar la capacitación.
	CloseObersuiteHireAlert(userID uint) error
	// UpdateInternal changes the stage/status of an internal alert ticket
	// (e.g. mark as resolved). It never touches Zoho.
	UpdateInternal(id uint, stage models.TicketStage, status string) (*models.Ticket, error)
	// AddInternalNote appends a follow-up note (channel = note) to an internal alert.
	AddInternalNote(id, agentID uint, content string) (*models.TicketMessage, error)

	// ListSupportAgents returns active customer_success users (transfer targets).
	ListSupportAgents() ([]models.User, error)
	// RecordTransfer persists a transfer audit row, notifies both parties, and
	// (for internal tickets) appends a system event to the timeline.
	RecordTransfer(in TransferInput) error
	// TransferInternal reassigns an internal alert ticket and audits it.
	TransferInternal(id, toUserID, byUserID uint, isSuperadmin bool, reason string) (*models.Ticket, error)
	// ListTransfers returns the transfer history for a ticket.
	ListTransfers(origin, ref string) ([]models.TicketTransfer, error)
	// GetUserName returns a user's display name by id (for audit labels).
	GetUserName(id uint) (string, error)
}

// RejectionAlertInput carries the data denormalized onto a work-hour rejection
// alert ticket, used both for the follow-up modal and the rejections report.
type RejectionAlertInput struct {
	ProfessionalID    uint
	ProfessionalName  string
	ProfessionalEmail string
	ProfessionalPhone string
	CompanyName       string
	RejectedByName    string
	Dates             string
	Reason            string
}

// InductionAlertInput lleva los datos de un profesional que no aprobó la
// inducción, denormalizados sobre la alerta para que Soporte pueda contactarlo
// sin buscar en otras pantallas.
type InductionAlertInput struct {
	ProfessionalID    uint
	ProfessionalName  string
	ProfessionalEmail string
	ProfessionalPhone string
	CompanyName       string
	Score             float64
	PassingScore      int
	Attempts          int
}

// ObersuiteHireInput son los datos de quien acaba de ser contratado en
// Obersuite. Van denormalizados sobre el ticket, como en las demás alertas,
// para que Soporte pueda contactarlo sin salir de la bandeja.
type ObersuiteHireInput struct {
	ProfessionalID    uint
	ProfessionalName  string
	ProfessionalEmail string
	ProfessionalPhone string
	CompanyName       string
	JobTitle          string
	// InductionSent distingue a quien ya recibió la capacitación de quien no:
	// cambia lo que hay que hacer con él.
	InductionSent bool
}

type ticketService struct {
	repo        repository.TicketRepository
	userRepo    repository.UserRepository
	notifSvc    NotificationService
	wahaSvc     *WahaService
	outbox      *WhatsAppOutbox
	brevoSvc    *BrevoService
	supportNtfy *SupportNotifier
	// importMu deja pasar un solo import de historial a la vez. Ahora hay dos
	// disparadores —el watcher periódico y el botón de la bandeja— y dos pasadas
	// simultáneas competirían creando el mismo contacto y el mismo ticket.
	importMu sync.Mutex
}

func NewTicketService(repo repository.TicketRepository, userRepo repository.UserRepository, notifSvc NotificationService, wahaSvc *WahaService, brevoSvc *BrevoService, supportNtfy *SupportNotifier, outbox *WhatsAppOutbox) TicketService {
	return &ticketService{repo: repo, userRepo: userRepo, notifSvc: notifSvc, wahaSvc: wahaSvc, brevoSvc: brevoSvc, supportNtfy: supportNtfy, outbox: outbox}
}

// TransferInput describes a ticket reassignment to be audited.
type TransferInput struct {
	Origin      string
	TicketRef   string
	TicketTitle string
	FromUserID  *uint
	FromName    string
	ToUserID    *uint
	ToName      string
	ByUserID    uint
	ByName      string
	Reason      string
	// AddTimelineEvent appends a "system" note to the local ticket (internal only).
	AddTimelineEvent bool
	LocalTicketID    uint
}

// enrichInternal backfills missing denormalized fields on an internal alert so
// the detail/report never shows empty data: contact info is resolved live from
// the linked professional (and employer), and dates/reason fall back to parsing
// the description (for legacy alerts created before these fields existed).
func (s *ticketService) enrichInternal(t *models.Ticket) {
	if t == nil || !models.IsLocalOrigin(t.Origin) {
		return
	}
	if (t.ProfessionalEmail == "" || t.ProfessionalPhone == "" || t.CompanyName == "") && t.UserID != nil && s.userRepo != nil {
		if u, err := s.userRepo.GetByID(*t.UserID); err == nil && u != nil {
			if t.ProfessionalEmail == "" {
				t.ProfessionalEmail = u.Email
			}
			if t.ProfessionalPhone == "" {
				t.ProfessionalPhone = u.PhoneNumber
			}
			if t.CompanyName == "" && u.EmpleadorID != nil {
				if emp, err := s.userRepo.GetByID(*u.EmpleadorID); err == nil && emp != nil {
					t.CompanyName = emp.CompanyName
				}
			}
		}
	}
	// Fallback: parse "Jornadas rechazadas (<dates>). Motivo: <reason>".
	//
	// Solo para las alertas internas: es un apaño para las de rechazo antiguas,
	// que no guardaban esos campos. Aplicado a un alta de Obersuite tomaría
	// cualquier paréntesis de su descripción como si fueran fechas de jornada, y
	// la ficha enseñaría un "Fechas: ..." inventado.
	if t.Origin == models.OriginInternal && (t.WorkDates == "" || t.Reason == "") && t.Description != "" {
		if t.WorkDates == "" {
			if a := strings.Index(t.Description, "("); a >= 0 {
				if b := strings.Index(t.Description[a:], ")"); b > 0 {
					t.WorkDates = t.Description[a+1 : a+b]
				}
			}
		}
		if t.Reason == "" {
			if i := strings.Index(t.Description, "Motivo: "); i >= 0 {
				t.Reason = t.Description[i+len("Motivo: "):]
			}
		}
	}
}

// canAccess returns true if the caller may view/act on the ticket.
func (s *ticketService) canAccess(userType string) bool {
	// Restrict to customer_success only (as per latest instruction)
	if userType == string(models.UserTypeCustomerSuccess) {
		return true
	}
	return false
}

func (s *ticketService) List(_ uint, userType string) ([]models.Ticket, error) {
	// Restrict to customer_success only
	if userType == string(models.UserTypeCustomerSuccess) {
		return s.repo.List(nil)
	}
	return nil, apperrors.ErrAccessDenied
}

func (s *ticketService) Get(id, requesterID uint, userType string) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !s.canAccess(userType) {
		return nil, apperrors.ErrAccessDenied
	}
	return ticket, nil
}

func (s *ticketService) Update(id, requesterID uint, userType string, stage models.TicketStage, status string, assignedTo *uint) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !s.canAccess(userType) {
		return nil, apperrors.ErrAccessDenied
	}

	if stage != "" {
		ticket.Stage = stage
	}
	if status != "" {
		ticket.Status = status
	}
	if assignedTo != nil {
		ticket.AssignedTo = assignedTo
	}

	if err := s.repo.SaveTicket(ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *ticketService) SendAgentMessage(id, agentID uint, userType string, content string, channel models.MessageChannel) (*models.TicketMessage, error) {
	ticket, err := s.repo.GetWithContact(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !s.canAccess(userType) {
		return nil, apperrors.ErrAccessDenied
	}

	// Send the outbound message via the appropriate integration.
	var deliveryStatus string
	switch channel {
	case models.ChannelWhatsApp:
		if ticket.Contact == nil {
			return nil, apperrors.ErrExternalSend
		}
		// La comprobación de cold-outreach sigue siendo síncrona: es una consulta
		// local y barata, y el agente tiene que enterarse en el acto de que no
		// puede escribir primero a ese contacto.
		if err := s.ensureCanColdOutreach(ticket.ID); err != nil {
			return nil, err
		}
		// El envío a WAHA lo hace el outbox: aquí solo se persiste la intención.
		deliveryStatus = models.DeliveryPending
	case models.ChannelEmail:
		if ticket.Contact == nil {
			return nil, apperrors.ErrExternalSend
		}
		if err := s.brevoSvc.SendEmail(ticket.Contact.Email, ticket.Contact.Name, ticket.Title, content); err != nil {
			return nil, apperrors.ErrExternalSend
		}
	}

	msg := &models.TicketMessage{
		TicketID:       ticket.ID,
		SenderType:     models.SenderTypeAgent,
		SenderID:       &agentID,
		Channel:        channel,
		Content:        content,
		DeliveryStatus: deliveryStatus,
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, err
	}
	// Con el mensaje ya a salvo en la BD se despierta al worker. Si esta señal se
	// perdiera, el tick periódico lo recogería igual: es una optimización de
	// latencia, no parte de la garantía de entrega.
	if deliveryStatus == models.DeliveryPending && s.outbox != nil {
		s.outbox.Signal()
	}
	return msg, nil
}

// resolveContactName devuelve el nombre del usuario de Obertrack si el teléfono
// coincide con un usuario activo, o wahaName si no se encuentra.
func (s *ticketService) resolveContactName(phone, wahaName string) string {
	digits := utils.NormalizePhoneDigits(phone)
	if digits != "" {
		if u, err := s.userRepo.FindActiveByPhoneDigits(digits); err == nil && u != nil && u.Name != "" {
			return u.Name
		}
	}
	return wahaName
}

// IngestWhatsApp handles a WhatsApp message: resolve/create the contact, attach
// it to the contact's open ticket (or open a new one), persist and broadcast it.
//
// `peer` es SIEMPRE el JID del contacto del otro lado, nunca el de la cuenta
// propia: en un mensaje saliente el llamador debe pasar el destinatario.
// `fromMe` distingue lo que se escribió desde el teléfono —que se guarda como
// mensaje de agente— de lo que escribió el contacto.
func (s *ticketService) IngestWhatsApp(session, peer, body, externalID string, fromMe bool) error {
	from := peer
	// Idempotencia: si el webhook se reintenta, el mismo external_id ya fue
	// procesado. Cortamos temprano para no recrear contacto/ticket ni redifundir.
	if externalID != "" {
		if exists, err := s.repo.MessageExistsByExternalID(externalID); err == nil && exists {
			return nil
		}
	}

	phone := from
	if i := strings.IndexByte(from, '@'); i >= 0 {
		phone = from[:i]
	}

	resolvedName := "WA User " + phone
	if contact, err := s.wahaSvc.GetContact(session, from); err == nil && contact != nil {
		if name := contact.BestName(); name != "" {
			resolvedName = name
		}
		if realPhone := contact.RealPhone(); realPhone != "" {
			phone = realPhone
		}
	}
	resolvedName = s.resolveContactName(phone, resolvedName)

	contact, err := s.repo.GetContactByPhone(phone)
	if err != nil {
		contact = &models.Contact{Phone: phone, Name: resolvedName, WaID: from}
		if err := s.repo.CreateContact(contact); err != nil {
			return err
		}
	} else {
		dirty := false
		if contact.Name != resolvedName && resolvedName != "" && resolvedName != "WA User "+phone {
			contact.Name = resolvedName
			dirty = true
		}
		if contact.WaID == "" && from != "" {
			contact.WaID = from
			dirty = true
		}
		if dirty {
			_ = s.repo.SaveContact(contact)
		}
	}

	// Un chat de WhatsApp es una conversación continua, no un caso que se reabre:
	// se reutiliza el ticket abierto del contacto sin ventana temporal. Con la
	// ventana de 1h que había antes, un contacto que escribía tras dos días de
	// silencio estrenaba ticket, y la interfaz —que muestra un hilo por ticket—
	// partía la conversación en dos, dejando las respuestas en el hilo viejo.
	// Cuando alguien resuelve el ticket, el siguiente mensaje sí abre uno nuevo.
	ticket, err := s.repo.GetOpenTicketByContact(contact.ID, session)
	if err != nil {
		ticket = &models.Ticket{
			ContactID: &contact.ID,
			Origin:    string(models.ChannelWhatsApp),
			Session:   session,
			Title:     "WA: " + phone,
			Stage:     models.StageNew,
			Status:    "open",
		}
		if err := s.repo.CreateTicket(ticket); err != nil {
			return err
		}
		// Solo se avisa a soporte cuando escribe el contacto: un mensaje que salió
		// del propio teléfono no es una solicitud que haya que atender.
		if s.supportNtfy != nil && !fromMe {
			s.supportNtfy.Notify(SupportTicketInfo{
				Type:        "WhatsApp",
				Requester:   resolvedName,
				Subject:     ticket.Title,
				Description: body,
				Link:        "/tickets",
			})
		}
	}

	// Lo escrito desde el teléfono se registra como mensaje de agente, igual que
	// hace el import del historial, para que el hilo muestre los dos lados.
	sender := models.SenderTypeContact
	if fromMe {
		sender = models.SenderTypeAgent
	}
	msg := &models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: sender,
		Channel:    models.ChannelWhatsApp,
		Content:    body,
		ExternalID: externalID,
		// Ya está entregado: lo mandó WhatsApp, no nuestra bandeja de salida.
		DeliveryStatus: deliveryStatusFor(fromMe),
	}
	// Insert idempotente: si una entrega concurrente ya guardó este external_id,
	// no se inserta de nuevo y evitamos redifundir un mensaje duplicado.
	inserted, err := s.repo.CreateMessageIfNew(msg)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}
	_ = s.repo.TouchTicket(ticket)

	broadcastTicketMessage(ticket.ID, msg)
	return nil
}

// deliveryStatusFor devuelve el estado de entrega de un mensaje ingerido. Los
// salientes que llegan por webhook ya fueron entregados por WhatsApp, así que se
// marcan como 'sent' y el worker del outbox no los toca; los entrantes no tienen
// estado de entrega (vacío = no aplica).
func deliveryStatusFor(fromMe bool) string {
	if fromMe {
		return models.DeliverySent
	}
	return ""
}

// IngestEmail handles an inbound email: resolve/create the contact, attach to an
// open ticket (or open a new one), persist the message and broadcast it.
func (s *ticketService) IngestEmail(fromEmail, fromName, subject, textBody, messageID string) error {
	if fromEmail == "" {
		return apperrors.ErrInvalidInput
	}

	// Idempotencia: los reintentos de Brevo reenvían el mismo Message-ID.
	if messageID != "" {
		if exists, err := s.repo.MessageExistsByExternalID(messageID); err == nil && exists {
			return nil
		}
	}

	contact, err := s.repo.GetContactByEmail(fromEmail)
	if err != nil {
		name := fromName
		if name == "" {
			name = fromEmail
		}
		contact = &models.Contact{Email: fromEmail, Name: name}
		if err := s.repo.CreateContact(contact); err != nil {
			return err
		}
	}

	// Los tickets de correo no pertenecen a ninguna sesión de WhatsApp.
	ticket, err := s.repo.GetOpenTicketByContact(contact.ID, "")
	if err != nil {
		title := subject
		if title == "" {
			title = "Email from " + fromEmail
		}
		ticket = &models.Ticket{
			ContactID: &contact.ID,
			Origin:    string(models.ChannelEmail),
			Title:     title,
			Stage:     models.StageNew,
			Status:    "open",
		}
		if err := s.repo.CreateTicket(ticket); err != nil {
			return err
		}
		if s.supportNtfy != nil {
			s.supportNtfy.Notify(SupportTicketInfo{
				Type:        "Email",
				Requester:   contact.Name,
				Subject:     subject,
				Description: textBody,
				Link:        "/tickets",
			})
		}
	}

	msg := &models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: models.SenderTypeContact,
		Channel:    models.ChannelEmail,
		Content:    textBody,
		ExternalID: messageID,
	}
	// Insert idempotente: backstop ante entregas concurrentes con el mismo Message-ID.
	inserted, err := s.repo.CreateMessageIfNew(msg)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	broadcastTicketMessage(ticket.ID, msg)
	return nil
}

// ListInternal devuelve los tickets que genera la propia plataforma y viven en
// nuestra base: las alertas (rechazos, inducción) y las altas llegadas desde
// Obersuite. Van juntos porque comparten bandeja y ficha de detalle.
func (s *ticketService) ListInternal() ([]models.Ticket, error) {
	tickets, err := s.repo.ListByOrigins(models.OriginInternal, models.OriginObersuite)
	if err != nil {
		return nil, err
	}
	for i := range tickets {
		s.enrichInternal(&tickets[i])
	}
	s.attachLastMessages(tickets)
	return tickets, nil
}

// attachLastMessages deja en cada ticket SOLO su último mensaje con contenido,
// que es lo único que consumen los listados (la vista previa de la tarjeta).
// Antes se precargaba Messages entero y el tablero —que se refresca solo cada
// minuto— descargaba el historial completo de todas las conversaciones.
// Best-effort: si la consulta falla, los tickets salen sin preview (se cae al
// título), no se corta el listado.
func (s *ticketService) attachLastMessages(tickets []models.Ticket) {
	if len(tickets) == 0 {
		return
	}
	ids := make([]uint, len(tickets))
	for i := range tickets {
		ids[i] = tickets[i].ID
	}
	last, err := s.repo.LastMessagesByTicketIDs(ids)
	if err != nil {
		log.Printf("[Tickets] no se pudo resolver la vista previa de los tickets: %v", err)
		last = nil
	}
	for i := range tickets {
		if m, ok := last[tickets[i].ID]; ok {
			tickets[i].Messages = []models.TicketMessage{m}
		} else {
			tickets[i].Messages = []models.TicketMessage{}
		}
	}
}

// ListWhatsApp devuelve SOLO las conversaciones de la sesión activa. Al cambiar
// el número conectado, los chats de la cuenta anterior siguen en la base pero
// desaparecen de la bandeja: mezclarlos era el problema.
func (s *ticketService) ListWhatsApp() ([]models.Ticket, error) {
	tickets, err := s.repo.ListByOriginAndSession(string(models.ChannelWhatsApp), s.wahaSvc.GetSession())
	if err != nil {
		return nil, err
	}
	s.attachLastMessages(tickets)
	return tickets, nil
}

// WhatsAppChatLookup es la respuesta a "¿puedo escribirle por WhatsApp a este
// número?". Existe porque la respuesta no es sí o no: depende de si ya hay
// conversación y de si esa conversación la empezaron ellos.
type WhatsAppChatLookup struct {
	// Digits es el teléfono ya normalizado; vacío si no había número.
	Digits string `json:"digits"`
	// TicketID es la conversación existente, o 0 si no hay ninguna.
	TicketID uint `json:"ticket_id"`
	// CanReply indica si se puede responder desde la bandeja. Es false cuando
	// no hay hilo, y también cuando lo hay pero nadie ha escrito desde el otro
	// lado: la guarda de contacto en frío rechazaría el envío igualmente, y es
	// mejor decirlo antes que enseñar un cuadro de texto que va a dar 403.
	CanReply bool `json:"can_reply"`
	// IsRegistered dice si el número corresponde a un usuario activo de Obertrack
	IsRegistered bool `json:"is_registered"`
}

// LookupWhatsAppChat busca la conversación de WhatsApp de un teléfono dentro de
// la sesión activa.
//
// No crea nada: iniciar una conversación con quien no nos ha escrito es
// exactamente lo que impide ensureCanColdOutreach para no exponer la línea
// oficial a un bloqueo de Meta. Esto solo informa de lo que ya existe.
func (s *ticketService) LookupWhatsAppChat(phone string) (*WhatsAppChatLookup, error) {
	digits := utils.NormalizePhoneDigits(phone)
	out := &WhatsAppChatLookup{Digits: digits}
	if digits == "" {
		return out, nil
	}
	
	out.IsRegistered = s.contactIsPlatformUser(digits)

	ticket, err := s.repo.FindWhatsAppTicketByPhoneDigits(digits, s.wahaSvc.GetSession())
	if err != nil {
		// Sin conversación no es un error: es la respuesta más común.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, nil
		}
		return nil, err
	}
	out.TicketID = ticket.ID

	// Solo se puede responder si escribieron ellos primero. Si la guarda está
	// desactivada por configuración, cualquier hilo existente vale.
	if !s.wahaSvc.RequireInboundBeforeSend() {
		out.CanReply = true
		return out, nil
	}
	inbound, err := s.repo.HasInboundMessage(ticket.ID)
	if err != nil {
		// Igual que ensureCanColdOutreach: ante un fallo de base se deja pasar,
		// y el envío real vuelve a comprobarlo antes de salir a WAHA.
		out.CanReply = true
		return out, nil
	}
	// Misma excepción que la guarda del envío: a nuestra propia gente (usuario
	// activo de la plataforma) se le puede escribir primero.
	out.CanReply = inbound || s.contactIsPlatformUser(digits)
	return out, nil
}

// CanReplyWhatsApp aplica la misma guarda que el envío real, para poder
// anticiparla en la interfaz.
func (s *ticketService) CanReplyWhatsApp(ticketID uint) bool {
	return s.ensureCanColdOutreach(ticketID) == nil
}

// OpenWhatsAppChat devuelve la conversación de un teléfono, creándola si aún no
// existe. Es lo que necesita el botón de WhatsApp de la ficha de empresa para
// llevar SIEMPRE a nuestra bandeja en vez de saltar a wa.me: quien atiende ve el
// historial, las notas y el estado en el mismo sitio que el resto.
//
// Crear la conversación NO permite escribir en frío: el envío sigue pasando por
// ensureCanColdOutreach. Lo que se crea es el hilo vacío donde apoyarse; si
// nadie ha escrito desde el otro lado, CanReply viaja en false y la interfaz lo
// dice en vez de dejar probar un envío que va a fallar.
func (s *ticketService) OpenWhatsAppChat(phone, name string) (*WhatsAppChatLookup, error) {
	out, err := s.LookupWhatsAppChat(phone)
	if err != nil {
		return nil, err
	}
	if out.Digits == "" {
		return nil, apperrors.ErrInvalidInput
	}
	if out.TicketID > 0 {
		return out, nil
	}

	contact, err := s.repo.GetContactByPhone(out.Digits)
	if err != nil || contact == nil {
		displayName := strings.TrimSpace(name)
		if displayName == "" {
			displayName = "WA User " + out.Digits
		}
		contact = &models.Contact{Phone: out.Digits, Name: displayName}
		if err := s.repo.CreateContact(contact); err != nil {
			return nil, err
		}
	}

	ticket := &models.Ticket{
		ContactID: &contact.ID,
		Origin:    string(models.ChannelWhatsApp),
		Session:   s.wahaSvc.GetSession(),
		Title:     "WA: " + out.Digits,
		Stage:     models.StageNew,
		Status:    "open",
	}
	if err := s.repo.CreateTicket(ticket); err != nil {
		return nil, err
	}
	// Sin aviso a soporte: esta conversación la abre el propio equipo desde una
	// ficha, no es una solicitud entrante que haya que repartir.
	out.TicketID = ticket.ID
	return out, nil
}

func (s *ticketService) GetWhatsAppTicket(id uint) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if ticket.Origin != string(models.ChannelWhatsApp) {
		return nil, apperrors.ErrNotFound
	}
	// El detalle se acota igual que el listado: si no, un enlace directo seguiría
	// abriendo conversaciones de la cuenta anterior.
	if ticket.Session != s.wahaSvc.GetSession() {
		return nil, apperrors.ErrNotFound
	}

	// If the contact is still a generic "WA User <n>" placeholder, resolve its
	// real name/phone from WAHA — but do it in the background so opening a chat
	// never waits on an external HTTP round-trip. The refreshed data lands in the
	// DB and shows on the next open (ContactSync also backfills these periodically).
	if c := ticket.Contact; c != nil && strings.HasPrefix(c.Name, "WA User ") && c.WaID != "" {
		go s.refreshWhatsAppContact(c.ID, c.WaID)
	}
	return ticket, nil
}

// refreshWhatsAppContact resolves a placeholder contact's real name/phone from
// WAHA and persists it. Runs off the request path (fire-and-forget goroutine).
func (s *ticketService) refreshWhatsAppContact(contactID uint, waID string) {
	resolved, err := s.wahaSvc.GetContact(s.wahaSvc.GetSession(), waID)
	if err != nil || resolved == nil {
		return
	}
	// Re-load the contact fresh to avoid racing with a concurrent update.
	c, err := s.repo.GetContactByID(contactID)
	if err != nil || c == nil {
		return
	}
	changed := false
	if realPhone := resolved.RealPhone(); realPhone != "" && realPhone != c.Phone {
		c.Phone = realPhone
		changed = true
	}

	resolvedName := s.resolveContactName(c.Phone, resolved.BestName())
	if resolvedName != "" && resolvedName != c.Name && resolvedName != "WA User "+c.Phone {
		c.Name = resolvedName
		changed = true
	}
	if changed {
		_ = s.repo.SaveContact(c)
	}
}

const (
	importMaxChats = 30 // how many recent chats to pull from the session
	importMaxMsgs  = 20 // messages per chat (chosen: last ~20)
)

// ImportWhatsAppHistory pulls recent 1:1 chats from the connected WAHA session
// and imports each chat's last messages as a ticket + messages. Idempotent: the
// external_id unique index dedups messages, so re-running only adds new ones.
func (s *ticketService) ImportWhatsAppHistory() (int, error) {
	// Si ya hay una pasada en curso se rechaza en vez de encolar: el que llega
	// segundo no aportaría nada (traería exactamente los mismos chats) y sí
	// competiría por crear los mismos contactos y tickets.
	if !s.importMu.TryLock() {
		return 0, apperrors.ErrSyncInProgress
	}
	defer s.importMu.Unlock()

	session := s.wahaSvc.GetSession()
	chats, err := s.wahaSvc.GetChatsOverview(session, importMaxChats)
	if err != nil {
		return 0, err
	}

	// Concurrencia limitada: Usamos un semáforo (canal de struct{})
	// para procesar hasta 3 chats en paralelo, acelerando el proceso
	// sin sobrecargar a WAHA ni el procesador.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	var importedMu sync.Mutex
	imported := 0

	for _, chat := range chats {
		if !IsIndividualChat(chat.ID) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{} // Adquirir ranura del semáforo

		go func(c WahaChatOverview) {
			defer func() {
				<-sem // Liberar ranura
				wg.Done()
			}()

			phone := c.ID
			if i := strings.IndexByte(c.ID, '@'); i >= 0 {
				phone = c.ID[:i]
			}
			resolvedName := strings.TrimSpace(c.Name)
			if resolved, rerr := s.wahaSvc.GetContact(session, c.ID); rerr == nil && resolved != nil {
				if realPhone := resolved.RealPhone(); realPhone != "" {
					phone = realPhone
				}
				if name := resolved.BestName(); name != "" {
					resolvedName = name
				}
			}
			resolvedName = s.resolveContactName(phone, resolvedName)
			if resolvedName == "" {
				resolvedName = "WA User " + phone
			}

			// Proteger operaciones críticas de base de datos o sincronizar
			// para evitar race conditions al crear contactos o tickets.
			importedMu.Lock()
			contact, cerr := s.repo.GetContactByPhone(phone)
			if cerr != nil {
				contact = &models.Contact{Phone: phone, Name: resolvedName, WaID: c.ID}
				if err := s.repo.CreateContact(contact); err != nil {
					importedMu.Unlock()
					return
				}
			} else {
				dirty := false
				if contact.WaID == "" && c.ID != "" {
					contact.WaID = c.ID
					dirty = true
				}
				if contact.Name != resolvedName && resolvedName != "WA User "+phone {
					contact.Name = resolvedName
					dirty = true
				}
				if dirty {
					_ = s.repo.SaveContact(contact)
				}
			}

			ticket, terr := s.repo.GetOpenTicketByContact(contact.ID, session)
			if terr != nil {
				ticket = &models.Ticket{
					ContactID: &contact.ID,
					Origin:    string(models.ChannelWhatsApp),
					Session:   session,
					Title:     "WA: " + phone,
					Stage:     models.StageNew,
					Status:    "open",
				}
				if err := s.repo.CreateTicket(ticket); err != nil {
					importedMu.Unlock()
					return
				}
			}
			importedMu.Unlock()

			// La descarga de mensajes de cada chat puede ser muy lenta y
			// no requiere bloquear la creación de otros tickets.
			msgs, merr := s.wahaSvc.GetChatMessages(session, c.ID, importMaxMsgs)
			if merr != nil {
				return
			}

			localImported := 0
			for i := len(msgs) - 1; i >= 0; i-- {
				m := msgs[i]
				body := strings.TrimSpace(m.Body)
				if body == "" {
					body = MediaPlaceholder(m.Type, m.MimeType(), m.HasMedia)
				}
				if body == "" {
					continue
				}
				sender := models.SenderTypeContact
				if m.FromMe {
					sender = models.SenderTypeAgent
				}
				tm := &models.TicketMessage{
					TicketID:   ticket.ID,
					SenderType: sender,
					Channel:    models.ChannelWhatsApp,
					Content:    body,
					ExternalID: m.ID,
				}
				if m.Timestamp > 0 {
					tm.CreatedAt = time.Unix(m.Timestamp, 0)
				}

				// El repositorio ya maneja 'ON CONFLICT DO NOTHING', pero la inserción
				// concurrente debe ser protegida para mantener consistencia.
				importedMu.Lock()
				if inserted, err := s.repo.CreateMessageIfNew(tm); err == nil && inserted {
					localImported++
				}
				importedMu.Unlock()
			}

			if localImported > 0 {
				importedMu.Lock()
				imported += localImported
				importedMu.Unlock()
			}

			// Actualizar la última actividad del ticket.
			importedMu.Lock()
			if err := s.repo.SyncTicketActivity(ticket.ID); err != nil {
				log.Printf("[ChatImport] no se pudo alinear la actividad del ticket %d: %v", ticket.ID, err)
			}
			importedMu.Unlock()

		}(chat)
	}

	wg.Wait()
	return imported, nil
}

func (s *ticketService) LoadOlderMessages(ticketID uint) (int, error) {
	ticket, err := s.repo.GetByID(ticketID)
	if err != nil {
		return 0, err
	}
	if ticket.Contact == nil || ticket.Contact.WaID == "" {
		return 0, fmt.Errorf("no es un ticket de WhatsApp")
	}
	session := s.wahaSvc.GetSession()
	if session == "" {
		return 0, fmt.Errorf("WAHA no configurado")
	}

	fetchCount := len(ticket.Messages) + 20
	msgs, merr := s.wahaSvc.GetChatMessages(session, ticket.Contact.WaID, fetchCount)
	if merr != nil {
		return 0, merr
	}

	existing := make(map[string]bool)
	for _, tm := range ticket.Messages {
		if tm.ExternalID != "" {
			existing[tm.ExternalID] = true
		}
	}

	imported := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if existing[m.ID] {
			continue
		}
		body := strings.TrimSpace(m.Body)
		if body == "" {
			body = MediaPlaceholder(m.Type, m.MimeType(), m.HasMedia)
		}
		if body == "" {
			continue
		}
		sender := models.SenderTypeContact
		if m.FromMe {
			sender = models.SenderTypeAgent
		}
		tm := &models.TicketMessage{
			TicketID:   ticket.ID,
			SenderType: sender,
			Channel:    models.ChannelWhatsApp,
			Content:    body,
			ExternalID: m.ID,
		}
		if m.Timestamp > 0 {
			tm.CreatedAt = time.Unix(m.Timestamp, 0)
		}
		if inserted, err := s.repo.CreateMessageIfNew(tm); err == nil && inserted {
			imported++
		}
	}
	if imported > 0 {
		_ = s.repo.SyncTicketActivity(ticket.ID)
	}
	return imported, nil
}

func (s *ticketService) DownloadWaMedia(ticketID uint, externalID string) (io.ReadCloser, string, error) {
	ticket, err := s.repo.GetByID(ticketID)
	if err != nil {
		return nil, "", err
	}
	if ticket.Contact == nil || ticket.Contact.WaID == "" {
		return nil, "", fmt.Errorf("no es un ticket de WhatsApp")
	}
	session := s.wahaSvc.GetSession()
	if session == "" {
		return nil, "", fmt.Errorf("WAHA no configurado")
	}
	return s.wahaSvc.DownloadMessageMedia(session, ticket.Contact.WaID, externalID)
}

// WhatsAppSessionInfo es una sesión con datos guardados. `Current` distingue la
// que está configurada ahora de las huérfanas: números ya desvinculados cuyas
// conversaciones no se ven en la bandeja pero siguen en la base.
type WhatsAppSessionInfo struct {
	Session  string `json:"session"`
	Tickets  int64  `json:"tickets"`
	Messages int64  `json:"messages"`
	Current  bool   `json:"current"`
}

func (s *ticketService) ListWhatsAppSessions() ([]WhatsAppSessionInfo, error) {
	stats, err := s.repo.ListWhatsAppSessions()
	if err != nil {
		return nil, err
	}
	current := s.wahaSvc.GetSession()
	out := make([]WhatsAppSessionInfo, 0, len(stats))
	for _, st := range stats {
		out = append(out, WhatsAppSessionInfo{
			Session:  st.Session,
			Tickets:  st.Tickets,
			Messages: st.Messages,
			Current:  st.Session == current,
		})
	}
	return out, nil
}

// PurgeWhatsAppSession borra las conversaciones de una sesión. No hay vuelta
// atrás y es a propósito: existe para que las conversaciones de un número
// desvinculado dejen de estar en la base, no para archivarlas.
func (s *ticketService) PurgeWhatsAppSession(session string) (repository.WhatsAppPurgeCounts, error) {
	if strings.TrimSpace(session) == "" {
		return repository.WhatsAppPurgeCounts{}, apperrors.ErrInvalidInput
	}
	counts, err := s.repo.PurgeWhatsAppSession(session)
	if err != nil {
		return counts, err
	}
	log.Printf("[WAHA] purga de la sesión %q: %d ticket(s), %d mensaje(s), %d contacto(s) borrados",
		session, counts.Tickets, counts.Messages, counts.Contacts)
	return counts, nil
}

// ensureCanColdOutreach blocks sending to a WhatsApp contact that never wrote
// first — cold outreach is the highest ban risk. Disabled via WAHA_REQUIRE_INBOUND.
// On a DB error it fails open (logs and allows) so a transient glitch never blocks
// a legitimate reply; the rate limiter remains the primary anti-ban control.
//
// EXCEPCIÓN: si el número pertenece a un usuario ACTIVO de la plataforma
// (profesional/empleado nuestro), se permite iniciar la conversación. El caso
// real es el check-in de emergencias del Mapa ("¿estás bien?"): escribirle
// primero a nuestra propia gente no es contacto en frío — nos conocen y el
// riesgo de reporte que la guarda previene no aplica. Para números externos
// (contactos de Zoho, desconocidos) la guarda sigue intacta.
func (s *ticketService) ensureCanColdOutreach(ticketID uint) error {
	if !s.wahaSvc.RequireInboundBeforeSend() {
		return nil
	}
	hasInbound, err := s.repo.HasInboundMessage(ticketID)
	if err != nil {
		log.Printf("[WAHA] cold-outreach check failed for ticket %d, allowing send: %v", ticketID, err)
		return nil
	}
	if !hasInbound {
		if ticket, terr := s.repo.GetWithContact(ticketID); terr == nil && ticket.Contact != nil &&
			s.contactIsPlatformUser(ticket.Contact.Phone) {
			return nil
		}
		return apperrors.ErrColdOutreach
	}
	return nil
}

// contactIsPlatformUser dice si un teléfono pertenece a un usuario activo de
// la plataforma (la excepción de la guarda de contacto en frío).
func (s *ticketService) contactIsPlatformUser(phone string) bool {
	digits := utils.NormalizePhoneDigits(phone)
	if len(digits) < 8 || s.userRepo == nil {
		return false
	}
	u, err := s.userRepo.FindActiveByPhoneDigits(digits)
	return err == nil && u != nil
}

func (s *ticketService) SendWhatsAppReply(id, agentID uint, content string) (*models.TicketMessage, error) {
	ticket, err := s.repo.GetWithContact(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if ticket.Origin != string(models.ChannelWhatsApp) || ticket.Contact == nil {
		return nil, apperrors.ErrExternalSend
	}
	if err := s.ensureCanColdOutreach(ticket.ID); err != nil {
		return nil, err
	}
	// El mensaje se encola en vez de enviarse aquí: el worker lo entrega con
	// reintentos y el agente lo ve en el chat en el acto, marcado como pendiente,
	// sin esperar los ~6s del espaciado antibaneo.
	msg := &models.TicketMessage{
		TicketID:       ticket.ID,
		SenderType:     models.SenderTypeAgent,
		SenderID:       &agentID,
		Channel:        models.ChannelWhatsApp,
		Content:        content,
		DeliveryStatus: models.DeliveryPending,
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, err
	}
	_ = s.repo.TouchTicket(ticket)
	broadcastTicketMessage(ticket.ID, msg)
	if s.outbox != nil {
		s.outbox.Signal()
	}
	return msg, nil
}

func (s *ticketService) WhatsAppAction(id, agentID uint, action string, isSuperadmin bool) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if ticket.Origin != string(models.ChannelWhatsApp) {
		return nil, apperrors.ErrNotFound
	}
	// Updates por columna y no SaveTicket: la fila entera pisaría cambios
	// concurrentes del webhook (mensajes entrantes tocando updated_at).
	switch action {
	case "claim":
		if isSuperadmin {
			// Retomar deliberado: un superadmin puede quitarle el chat a un agente
			// (es el único "Tomar" que la UI ofrece sobre un chat ya atendido).
			if err := s.repo.UpdateTicketFields(ticket.ID, map[string]interface{}{
				"assigned_to": agentID,
				"stage":       models.StageInProgress,
				"status":      "open",
			}); err != nil {
				return nil, err
			}
		} else {
			claimed, cerr := s.repo.ClaimTicketIfFree(ticket.ID, agentID)
			if cerr != nil {
				return nil, cerr
			}
			if !claimed {
				return nil, apperrors.ErrConflict
			}
		}
	case "resolve":
		if err := s.repo.UpdateTicketFields(ticket.ID, map[string]interface{}{
			"stage":  models.StageClosed,
			"status": "closed",
		}); err != nil {
			return nil, err
		}
	case "reopen":
		if err := s.repo.UpdateTicketFields(ticket.ID, map[string]interface{}{
			"assigned_to": nil,
			"stage":       models.StageNew,
			"status":      "open",
		}); err != nil {
			return nil, err
		}
	default:
		return nil, apperrors.ErrInvalidInput
	}
	return s.repo.GetByID(id)
}

// ListInternalReport returns internal alerts created within [start, end].
func (s *ticketService) ListInternalReport(start, end time.Time) ([]models.Ticket, error) {
	tickets, err := s.repo.ListInternalReport(start, end)
	if err != nil {
		return nil, err
	}
	for i := range tickets {
		s.enrichInternal(&tickets[i])
	}
	return tickets, nil
}

// CreateWorkHourRejectionAlert opens an internal support alert describing a
// work-hour rejection so the support team has a trace of it.
func (s *ticketService) CreateWorkHourRejectionAlert(in RejectionAlertInput) error {
	pid := in.ProfessionalID
	ticket := &models.Ticket{
		Origin:            models.OriginInternal,
		UserID:            &pid,
		Title:             "Rechazo de horas: " + in.ProfessionalName,
		Description:       "Jornadas rechazadas (" + in.Dates + "). Motivo: " + in.Reason,
		ProfessionalEmail: in.ProfessionalEmail,
		ProfessionalPhone: in.ProfessionalPhone,
		CompanyName:       in.CompanyName,
		RejectedByName:    in.RejectedByName,
		Reason:            in.Reason,
		WorkDates:         in.Dates,
		Stage:             models.StageNew,
		Status:            "open",
	}
	if err := s.repo.CreateTicket(ticket); err != nil {
		return err
	}
	if s.supportNtfy != nil {
		s.supportNtfy.Notify(SupportTicketInfo{
			Type:        "Rechazo de horas",
			Requester:   in.ProfessionalName,
			Company:     in.CompanyName,
			Subject:     ticket.Title,
			Description: ticket.Description,
			Reason:      in.Reason,
			Link:        fmt.Sprintf("/tickets/internal/%d", ticket.ID),
		})
	}
	return nil
}

func (s *ticketService) CreateInductionFailureAlert(in InductionAlertInput) error {
	pid := in.ProfessionalID
	reason := fmt.Sprintf("Obtuvo %.0f%% y el mínimo aprobatorio es %d%%. Agotó sus %d intentos.",
		in.Score, in.PassingScore, in.Attempts)
	ticket := &models.Ticket{
		Origin:            models.OriginInternal,
		UserID:            &pid,
		Title:             "Inducción no aprobada: " + in.ProfessionalName,
		Description:       "El profesional no alcanzó el mínimo aprobatorio de la inducción y su acceso quedó bloqueado. " + reason + " Contactar para acompañarlo y, si corresponde, reiniciar sus intentos.",
		ProfessionalEmail: in.ProfessionalEmail,
		ProfessionalPhone: in.ProfessionalPhone,
		CompanyName:       in.CompanyName,
		Reason:            reason,
		Stage:             models.StageNew,
		Status:            "open",
	}
	if err := s.repo.CreateTicket(ticket); err != nil {
		return err
	}
	if s.supportNtfy != nil {
		s.supportNtfy.Notify(SupportTicketInfo{
			Type:        "Inducción no aprobada",
			Requester:   in.ProfessionalName,
			Company:     in.CompanyName,
			Subject:     ticket.Title,
			Description: ticket.Description,
			Reason:      reason,
			Link:        fmt.Sprintf("/tickets/internal/%d", ticket.ID),
		})
	}
	return nil
}

// CreateObersuiteHireAlert abre el ticket de una incorporación llegada desde
// Obersuite. No es una conversación ni una alerta de algo que salió mal: es el
// aviso de que hay alguien nuevo al que acompañar, y existe porque hasta ahora
// la contratación solo se veía en el panel de profesionales, donde no se mira
// salvo que se sepa que hay que mirar.
//
// Si ya hay uno abierto para esa persona no se duplica: una re-contratación no
// es un segundo caso que atender.
func (s *ticketService) CreateObersuiteHireAlert(in ObersuiteHireInput) error {
	if abierto, err := s.repo.FindOpenByUserAndOrigin(in.ProfessionalID, models.OriginObersuite); err == nil && abierto != nil {
		return nil
	}

	pid := in.ProfessionalID
	detalle := "Contratado en Obersuite"
	if in.JobTitle != "" {
		detalle += " como " + in.JobTitle
	}
	if in.CompanyName != "" {
		detalle += " para " + in.CompanyName
	}
	seguimiento := " Revisar su ficha y, si no le llegó, reenviarle la capacitación."
	if !in.InductionSent {
		seguimiento = " No se le envió capacitación (la empresa la tiene desactivada o ya la tenía aprobada); revisar su ficha por si falta algo."
	}

	ticket := &models.Ticket{
		Origin:            models.OriginObersuite,
		UserID:            &pid,
		Title:             "Alta desde Obersuite: " + in.ProfessionalName,
		Description:       detalle + "." + seguimiento,
		ProfessionalEmail: in.ProfessionalEmail,
		ProfessionalPhone: in.ProfessionalPhone,
		CompanyName:       in.CompanyName,
		Stage:             models.StageNew,
		Status:            "open",
	}
	if err := s.repo.CreateTicket(ticket); err != nil {
		return err
	}
	if s.supportNtfy != nil {
		s.supportNtfy.Notify(SupportTicketInfo{
			Type:        "Alta desde Obersuite",
			Requester:   in.ProfessionalName,
			Company:     in.CompanyName,
			Subject:     ticket.Title,
			Description: ticket.Description,
			Link:        fmt.Sprintf("/tickets/internal/%d", ticket.ID),
		})
	}
	return nil
}

// CloseObersuiteHireAlert cierra el ticket de incorporación cuando la persona
// aprueba la capacitación: es el momento en que deja de haber algo que hacer.
// Silencioso si no hay ninguno abierto (altas anteriores a esto, o inducción
// desactivada).
func (s *ticketService) CloseObersuiteHireAlert(userID uint) error {
	ticket, err := s.repo.FindOpenByUserAndOrigin(userID, models.OriginObersuite)
	if err != nil || ticket == nil {
		return nil
	}
	ticket.Stage = models.StageClosed
	ticket.Status = "closed"
	return s.repo.SaveTicket(ticket)
}

// GetInternal returns a single internal alert ticket (with notes/messages).
func (s *ticketService) GetInternal(id uint) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !models.IsLocalOrigin(ticket.Origin) {
		return nil, apperrors.ErrNotFound
	}
	s.enrichInternal(ticket)
	return ticket, nil
}

// AddInternalNote appends a follow-up note to an internal alert ticket.
func (s *ticketService) AddInternalNote(id, agentID uint, content string) (*models.TicketMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, apperrors.ErrInvalidInput
	}
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !models.IsLocalOrigin(ticket.Origin) {
		return nil, apperrors.ErrNotFound
	}
	msg := &models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: models.SenderTypeAgent,
		SenderID:   &agentID,
		Channel:    models.ChannelNote,
		Content:    content,
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// UpdateInternal changes the stage/status of an internal alert ticket. It only
// operates on locally stored internal tickets and never touches Zoho.
func (s *ticketService) UpdateInternal(id uint, stage models.TicketStage, status string) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !models.IsLocalOrigin(ticket.Origin) {
		return nil, apperrors.ErrNotFound
	}
	if stage != "" {
		ticket.Stage = stage
	}
	if status != "" {
		ticket.Status = status
	}
	if err := s.repo.SaveTicket(ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

// ListSupportAgents returns active users who can handle the support inbox
// (customer_success + superadmin) as valid transfer targets.
func (s *ticketService) ListSupportAgents() ([]models.User, error) {
	cs, _, err := s.userRepo.GetAll(string(models.UserTypeCustomerSuccess), "", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}
	sa, _, err := s.userRepo.GetAll(string(models.UserTypeSuperadmin), "", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}
	active := make([]models.User, 0, len(cs)+len(sa))
	for _, u := range append(cs, sa...) {
		if u.IsActive && !isAssignableAgentExcluded(u) {
			active = append(active, u)
		}
	}
	return active, nil
}

// isAssignableAgentExcluded aparta del selector de traspaso a las cuentas que no
// son personas.
//
// El usuario de sistema "Obertrack" es de tipo superadmin para poder publicar
// avisos automáticos, así que entra en cualquier listado de superadmins. Ya se lo
// excluye del selector de chat y de los asignables de Tareas; en el traspaso de
// tickets faltaba, y ofrecía asignarle una conversación a un bot que no la va a
// atender nunca.
func isAssignableAgentExcluded(u models.User) bool {
	return u.Email == models.SystemBotEmail
}

// ListTransfers returns the transfer history for a ticket.
func (s *ticketService) ListTransfers(origin, ref string) ([]models.TicketTransfer, error) {
	return s.repo.ListTransfers(origin, ref)
}

// GetUserName returns a user's display name by id (for audit labels).
func (s *ticketService) GetUserName(id uint) (string, error) {
	u, err := s.userRepo.GetByID(id)
	if err != nil || u == nil {
		return "", apperrors.ErrNotFound
	}
	return u.Name, nil
}

// RecordTransfer persists the audit row, notifies both parties and (for
// internal tickets) appends a system event to the timeline.
func (s *ticketService) RecordTransfer(in TransferInput) error {
	transfer := &models.TicketTransfer{
		Origin:      in.Origin,
		TicketRef:   in.TicketRef,
		TicketTitle: in.TicketTitle,
		FromUserID:  in.FromUserID,
		FromName:    in.FromName,
		ToUserID:    in.ToUserID,
		ToName:      in.ToName,
		ByUserID:    in.ByUserID,
		ByName:      in.ByName,
		Reason:      in.Reason,
	}
	if err := s.repo.CreateTransfer(transfer); err != nil {
		return err
	}

	if in.AddTimelineEvent && in.LocalTicketID > 0 {
		content := fmt.Sprintf("Ticket traspasado a %s por %s.", in.ToName, in.ByName)
		if in.FromName != "" {
			content = fmt.Sprintf("Ticket traspasado de %s a %s por %s.", in.FromName, in.ToName, in.ByName)
		}
		_ = s.repo.CreateMessage(&models.TicketMessage{
			TicketID:   in.LocalTicketID,
			SenderType: models.SenderTypeSystem,
			Channel:    models.ChannelNote,
			Content:    content,
		})
	}

	if s.notifSvc != nil {
		// Internal tickets have a detail page; external ones land on the board.
		link := "/tickets"
		if in.LocalTicketID > 0 && models.IsLocalOrigin(in.Origin) {
			link = fmt.Sprintf("/tickets/internal/%d", in.LocalTicketID)
		}
		data := map[string]interface{}{"ticket": in.TicketTitle, "origin": in.Origin, "ref": in.TicketRef, "link": link}
		if in.ToUserID != nil {
			_ = s.notifSvc.CreateNotification(*in.ToUserID, "ticket_transfer",
				"Ticket asignado a ti",
				fmt.Sprintf("%s te traspasó el ticket \"%s\".", in.ByName, in.TicketTitle), data)
		}
		if in.FromUserID != nil && (in.ToUserID == nil || *in.FromUserID != *in.ToUserID) {
			_ = s.notifSvc.CreateNotification(*in.FromUserID, "ticket_transfer",
				"Ticket traspasado",
				fmt.Sprintf("%s traspasó el ticket \"%s\" a %s.", in.ByName, in.TicketTitle, in.ToName), data)
		}
	}
	return nil
}

// TransferInternal reassigns an internal alert ticket and audits it.
func (s *ticketService) TransferInternal(id, toUserID, byUserID uint, isSuperadmin bool, reason string) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil || !models.IsLocalOrigin(ticket.Origin) {
		return nil, apperrors.ErrNotFound
	}
	// Permission: superadmin always; otherwise current owner, or anyone if unassigned.
	if !isSuperadmin && ticket.AssignedTo != nil && *ticket.AssignedTo != byUserID {
		return nil, apperrors.ErrAccessDenied
	}

	var fromUserID *uint
	fromName := ""
	if ticket.AssignedTo != nil {
		fromUserID = ticket.AssignedTo
		if from, err := s.userRepo.GetByID(*ticket.AssignedTo); err == nil && from != nil {
			fromName = from.Name
		}
	}

	target, err := s.userRepo.GetByID(toUserID)
	if err != nil || target == nil {
		return nil, apperrors.ErrInvalidInput
	}
	byName := ""
	if by, err := s.userRepo.GetByID(byUserID); err == nil && by != nil {
		byName = by.Name
	}

	tid := toUserID
	ticket.AssignedTo = &tid
	if err := s.repo.SaveTicket(ticket); err != nil {
		return nil, err
	}

	_ = s.RecordTransfer(TransferInput{
		Origin:           models.OriginInternal,
		TicketRef:        strconv.FormatUint(uint64(ticket.ID), 10),
		TicketTitle:      ticket.Title,
		FromUserID:       fromUserID,
		FromName:         fromName,
		ToUserID:         &tid,
		ToName:           target.Name,
		ByUserID:         byUserID,
		ByName:           byName,
		Reason:           reason,
		AddTimelineEvent: true,
		LocalTicketID:    ticket.ID,
	})

	return s.repo.GetByID(ticket.ID)
}

// WhatsAppAssign traspasa la conversación a otro agente.
//
// Hasta ahora un chat de WhatsApp solo se podía tomar, resolver o reabrir: para
// pasárselo a un compañero había que reabrirlo y pedirle que lo tomara él, con
// lo que el hilo perdía el responsable por el medio. Esto lo hace en un paso y
// con el mismo rastro que un ticket interno.
//
// Permisos, calcados de TransferInternal: un superadmin siempre; si no, el
// responsable actual, o cualquiera si el chat todavía no tiene dueño.
func (s *ticketService) WhatsAppAssign(id, actorID, assigneeID uint, isSuperadmin bool, reason string) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil || ticket.Origin != string(models.ChannelWhatsApp) {
		return nil, apperrors.ErrNotFound
	}
	if ticket.Session != s.wahaSvc.GetSession() {
		return nil, apperrors.ErrNotFound
	}
	if !isSuperadmin && ticket.AssignedTo != nil && *ticket.AssignedTo != actorID {
		return nil, apperrors.ErrAccessDenied
	}

	target, err := s.userRepo.GetByID(assigneeID)
	if err != nil || target == nil {
		return nil, apperrors.ErrInvalidInput
	}

	var fromUserID *uint
	fromName := ""
	if ticket.AssignedTo != nil {
		fromUserID = ticket.AssignedTo
		if from, ferr := s.userRepo.GetByID(*ticket.AssignedTo); ferr == nil && from != nil {
			fromName = from.Name
		}
	}
	byName := ""
	if by, berr := s.userRepo.GetByID(actorID); berr == nil && by != nil {
		byName = by.Name
	}

	tid := assigneeID
	ticket.AssignedTo = &tid
	// Reasignar implica que alguien lo está atendiendo: si venía sin tomar, pasa
	// a en curso en vez de quedarse en "Nuevo" con dueño, que es contradictorio.
	if ticket.Stage == models.StageNew {
		ticket.Stage = models.StageInProgress
	}
	// Se limpian las asociaciones precargadas antes de guardar: con ellas puestas,
	// Save arrastra el contacto y los mensajes y puede pisar sus claves foráneas.
	ticket.Contact = nil
	ticket.Assignee = nil
	ticket.Messages = nil
	if err := s.repo.SaveTicket(ticket); err != nil {
		return nil, err
	}

	_ = s.RecordTransfer(TransferInput{
		Origin:      string(models.ChannelWhatsApp),
		TicketRef:   strconv.FormatUint(uint64(ticket.ID), 10),
		TicketTitle: ticket.Title,
		FromUserID:  fromUserID,
		FromName:    fromName,
		ToUserID:    &tid,
		ToName:      target.Name,
		ByUserID:    actorID,
		ByName:      byName,
		Reason:      reason,
		// Sin evento en el hilo: aquí el hilo ES la conversación con el cliente, y
		// un mensaje de sistema se colaría entre lo que escribió el contacto (la
		// bandeja pinta como entrante todo lo que no sea del agente). El rastro
		// queda en la bitácora de traspasos y en el aviso al nuevo responsable.
		AddTimelineEvent: false,
	})

	return s.repo.GetByID(ticket.ID)
}

// broadcastTicketMessage notifies connected clients of a new ticket message.
func broadcastTicketMessage(ticketID uint, msg *models.TicketMessage) {
	websocket.GlobalNotifHub.BroadcastToAll("new_ticket_message", map[string]interface{}{
		"ticket_id": ticketID,
		"message":   msg,
	})
}
