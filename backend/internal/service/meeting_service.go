package service

import (
	"errors"
	"fmt"
	"log"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

var (
	// ErrMeetingValidation envuelve los errores de datos del convocante, para que
	// el handler los devuelva como 400 y no como 500.
	ErrMeetingValidation = errors.New("datos de la sesión inválidos")
	// ErrMeetingForbidden: la sesión existe pero el usuario no puede tocarla.
	ErrMeetingForbidden = errors.New("no tienes acceso a esta sesión")
	// ErrGoogleNotConnected: el organizador no ha vinculado su cuenta. No es un
	// fallo del sistema sino el estado normal de quien nunca conectó Google, y la
	// UI lo convierte en el CTA de conexión.
	ErrGoogleNotConnected = errors.New("conecta tu cuenta de Google para convocar sesiones")
)

// MeetingInput son los datos con los que se convoca o edita una sesión.
type MeetingInput struct {
	Title       string
	Description string
	StartAt     time.Time
	EndAt       time.Time
	TimeZone    string
	// AttendeeUserIDs son invitados con cuenta en Obertrack; AttendeeEmails son
	// externos (clientes, candidatos) de los que solo se conoce el correo.
	AttendeeUserIDs []uint
	AttendeeEmails  []string
	TaskID          *uint
	RecurrenceRule  string
}

// MeetingService convoca reuniones con sala de Google Meet.
//
// A diferencia de la sincronización de tareas —que va por una cola con
// reintentos— aquí las llamadas a Google son SÍNCRONAS. Es deliberado: quien
// convoca necesita el enlace de Meet en el momento para poder compartirlo, y no
// se le puede responder "tu sesión existirá dentro de veinte segundos". La
// contrapartida (un fallo de red pierde la operación en vez de reintentarla) es
// asumible porque hay una persona mirando que puede volver a intentarlo, y el
// error que ve es el tipado de siempre: needs_reauth → reconectar, permanente →
// el mensaje de Google.
type MeetingService interface {
	Create(organizerID, tenantID uint, in MeetingInput) (*models.MeetingSession, error)
	Update(sessionID, actorID uint, in MeetingInput) (*models.MeetingSession, error)
	Cancel(sessionID, actorID uint) error
	Get(sessionID, actorID uint) (*models.MeetingSession, error)
	List(tenantID, userID uint, past bool, taskID uint) ([]models.MeetingSession, error)
	Upcoming(tenantID, userID uint, limit int) ([]models.MeetingSession, error)
	// Presence dice quién está conectado ahora mismo a la sala de una sesión.
	Presence(sessionID, actorID uint) (*MeetPresence, error)
	// SetSystemDM cablea el emisor de DMs del bot "Obertrack". Callback inyectado
	// —mismo patrón que taskService— para no acoplar meeting→channel. Sin cablear
	// (nil): solo se envía la notificación de campanita.
	SetSystemDM(fn func(recipientID uint, content string))
}

type meetingService struct {
	repo       repository.MeetingRepository
	userRepo   repository.UserRepository
	googleRepo repository.GoogleCalendarRepository
	google     GoogleCalendarService
	notifSvc   NotificationService
	// postSystemDM publica el DM del bot con el enlace. Inyectado igual que en
	// taskService; nil = sin DMs.
	postSystemDM func(recipientID uint, content string)
}

func NewMeetingService(
	repo repository.MeetingRepository,
	userRepo repository.UserRepository,
	googleRepo repository.GoogleCalendarRepository,
	google GoogleCalendarService,
	notifSvc NotificationService,
) MeetingService {
	return &meetingService{
		repo:       repo,
		userRepo:   userRepo,
		googleRepo: googleRepo,
		google:     google,
		notifSvc:   notifSvc,
	}
}

func (s *meetingService) SetSystemDM(fn func(recipientID uint, content string)) {
	s.postSystemDM = fn
}

// --- Creación ---

func (s *meetingService) Create(organizerID, tenantID uint, in MeetingInput) (*models.MeetingSession, error) {
	if err := s.validate(in); err != nil {
		return nil, err
	}
	account, err := s.organizerAccount(organizerID)
	if err != nil {
		return nil, err
	}

	attendees, err := s.resolveAttendees(in, organizerID)
	if err != nil {
		return nil, err
	}
	rule, seriesEndsAt, err := s.seriesFields(in)
	if err != nil {
		return nil, err
	}

	event, err := s.google.CreateEvent(organizerID, account.CalendarID, CalendarEventInput{
		Summary:          strings.TrimSpace(in.Title),
		Description:      s.eventDescription(in.Description),
		StartAt:          in.StartAt,
		EndAt:            in.EndAt,
		TimeZone:         in.TimeZone,
		Attendees:        emailsOf(attendees),
		CreateConference: true,
		Recurrence:       recurrenceOf(rule),
	})
	if err != nil {
		return nil, err
	}

	// La sala puede tardar un instante en existir. Se resuelve aquí y no se deja
	// para después: una sesión sin enlace no sirve para nada al convocante.
	meetURL := event.MeetURL
	if meetURL == "" && event.ConferencePending {
		meetURL = s.resolvePendingMeetURL(organizerID, account.CalendarID, event.ID)
	}

	session := &models.MeetingSession{
		TenantID:       tenantID,
		Title:          strings.TrimSpace(in.Title),
		Description:    strings.TrimSpace(in.Description),
		StartAt:        in.StartAt.UTC(),
		EndAt:          in.EndAt.UTC(),
		TimeZone:       in.TimeZone,
		OrganizerID:    organizerID,
		GoogleEventID:  event.ID,
		CalendarID:     account.CalendarID,
		MeetURL:        meetURL,
		HTMLLink:       event.HTMLLink,
		Status:         models.MeetingStatusScheduled,
		TaskID:         in.TaskID,
		RecurrenceRule: rule,
		SeriesEndsAt:   seriesEndsAt,
		Attendees:      attendees,
	}
	if err := s.repo.Create(session); err != nil {
		// El evento ya existe en Google pero no se pudo guardar: se deshace para
		// no dejar una reunión fantasma en el calendario del organizador que
		// Obertrack no sabe ni que existe.
		if delErr := s.google.DeleteEvent(organizerID, account.CalendarID, event.ID); delErr != nil {
			log.Printf("[meetings] no se pudo revertir el evento %s tras fallar el guardado: %v", event.ID, delErr)
		}
		return nil, err
	}

	s.notifyAttendees(session, "meeting_invite", "Nueva sesión", "te invitó a")

	// Se relee para devolverla con sus asociaciones ya cargadas, y se decora para
	// que la respuesta lleve la próxima ocurrencia igual que el listado.
	saved, err := s.repo.GetByID(session.ID)
	if err != nil {
		return nil, err
	}
	fillNextOccurrence(saved, time.Now())
	return saved, nil
}

// --- Edición ---

func (s *meetingService) Update(sessionID, actorID uint, in MeetingInput) (*models.MeetingSession, error) {
	session, err := s.repo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session.OrganizerID != actorID {
		return nil, ErrMeetingForbidden
	}
	if session.Status == models.MeetingStatusCancelled {
		return nil, fmt.Errorf("%w: la sesión ya fue cancelada", ErrMeetingValidation)
	}
	if err := s.validate(in); err != nil {
		return nil, err
	}

	attendees, err := s.resolveAttendees(in, session.OrganizerID)
	if err != nil {
		return nil, err
	}
	rule, seriesEndsAt, err := s.seriesFields(in)
	if err != nil {
		return nil, err
	}

	input := CalendarEventInput{
		Summary:     strings.TrimSpace(in.Title),
		Description: s.eventDescription(in.Description),
		StartAt:     in.StartAt,
		EndAt:       in.EndAt,
		TimeZone:    in.TimeZone,
		Attendees:   emailsOf(attendees),
		Recurrence:  recurrenceOf(rule),
	}

	// Si el organizador borró el evento a mano en Google, se re-crea en vez de
	// dejar la sesión sin reflejo — mismo criterio que processUpsert en la
	// sincronización de tareas.
	err = s.google.UpdateEvent(session.OrganizerID, session.CalendarID, session.GoogleEventID, input)
	if errors.Is(err, ErrEventGone) {
		input.CreateConference = true
		var event *CalendarEvent
		event, err = s.google.CreateEvent(session.OrganizerID, session.CalendarID, input)
		if err == nil {
			session.GoogleEventID = event.ID
			session.HTMLLink = event.HTMLLink
			session.MeetURL = event.MeetURL
			if session.MeetURL == "" && event.ConferencePending {
				session.MeetURL = s.resolvePendingMeetURL(session.OrganizerID, session.CalendarID, event.ID)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"title":           strings.TrimSpace(in.Title),
		"description":     strings.TrimSpace(in.Description),
		"start_at":        in.StartAt.UTC(),
		"end_at":          in.EndAt.UTC(),
		"time_zone":       in.TimeZone,
		"task_id":         in.TaskID,
		"recurrence_rule": rule,
		"series_ends_at":  seriesEndsAt,
		"google_event_id": session.GoogleEventID,
		"meet_url":        session.MeetURL,
		"html_link":       session.HTMLLink,
	}
	if err := s.repo.UpdateFields(sessionID, updates); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceAttendees(sessionID, attendees); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	fillNextOccurrence(updated, time.Now())
	s.notifyAttendees(updated, "meeting_updated", "Sesión actualizada", "actualizó")
	return updated, nil
}

// --- Cancelación ---

func (s *meetingService) Cancel(sessionID, actorID uint) error {
	session, err := s.repo.GetByID(sessionID)
	if err != nil {
		return err
	}
	if session.OrganizerID != actorID {
		return ErrMeetingForbidden
	}
	if session.Status == models.MeetingStatusCancelled {
		return nil // ya está en el estado pedido.
	}

	if session.GoogleEventID != "" {
		// DeleteEvent ya es idempotente ante 404/410, así que un evento borrado a
		// mano no impide cancelar.
		if err := s.google.DeleteEvent(session.OrganizerID, session.CalendarID, session.GoogleEventID); err != nil {
			return err
		}
	}

	if err := s.repo.UpdateFields(sessionID, map[string]interface{}{
		"status": models.MeetingStatusCancelled,
	}); err != nil {
		return err
	}

	session.Status = models.MeetingStatusCancelled
	s.notifyAttendees(session, "meeting_cancelled", "Sesión cancelada", "canceló")
	return nil
}

// --- Consulta ---

func (s *meetingService) Get(sessionID, actorID uint) (*models.MeetingSession, error) {
	session, err := s.repo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.IsParticipant(sessionID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrMeetingForbidden
	}

	// Rescate del enlace que quedó pendiente al crear: se resuelve la primera vez
	// que alguien abre la sesión, en vez de dejarla sin enlace para siempre.
	if session.MeetURL == "" && session.GoogleEventID != "" && session.Status == models.MeetingStatusScheduled {
		if url := s.resolvePendingMeetURL(session.OrganizerID, session.CalendarID, session.GoogleEventID); url != "" {
			session.MeetURL = url
			if err := s.repo.UpdateFields(sessionID, map[string]interface{}{"meet_url": url}); err != nil {
				log.Printf("[meetings] no se pudo guardar el enlace resuelto de la sesión %d: %v", sessionID, err)
			}
		}
	}
	fillNextOccurrence(session, time.Now())
	return session, nil
}

func (s *meetingService) List(tenantID, userID uint, past bool, taskID uint) ([]models.MeetingSession, error) {
	sessions, err := s.repo.List(repository.MeetingFilter{
		TenantID: tenantID,
		UserID:   userID,
		Past:     past,
		TaskID:   taskID,
	})
	if err != nil {
		return nil, err
	}
	return decorate(sessions), nil
}

func (s *meetingService) Upcoming(tenantID, userID uint, limit int) ([]models.MeetingSession, error) {
	if limit <= 0 {
		limit = 3
	}
	// El recorte se hace DESPUÉS de ordenar por próxima ocurrencia, no con un
	// LIMIT en SQL: una serie diaria creada hace un mes tiene un start_at viejo,
	// así que el orden de la consulta la pondría al principio aunque su próxima
	// reunión sea la más lejana. Son pocas filas por empresa, así que traerlas
	// todas y recortar en memoria sale más barato que equivocarse.
	sessions, err := s.repo.List(repository.MeetingFilter{
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}
	sessions = decorate(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

// Presence consulta la sala con la credencial del ORGANIZADOR, no la de quien
// pregunta: la API de Meet exige ser dueño de la sala o estar dentro, y un
// invitado que aún no ha entrado no cumple ninguna de las dos. Como el permiso
// para ver la sesión ya se comprobó, usar la credencial del organizador no
// filtra nada que el que pregunta no pudiera ver entrando.
func (s *meetingService) Presence(sessionID, actorID uint) (*MeetPresence, error) {
	session, err := s.repo.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.IsParticipant(sessionID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrMeetingForbidden
	}
	// Sin sala (o cancelada) no hay nada que consultar, y preguntarlo solo
	// gastaría cuota.
	if session.MeetURL == "" || session.Status != models.MeetingStatusScheduled {
		return &MeetPresence{Live: false}, nil
	}

	presence, err := s.google.MeetPresence(session.OrganizerID, session.MeetURL)
	// La sala no existe todavía porque nadie ha entrado nunca: eso es una sala
	// vacía, no un fallo.
	if errors.Is(err, ErrEventGone) {
		return &MeetPresence{Live: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return presence, nil
}

// --- Helpers ---

func (s *meetingService) validate(in MeetingInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return fmt.Errorf("%w: el título es obligatorio", ErrMeetingValidation)
	}
	if in.StartAt.IsZero() {
		return fmt.Errorf("%w: falta la fecha y hora de inicio", ErrMeetingValidation)
	}
	if !in.EndAt.After(in.StartAt) {
		return fmt.Errorf("%w: la hora de fin debe ser posterior a la de inicio", ErrMeetingValidation)
	}
	if strings.TrimSpace(in.TimeZone) == "" {
		return fmt.Errorf("%w: falta la zona horaria", ErrMeetingValidation)
	}
	// La zona debe existir en la base de datos IANA: una zona inventada haría
	// que Google rechazara el evento con un 400 opaco.
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return fmt.Errorf("%w: zona horaria desconocida (%s)", ErrMeetingValidation, in.TimeZone)
	}
	if len(in.AttendeeUserIDs) == 0 && len(in.AttendeeEmails) == 0 {
		return fmt.Errorf("%w: invita al menos a una persona", ErrMeetingValidation)
	}
	for _, email := range in.AttendeeEmails {
		if _, err := mail.ParseAddress(strings.TrimSpace(email)); err != nil {
			return fmt.Errorf("%w: el correo %q no es válido", ErrMeetingValidation, email)
		}
	}
	// La regla se valida aquí y no al pintarla: una RRULE que no sepamos calcular
	// dejaría la sesión sin próxima ocurrencia y desaparecería del listado sin
	// que nadie entendiera por qué.
	if _, err := ParseRecurrence(in.RecurrenceRule); err != nil {
		return err
	}
	return nil
}

// seriesFields calcula lo que hay que guardar de la recurrencia: la regla ya
// normalizada y el fin de la serie (NULL si no termina).
func (s *meetingService) seriesFields(in MeetingInput) (rule string, endsAt *time.Time, err error) {
	rec, err := ParseRecurrence(in.RecurrenceRule)
	if err != nil {
		return "", nil, err
	}
	loc, locErr := time.LoadLocation(in.TimeZone)
	if locErr != nil {
		loc = time.UTC
	}
	return rec.String(), rec.SeriesEnd(in.StartAt, in.EndAt.Sub(in.StartAt), loc), nil
}

// decorate rellena la próxima ocurrencia de cada sesión y ordena por ella. Sin
// esto, una serie diaria se mostraría siempre con la fecha de su primer día.
func decorate(sessions []models.MeetingSession) []models.MeetingSession {
	now := time.Now()
	for i := range sessions {
		fillNextOccurrence(&sessions[i], now)
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		a, b := sessions[i].NextStartAt, sessions[j].NextStartAt
		// Las series agotadas (sin próxima ocurrencia) van al final.
		if a == nil || b == nil {
			return a != nil
		}
		return a.Before(*b)
	})
	return sessions
}

func fillNextOccurrence(session *models.MeetingSession, now time.Time) {
	rec, err := ParseRecurrence(session.RecurrenceRule)
	if err != nil {
		// Una regla ilegible en BD no debe romper el listado: se trata como
		// sesión única, que es como se comportaba antes de la recurrencia.
		log.Printf("[meetings] regla de repetición ilegible en la sesión %d: %v", session.ID, err)
		rec = nil
	}
	duration := session.EndAt.Sub(session.StartAt)
	start, ok := rec.NextOccurrence(session.StartAt, duration, sessionLocation(session), now)
	if !ok {
		return
	}
	end := start.Add(duration)
	session.NextStartAt = &start
	session.NextEndAt = &end
}

// organizerAccount comprueba que quien convoca tenga Google conectado y en uso.
func (s *meetingService) organizerAccount(organizerID uint) (*models.GoogleCalendarAccount, error) {
	if !s.google.Enabled() {
		return nil, ErrGoogleDisabled
	}
	account, err := s.googleRepo.GetByUser(organizerID)
	if errors.Is(err, repository.ErrGoogleAccountNotFound) {
		return nil, ErrGoogleNotConnected
	}
	if err != nil {
		return nil, err
	}
	if account.Status == models.GoogleCalStatusNeedsReauth {
		return nil, ErrNeedsReauth
	}
	return account, nil
}

// resolveAttendees convierte los ids internos y los correos externos en una
// lista única. El organizador se excluye: Google ya lo añade como dueño del
// evento, y volver a listarlo duplicaría la entrada en el calendario.
func (s *meetingService) resolveAttendees(in MeetingInput, organizerID uint) ([]models.MeetingAttendee, error) {
	seen := map[string]bool{}
	var attendees []models.MeetingAttendee

	for _, userID := range in.AttendeeUserIDs {
		if userID == organizerID {
			continue
		}
		user, err := s.userRepo.GetByID(userID)
		if err != nil {
			return nil, fmt.Errorf("%w: el usuario %d no existe", ErrMeetingValidation, userID)
		}
		email := strings.ToLower(strings.TrimSpace(user.Email))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		id := user.ID
		attendees = append(attendees, models.MeetingAttendee{
			UserID: &id,
			Email:  email,
			Name:   user.Name,
		})
	}

	for _, raw := range in.AttendeeEmails {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		// Un "externo" cuyo correo sí existe en Obertrack se trata como interno:
		// así recibe también campanita y DM en vez de solo el correo de Google.
		if user, err := s.userRepo.GetByEmail(email); err == nil && user != nil {
			if user.ID == organizerID {
				continue
			}
			id := user.ID
			attendees = append(attendees, models.MeetingAttendee{UserID: &id, Email: email, Name: user.Name})
			continue
		}
		attendees = append(attendees, models.MeetingAttendee{Email: email})
	}

	// Orden estable para que el diff entre ediciones sea comparable y la lista no
	// baile en la UI.
	sort.Slice(attendees, func(i, j int) bool { return attendees[i].Email < attendees[j].Email })
	return attendees, nil
}

// eventDescription firma el evento para que en Google se sepa de dónde salió.
func (s *meetingService) eventDescription(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return "— Convocada desde Obertrack"
	}
	return description + "\n\n— Convocada desde Obertrack"
}

// resolvePendingMeetURL vuelve a consultar el evento para obtener el enlace que
// Google todavía no había terminado de crear. Best-effort: si sigue sin estar,
// se devuelve vacío y se reintenta la próxima vez que alguien abra la sesión.
func (s *meetingService) resolvePendingMeetURL(userID uint, calendarID, eventID string) string {
	event, err := s.google.GetEvent(userID, calendarID, eventID)
	if err != nil {
		log.Printf("[meetings] no se pudo resolver el enlace de Meet del evento %s: %v", eventID, err)
		return ""
	}
	return event.MeetURL
}

// notifyAttendees avisa DENTRO de Obertrack a los invitados con cuenta: campanita
// y DM del bot con el enlace. A los externos ya los invitó Google por correo
// (sendUpdates=all), que es el único canal que tenemos con ellos.
func (s *meetingService) notifyAttendees(session *models.MeetingSession, notifType, title, verb string) {
	organizerName := "Alguien"
	if organizer, err := s.userRepo.GetByID(session.OrganizerID); err == nil && organizer != nil {
		organizerName = organizer.Name
	}

	when := session.StartAt.In(sessionLocation(session)).Format("02/01/2006 15:04")
	message := fmt.Sprintf("%s %s la sesión «%s» del %s", organizerName, verb, session.Title, when)

	for _, attendee := range session.Attendees {
		if attendee.UserID == nil {
			continue
		}
		data := map[string]interface{}{"meeting_id": session.ID}
		if session.MeetURL != "" {
			data["meet_url"] = session.MeetURL
		}
		if err := s.notifSvc.CreateNotification(*attendee.UserID, notifType, title, message, data); err != nil {
			log.Printf("[meetings] no se pudo notificar al usuario %d: %v", *attendee.UserID, err)
		}
		s.sendDM(*attendee.UserID, session, message)
	}
}

func (s *meetingService) sendDM(userID uint, session *models.MeetingSession, message string) {
	if s.postSystemDM == nil {
		return
	}
	content := "📅 " + message
	if session.MeetURL != "" && session.Status == models.MeetingStatusScheduled {
		content += "\n" + session.MeetURL
	}
	s.postSystemDM(userID, content)
}

// sessionLocation resuelve la zona de la sesión para mostrar la hora como la
// convocó el organizador. Si la zona guardada ya no existe, cae a UTC antes que
// romper el aviso.
func sessionLocation(session *models.MeetingSession) *time.Location {
	loc, err := time.LoadLocation(session.TimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func emailsOf(attendees []models.MeetingAttendee) []string {
	emails := make([]string, 0, len(attendees))
	for _, a := range attendees {
		emails = append(emails, a.Email)
	}
	return emails
}

func recurrenceOf(rule string) []string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil
	}
	return []string{rule}
}
