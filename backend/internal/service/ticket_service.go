package service

import (
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
}

type ticketService struct {
	repo     repository.TicketRepository
	wahaSvc  *WahaService
	brevoSvc *BrevoService
}

func NewTicketService(repo repository.TicketRepository, wahaSvc *WahaService, brevoSvc *BrevoService) TicketService {
	return &ticketService{repo: repo, wahaSvc: wahaSvc, brevoSvc: brevoSvc}
}

// canAccess returns true if the caller may view/act on the ticket.
func (s *ticketService) canAccess(t *models.Ticket, requesterID uint, userType string) bool {
	if userType == string(models.UserTypeSuperadmin) || userType == string(models.UserTypeCustomerSuccess) {
		return true
	}
	return t.AssignedTo != nil && *t.AssignedTo == requesterID
}

func (s *ticketService) List(requesterID uint, userType string) ([]models.Ticket, error) {
	if userType == string(models.UserTypeSuperadmin) || userType == string(models.UserTypeCustomerSuccess) {
		return s.repo.List(nil)
	}
	return s.repo.List(&requesterID)
}

func (s *ticketService) Get(id, requesterID uint, userType string) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !s.canAccess(ticket, requesterID, userType) {
		return nil, apperrors.ErrAccessDenied
	}
	return ticket, nil
}

func (s *ticketService) Update(id, requesterID uint, userType string, stage models.TicketStage, status string, assignedTo *uint) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if !s.canAccess(ticket, requesterID, userType) {
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
	if !s.canAccess(ticket, agentID, userType) {
		return nil, apperrors.ErrAccessDenied
	}

	// Send the outbound message via the appropriate integration.
	switch channel {
	case models.ChannelWhatsApp:
		session := s.wahaSvc.GetSession()
		if err := s.wahaSvc.SendMessage(session, ticket.Contact.Phone, content); err != nil {
			return nil, apperrors.ErrExternalSend
		}
	case models.ChannelEmail:
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
			ContactID: contact.ID,
			Title:     "WA: " + phone,
			Stage:     models.StageNew,
			Status:    "open",
		}
		if err := s.repo.CreateTicket(ticket); err != nil {
			return err
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
			ContactID: contact.ID,
			Title:     title,
			Stage:     models.StageNew,
			Status:    "open",
		}
		if err := s.repo.CreateTicket(ticket); err != nil {
			return err
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

// broadcastTicketMessage notifies connected clients of a new ticket message.
func broadcastTicketMessage(ticketID uint, msg *models.TicketMessage) {
	websocket.GlobalNotifHub.BroadcastToAll("new_ticket_message", map[string]interface{}{
		"ticket_id": ticketID,
		"message":   msg,
	})
}

