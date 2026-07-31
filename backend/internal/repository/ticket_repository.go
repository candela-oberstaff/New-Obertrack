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
	// GetOpenTicketByContact busca el hilo abierto del contacto DENTRO de una
	// sesión: el mismo teléfono escribiendo a dos números de la empresa son dos
	// conversaciones distintas y no deben compartir ticket.
	GetOpenTicketByContact(contactID uint, session string) (*models.Ticket, error)
	// FindWhatsAppTicketByPhoneDigits busca la conversación de WhatsApp de un
	// teléfono comparando solo dígitos, porque el número guardado en el contacto
	// viene de WAHA y el que se teclea en una ficha viene con "+", espacios o
	// guiones. Prefiere el hilo abierto y, entre varios, el más reciente.
	FindWhatsAppTicketByPhoneDigits(digits, session string) (*models.Ticket, error)
	// ListByOriginAndSession lista los tickets de un canal acotados a una sesión.
	ListByOriginAndSession(origin, session string) ([]models.Ticket, error)
	CreateTicket(t *models.Ticket) error
	SaveTicket(t *models.Ticket) error
	TouchTicket(t *models.Ticket) error
	// SyncTicketActivity realinea updated_at con la fecha del mensaje más reciente
	// del ticket. TouchTicket no sirve para esto porque fuerza time.Now(): al
	// importar historial todos los tickets quedarían sellados con el instante del
	// import, que es justo lo que hace que la lista de chats muestre la misma hora
	// en todas las conversaciones y las ordene mal.
	SyncTicketActivity(ticketID uint) error

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

	// --- Bandeja de salida de WhatsApp ---
	// ClaimPendingOutbound devuelve los mensajes salientes listos para entregar.
	ClaimPendingOutbound(limit int, now time.Time) ([]models.TicketMessage, error)
	// MarkMessageSent cierra el envío guardando el ID que asignó WAHA.
	MarkMessageSent(msgID uint, externalID string) error
	// MarkMessageFailed registra el intento fallido; retryAt nil = agotado.
	MarkMessageFailed(msgID uint, attempts int, errMsg string, retryAt *time.Time) error
	// RescheduleMessage pospone un envío SIN gastar un intento. Es el caso del
	// límite antibaneo: la cola no falló, solo va a su ritmo.
	RescheduleMessage(msgID uint, at time.Time) error

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

func (r *ticketRepository) GetOpenTicketByContact(contactID uint, session string) (*models.Ticket, error) {
	var t models.Ticket
	if err := r.db.Where("contact_id = ? AND status = ? AND session = ?", contactID, "open", session).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// FindWhatsAppTicketByPhoneDigits cruza el teléfono contra contacts comparando
// dígitos a ambos lados (regexp_replace en la columna, ya normalizado en el
// parámetro). Se mira también wa_id porque en los contactos importados el
// número real vive ahí y phone puede venir vacío.
//
// El orden prioriza el hilo abierto y, dentro de eso, la actividad más
// reciente: si una empresa escribió hace un año y volvió ayer, soporte quiere
// la conversación de ayer.
func (r *ticketRepository) FindWhatsAppTicketByPhoneDigits(digits, session string) (*models.Ticket, error) {
	if digits == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var t models.Ticket
	err := r.db.
		Joins("JOIN contacts c ON c.id = tickets.contact_id AND c.deleted_at IS NULL").
		Where("tickets.origin = ? AND tickets.session = ? AND tickets.deleted_at IS NULL", string(models.ChannelWhatsApp), session).
		Where(`regexp_replace(COALESCE(c.phone, ''), '\D', '', 'g') = ?
			OR regexp_replace(COALESCE(c.wa_id, ''), '\D', '', 'g') = ?`, digits, digits).
		Order("CASE WHEN tickets.status = 'open' THEN 0 ELSE 1 END, tickets.updated_at DESC").
		Preload("Contact").
		First(&t).Error
	if err != nil {
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

// SyncTicketActivity fija updated_at en la fecha del último mensaje del ticket.
//
// Se resuelve en una sola sentencia SQL en vez de leer los mensajes en memoria: el
// COALESCE deja el valor actual intacto si el ticket todavía no tiene mensajes, y
// se filtra por id con un modelo limpio para no arrastrar asociaciones precargadas.
func (r *ticketRepository) SyncTicketActivity(ticketID uint) error {
	return r.db.Model(&models.Ticket{}).
		Where("id = ?", ticketID).
		Update("updated_at", gorm.Expr(
			`COALESCE((SELECT MAX(created_at) FROM ticket_messages
			   WHERE ticket_id = ? AND deleted_at IS NULL), updated_at)`, ticketID,
		)).Error
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

func (r *ticketRepository) ListByOriginAndSession(origin, session string) ([]models.Ticket, error) {
	var tickets []models.Ticket
	if err := r.db.Preload("Contact").Preload("Assignee").Preload("Messages").
		Where("origin = ? AND session = ?", origin, session).
		Order("updated_at desc").Find(&tickets).Error; err != nil {
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

// ClaimPendingOutbound selecciona los mensajes salientes procesables: pendientes
// y ya vencidos. El worker es único (una goroutine), así que no hace falta
// SELECT ... FOR UPDATE; el orden por id garantiza FIFO, que es lo que preserva
// el orden de la conversación tal como lo escribió el agente.
//
// Un mensaje esperando su backoff no bloquea la cola: queda fuera de la selección
// y los demás siguen entregándose.
func (r *ticketRepository) ClaimPendingOutbound(limit int, now time.Time) ([]models.TicketMessage, error) {
	var msgs []models.TicketMessage
	err := r.db.
		Where("delivery_status = ?", models.DeliveryPending).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Order("id ASC").
		Limit(limit).
		Find(&msgs).Error
	return msgs, err
}

func (r *ticketRepository) MarkMessageSent(msgID uint, externalID string) error {
	return r.db.Model(&models.TicketMessage{}).
		Where("id = ?", msgID).
		Updates(map[string]interface{}{
			"delivery_status": models.DeliverySent,
			"external_id":     externalID,
			"next_attempt_at": nil,
			"delivery_error":  "",
		}).Error
}

func (r *ticketRepository) MarkMessageFailed(msgID uint, attempts int, errMsg string, retryAt *time.Time) error {
	status := models.DeliveryPending
	if retryAt == nil {
		status = models.DeliveryFailed
	}
	return r.db.Model(&models.TicketMessage{}).
		Where("id = ?", msgID).
		Updates(map[string]interface{}{
			"delivery_status": status,
			// next_attempt_at se escribe siempre, incluido el nil de un mensaje
			// agotado: dejar la fecha del intento anterior en una fila 'failed' solo
			// confundiría al leer la tabla para diagnosticar.
			"next_attempt_at":   retryAt,
			"delivery_attempts": attempts,
			"delivery_error":    errMsg,
		}).Error
}

func (r *ticketRepository) RescheduleMessage(msgID uint, at time.Time) error {
	return r.db.Model(&models.TicketMessage{}).
		Where("id = ?", msgID).
		Update("next_attempt_at", at).Error
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
