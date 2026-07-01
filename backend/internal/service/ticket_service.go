package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
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

	IngestWhatsApp(session, from, body, externalID string) error
	IngestEmail(fromEmail, fromName, subject, textBody, messageID string) error

	// ListInternal returns Obertrack-generated alert tickets (origin = internal).
	ListInternal() ([]models.Ticket, error)
	// GetInternal returns a single internal alert ticket (with notes).
	GetInternal(id uint) (*models.Ticket, error)
	// ListInternalReport returns internal alerts created within [start, end].
	ListInternalReport(start, end time.Time) ([]models.Ticket, error)
	// CreateWorkHourRejectionAlert opens an internal support alert when a
	// professional's work hours are rejected.
	CreateWorkHourRejectionAlert(in RejectionAlertInput) error
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

type ticketService struct {
	repo        repository.TicketRepository
	userRepo    repository.UserRepository
	notifSvc    NotificationService
	wahaSvc     *WahaService
	brevoSvc    *BrevoService
	supportNtfy *SupportNotifier
}

func NewTicketService(repo repository.TicketRepository, userRepo repository.UserRepository, notifSvc NotificationService, wahaSvc *WahaService, brevoSvc *BrevoService, supportNtfy *SupportNotifier) TicketService {
	return &ticketService{repo: repo, userRepo: userRepo, notifSvc: notifSvc, wahaSvc: wahaSvc, brevoSvc: brevoSvc, supportNtfy: supportNtfy}
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
	if t == nil || t.Origin != models.OriginInternal {
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
	if (t.WorkDates == "" || t.Reason == "") && t.Description != "" {
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
	switch channel {
	case models.ChannelWhatsApp:
		if ticket.Contact == nil {
			return nil, apperrors.ErrExternalSend
		}
		session := s.wahaSvc.GetSession()
		if err := s.wahaSvc.SendMessage(session, ticket.Contact.Phone, content); err != nil {
			return nil, apperrors.ErrExternalSend
		}
	case models.ChannelEmail:
		if ticket.Contact == nil {
			return nil, apperrors.ErrExternalSend
		}
		if err := s.brevoSvc.SendEmail(ticket.Contact.Email, ticket.Contact.Name, ticket.Title, content); err != nil {
			return nil, apperrors.ErrExternalSend
		}
	}

	msg := &models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: models.SenderTypeAgent,
		SenderID:   &agentID,
		Channel:    channel,
		Content:    content,
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// IngestWhatsApp handles an inbound WhatsApp message: resolve/create the contact,
// attach it to a recent open ticket (or open a new one), persist the message and
// broadcast it.
func (s *ticketService) IngestWhatsApp(session, from, body, externalID string) error {
	phone := from
	if i := strings.IndexByte(from, '@'); i >= 0 {
		phone = from[:i]
	}

	resolvedName := "WA User " + phone
	if contact, err := s.wahaSvc.GetContact(session, from); err == nil && contact != nil {
		if contact.Name != "" {
			resolvedName = contact.Name
		}
		if contact.Phone != "" {
			phone = contact.Phone
		}
	}

	contact, err := s.repo.GetContactByPhone(phone)
	if err != nil {
		contact = &models.Contact{Phone: phone, Name: resolvedName}
		if err := s.repo.CreateContact(contact); err != nil {
			return err
		}
	} else if contact.Name == "WA User "+phone && resolvedName != "WA User "+phone {
		contact.Name = resolvedName
		_ = s.repo.SaveContact(contact)
	}

	ticket, err := s.repo.GetOpenTicketByContactSince(contact.ID, time.Now().Add(-1*time.Hour))
	if err != nil {
		ticket = &models.Ticket{
			ContactID: &contact.ID,
			Origin:    string(models.ChannelWhatsApp),
			Title:     "WA: " + phone,
			Stage:     models.StageNew,
			Status:    "open",
		}
		if err := s.repo.CreateTicket(ticket); err != nil {
			return err
		}
		if s.supportNtfy != nil {
			s.supportNtfy.Notify(SupportTicketInfo{
				Type:        "WhatsApp",
				Requester:   resolvedName,
				Subject:     ticket.Title,
				Description: body,
				Link:        "/tickets",
			})
		}
	}

	msg := &models.TicketMessage{
		TicketID:   ticket.ID,
		SenderType: models.SenderTypeContact,
		Channel:    models.ChannelWhatsApp,
		Content:    body,
		ExternalID: externalID,
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return err
	}
	_ = s.repo.TouchTicket(ticket)

	broadcastTicketMessage(ticket.ID, msg)
	return nil
}

// IngestEmail handles an inbound email: resolve/create the contact, attach to an
// open ticket (or open a new one), persist the message and broadcast it.
func (s *ticketService) IngestEmail(fromEmail, fromName, subject, textBody, messageID string) error {
	if fromEmail == "" {
		return apperrors.ErrInvalidInput
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

	ticket, err := s.repo.GetOpenTicketByContact(contact.ID)
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
	if err := s.repo.CreateMessage(msg); err != nil {
		return err
	}

	broadcastTicketMessage(ticket.ID, msg)
	return nil
}

// ListInternal returns Obertrack-generated alert tickets (origin = internal).
func (s *ticketService) ListInternal() ([]models.Ticket, error) {
	tickets, err := s.repo.ListByOrigin(models.OriginInternal)
	if err != nil {
		return nil, err
	}
	for i := range tickets {
		s.enrichInternal(&tickets[i])
	}
	return tickets, nil
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

// GetInternal returns a single internal alert ticket (with notes/messages).
func (s *ticketService) GetInternal(id uint) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if ticket.Origin != models.OriginInternal {
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
	if ticket.Origin != models.OriginInternal {
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
	if ticket.Origin != models.OriginInternal {
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
		if u.IsActive {
			active = append(active, u)
		}
	}
	return active, nil
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
		if in.LocalTicketID > 0 && in.Origin == string(models.OriginInternal) {
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
	if err != nil || ticket.Origin != models.OriginInternal {
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

// broadcastTicketMessage notifies connected clients of a new ticket message.
func broadcastTicketMessage(ticketID uint, msg *models.TicketMessage) {
	websocket.GlobalNotifHub.BroadcastToAll("new_ticket_message", map[string]interface{}{
		"ticket_id": ticketID,
		"message":   msg,
	})
}
