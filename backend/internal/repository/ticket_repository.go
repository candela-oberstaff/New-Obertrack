package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TicketRepository centralizes all DB access for the support-inbox domain
// (contacts, tickets, ticket messages). Handlers must go through the service,
// which goes through this repository — never touching *gorm.DB directly.
type TicketRepository interface {
	GetContactByID(id uint) (*models.Contact, error)
	GetContactByPhone(phone string) (*models.Contact, error)
	GetContactByEmail(email string) (*models.Contact, error)
	CreateContact(c *models.Contact) error
	SaveContact(c *models.Contact) error

	GetOpenTicketByContactSince(contactID uint, since time.Time) (*models.Ticket, error)
	GetOpenTicketByContact(contactID uint) (*models.Ticket, error)
	CreateTicket(t *models.Ticket) error
	SaveTicket(t *models.Ticket) error
	TouchTicket(t *models.Ticket) error

	GetByID(id uint) (*models.Ticket, error)
	GetWithContact(id uint) (*models.Ticket, error)
	List(assignedTo *uint) ([]models.Ticket, error)
	ListByOrigin(origin string) ([]models.Ticket, error)
	ListInternalReport(start, end time.Time) ([]models.Ticket, error)

	CreateMessage(m *models.TicketMessage) error
	// HasInboundMessage reports whether the ticket has at least one message from
	// the contact — i.e. the contact wrote first. Used by the cold-outreach guard.
	HasInboundMessage(ticketID uint) (bool, error)
	// MessageExistsByExternalID reports whether a message with the given
	// (non-empty) provider ID is already stored — used to make inbound webhook
	// ingestion idempotent against retries.
	MessageExistsByExternalID(externalID string) (bool, error)
	// CreateMessageIfNew inserts the message unless one with the same external_id
	// already exists (ON CONFLICT DO NOTHING against the partial unique index).
	// Returns true when a row was actually inserted, false when it was a
	// duplicate. This is the race-safe backstop for concurrent webhook deliveries.
	CreateMessageIfNew(m *models.TicketMessage) (bool, error)

	CreateTransfer(t *models.TicketTransfer) error
	ListTransfers(origin, ref string) ([]models.TicketTransfer, error)
}

type ticketRepository struct {
	db *gorm.DB
}

func NewTicketRepository(db *gorm.DB) TicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) GetContactByID(id uint) (*models.Contact, error) {
	var c models.Contact
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ticketRepository) GetContactByPhone(phone string) (*models.Contact, error) {
	var c models.Contact
	if err := r.db.Where("phone = ?", phone).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ticketRepository) GetContactByEmail(email string) (*models.Contact, error) {
	var c models.Contact
	if err := r.db.Where("email = ?", email).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ticketRepository) CreateContact(c *models.Contact) error {
	return r.db.Create(c).Error
}

func (r *ticketRepository) SaveContact(c *models.Contact) error {
	return r.db.Save(c).Error
}

func (r *ticketRepository) GetOpenTicketByContactSince(contactID uint, since time.Time) (*models.Ticket, error) {
	var t models.Ticket
	err := r.db.Where("contact_id = ? AND status = ? AND updated_at >= ?", contactID, "open", since).
		Order("updated_at desc").First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ticketRepository) GetOpenTicketByContact(contactID uint) (*models.Ticket, error) {
	var t models.Ticket
	if err := r.db.Where("contact_id = ? AND status = ?", contactID, "open").First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ticketRepository) CreateTicket(t *models.Ticket) error {
	return r.db.Create(t).Error
}

func (r *ticketRepository) SaveTicket(t *models.Ticket) error {
	return r.db.Save(t).Error
}

func (r *ticketRepository) TouchTicket(t *models.Ticket) error {
	return r.db.Model(t).Update("updated_at", time.Now()).Error
}

func (r *ticketRepository) GetByID(id uint) (*models.Ticket, error) {
	var t models.Ticket
	if err := r.db.Preload("Contact").Preload("Assignee").Preload("Messages").First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ticketRepository) GetWithContact(id uint) (*models.Ticket, error) {
	var t models.Ticket
	if err := r.db.Preload("Contact").First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ticketRepository) List(assignedTo *uint) ([]models.Ticket, error) {
	var tickets []models.Ticket
	q := r.db.Preload("Contact").Preload("Assignee").Order("updated_at desc")
	if assignedTo != nil {
		q = q.Where("assigned_to = ?", *assignedTo)
	}
	if err := q.Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *ticketRepository) ListByOrigin(origin string) ([]models.Ticket, error) {
	var tickets []models.Ticket
	if err := r.db.Preload("Contact").Preload("Assignee").Preload("Messages").
		Where("origin = ?", origin).Order("updated_at desc").Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *ticketRepository) ListInternalReport(start, end time.Time) ([]models.Ticket, error) {
	var tickets []models.Ticket
	if err := r.db.Preload("Messages").
		Where("origin = ? AND created_at BETWEEN ? AND ?", models.OriginInternal, start, end).
		Order("created_at desc").Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *ticketRepository) CreateMessage(m *models.TicketMessage) error {
	return r.db.Create(m).Error
}

func (r *ticketRepository) HasInboundMessage(ticketID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&models.TicketMessage{}).
		Where("ticket_id = ? AND sender_type = ?", ticketID, models.SenderTypeContact).
		Limit(1).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ticketRepository) MessageExistsByExternalID(externalID string) (bool, error) {
	if externalID == "" {
		return false, nil
	}
	var count int64
	if err := r.db.Model(&models.TicketMessage{}).
		Where("external_id = ?", externalID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ticketRepository) CreateMessageIfNew(m *models.TicketMessage) (bool, error) {
	res := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(m)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *ticketRepository) CreateTransfer(t *models.TicketTransfer) error {
	return r.db.Create(t).Error
}

func (r *ticketRepository) ListTransfers(origin, ref string) ([]models.TicketTransfer, error) {
	var transfers []models.TicketTransfer
	if err := r.db.Where("origin = ? AND ticket_ref = ?", origin, ref).
		Order("created_at desc").Find(&transfers).Error; err != nil {
		return nil, err
	}
	return transfers, nil
}
