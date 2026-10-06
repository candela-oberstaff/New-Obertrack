package service

import (
	"fmt"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/utils"
)

// notifyCompanyCS avisa por la campanita al Customer Success asignado a la
// empresa de un ticket nuevo: el de la empresa misma o el de alguno de sus
// profesionales. Es el que conoce al cliente, así que se entera aunque el
// ticket lo atienda otra persona de soporte.
//
// Los tickets de WhatsApp y correo no guardan la empresa: se deduce de quién
// escribe (teléfono o correo de un usuario de Obertrack). Las alertas internas
// la sacan del profesional del ticket. Sin empresa o sin CS, no hace nada.
func (s *ticketService) notifyCompanyCS(ticket *models.Ticket, phone, email string) {
	if s.notifSvc == nil || s.userRepo == nil || ticket == nil {
		return
	}
	company, person := s.ticketCompany(ticket, phone, email)
	if company == nil || company.AssignedCSID == nil || *company.AssignedCSID == 0 {
		return
	}
	csID := *company.AssignedCSID
	// Si el ticket ya es suyo, ya lo sabe.
	if ticket.AssignedTo != nil && *ticket.AssignedTo == csID {
		return
	}
	if cs, err := s.userRepo.GetByID(csID); err != nil || cs == nil || !cs.IsActive {
		return
	}

	companyName := strings.TrimSpace(company.CompanyName)
	if companyName == "" {
		companyName = company.Name
	}
	message := fmt.Sprintf("«%s»", ticket.Title)
	if person != nil && person.ID != company.ID && person.Name != "" {
		message = fmt.Sprintf("%s, de %s: «%s»", person.Name, companyName, ticket.Title)
	}
	_ = s.notifSvc.CreateNotification(csID, "ticket_cliente",
		"Nuevo ticket de tu cliente "+companyName,
		message,
		map[string]interface{}{
			"ticket":     ticket.ID,
			"origin":     ticket.Origin,
			"company_id": company.ID,
			"link":       ticketLink(ticket),
		})
}

// ticketCompany resuelve la empresa del ticket y la persona sobre la que trata.
// Orden: el usuario del ticket (alertas internas); si no, quien escribe, por
// teléfono y luego por correo. La persona puede ser la empresa misma.
func (s *ticketService) ticketCompany(ticket *models.Ticket, phone, email string) (*models.User, *models.User) {
	var person *models.User
	if ticket.UserID != nil && *ticket.UserID > 0 {
		if u, err := s.userRepo.GetByID(*ticket.UserID); err == nil {
			person = u
		}
	}
	if person == nil {
		if digits := utils.NormalizePhoneDigits(phone); digits != "" {
			if u, err := s.userRepo.FindActiveByPhoneDigits(digits); err == nil {
				person = u
			}
		}
	}
	if person == nil {
		if e := strings.TrimSpace(email); e != "" {
			if u, err := s.userRepo.GetByEmail(e); err == nil {
				person = u
			}
		}
	}
	if person == nil {
		return nil, nil
	}
	tenantID := models.TenantForUser(person)
	if tenantID == 0 {
		return nil, person
	}
	company := person
	if person.ID != tenantID {
		c, err := s.userRepo.GetByID(tenantID)
		if err != nil || c == nil {
			return nil, person
		}
		company = c
	}
	if company.UserType != models.UserTypeEmployer {
		return nil, person
	}
	return company, person
}

// ticketLink lleva al detalle del ticket: los internos y de Obersuite tienen
// su página; los de WhatsApp, su conversación; el resto, el tablero.
func ticketLink(ticket *models.Ticket) string {
	switch {
	case models.IsLocalOrigin(ticket.Origin):
		return fmt.Sprintf("/tickets/internal/%d", ticket.ID)
	case ticket.Origin == string(models.ChannelWhatsApp):
		return fmt.Sprintf("/tickets/wa/%d", ticket.ID)
	default:
		return "/tickets"
	}
}
