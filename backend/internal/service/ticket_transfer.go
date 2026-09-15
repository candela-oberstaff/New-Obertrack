package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// Chats de WhatsApp transferidos desde Obersuite, v2: un ticket de WhatsApp DE
// VERDAD —contacto, mensajes en burbujas con su hora original, y respuesta
// desde /tickets/wa/:id por nuestro número—, no un ticket interno con el
// historial pegado en la descripción (la v1, que Customer Success no podía
// contestar).
//
// Dos decisiones que se tomaron mirando cómo envía WhatsApp Obertrack, y no
// como las proponía Obersuite:
//
//   - La sesión del ticket es la NUESTRA (WAHA_SESSION), no la que manda
//     Obersuite. Obertrack envía siempre por un único número, y la bandeja de
//     WhatsApp filtra los tickets por esa sesión: uno etiquetado con la suya no
//     se vería. La sesión de origen queda escrita en el mensaje de sistema.
//   - Transferir dos veces el mismo chat NO abre un segundo ticket. Un chat de
//     WhatsApp es una conversación continua (así lo trata el webhook: reutiliza
//     el ticket abierto del contacto), y partirla en dos dejaría respuestas en
//     el hilo viejo. La segunda transferencia se ANEXA al abierto: su mensaje de
//     sistema y los mensajes que aún no estuvieran.

type ObersuiteTransferMessage struct {
	At   *time.Time
	From string // "contact" | "agent"
	Text string
}

// ObersuiteTransferInput es lo que manda Obersuite. Los campos de la v1
// (context, candidate_phone sin JID) se siguen aceptando por si algún envío
// viejo reintenta; context NO se interpreta, es solo respaldo legible.
type ObersuiteTransferInput struct {
	ExternalID        string
	CandidateName     string
	CandidatePhone    string
	WahaChatID        string
	WahaSession       string
	TransferredByName string
	TransferredAt     *time.Time
	Reason            string
	Context           string
	Source            string
	Messages          []ObersuiteTransferMessage
}

type ObersuiteTransferResult struct {
	ID     uint   `json:"id"`
	Status string `json:"status"` // "created" | "already_exists"
	URL    string `json:"url"`
}

// maxTransferMessages acota lo que se importa: Obersuite manda hasta 30.
const maxTransferMessages = 50

// transferMarkerSuffix es cómo se reconoce el mensaje de sistema de UNA
// transferencia: su external_id es el de la transferencia más este sufijo. Es
// la idempotencia de las transferencias que se anexan a un ticket ya abierto,
// donde tickets.external_id sigue siendo el de la primera.
const transferMarkerSuffix = "-transfer"

func (s *ticketService) CreateObersuiteTransfer(in ObersuiteTransferInput) (*ObersuiteTransferResult, error) {
	externalID := strings.TrimSpace(in.ExternalID)
	if externalID == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id es obligatorio: es lo que evita duplicar la transferencia al reintentar")
	}
	if len(externalID) > 200 {
		return nil, hireFail(apperrors.ErrInvalidInput, "external_id no puede superar los 200 caracteres")
	}
	name := strings.TrimSpace(in.CandidateName)
	if name == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "candidate_name es obligatorio: es el título del ticket")
	}
	by := strings.TrimSpace(in.TransferredByName)
	if by == "" {
		return nil, hireFail(apperrors.ErrInvalidInput, "transferred_by_name es obligatorio: Customer Success tiene que saber a quién preguntar")
	}
	phone := strings.TrimSpace(in.CandidatePhone)
	jid := strings.TrimSpace(in.WahaChatID)
	if jid == "" {
		// Sin JID no hay a quién responder. Se deriva del teléfono si vino:
		// un envío de la v1 sigue funcionando.
		if phone == "" {
			return nil, hireFail(apperrors.ErrInvalidInput, "waha_chat_id es obligatorio (o candidate_phone para derivarlo): sin él no hay a quién responder")
		}
		jid = phone + "@c.us"
	}
	if phone == "" {
		if i := strings.IndexByte(jid, '@'); i > 0 {
			phone = jid[:i]
		}
	}
	if len(in.Messages) > maxTransferMessages {
		return nil, hireFail(apperrors.ErrInvalidInput, fmt.Sprintf("messages admite como mucho %d elementos", maxTransferMessages))
	}
	for i, m := range in.Messages {
		if m.From != string(models.SenderTypeContact) && m.From != string(models.SenderTypeAgent) {
			return nil, hireFail(apperrors.ErrInvalidInput,
				fmt.Sprintf("messages[%d].from %q no vale: solo contact (el candidato) o agent (el reclutador)", i, m.From))
		}
	}
	reason := strings.TrimSpace(in.Reason)
	when := time.Now()
	if in.TransferredAt != nil && !in.TransferredAt.IsZero() {
		when = *in.TransferredAt
	}

	// 1. Idempotencia, ANTES de tocar nada: por el ticket que nació de esta
	//    transferencia, o por su marca dentro de un ticket al que se anexó.
	if t, err := s.repo.FindByExternalID(externalID); err != nil {
		return nil, err
	} else if t != nil {
		return s.transferResult(t.ID, "already_exists"), nil
	}
	if tid, found, err := s.repo.TicketIDByMessageExternalID(externalID + transferMarkerSuffix); err != nil {
		return nil, err
	} else if found {
		return s.transferResult(tid, "already_exists"), nil
	}

	// 2. Contacto: por el JID primero (es la identidad de WhatsApp), por
	//    teléfono después. Nunca dos contactos para el mismo JID.
	contact, err := s.repo.GetContactByWaID(jid)
	if err != nil || contact == nil {
		if phone != "" {
			contact, _ = s.repo.GetContactByPhone(phone)
		}
	}
	if contact == nil {
		contact = &models.Contact{Phone: phone, WaID: jid, Name: name}
		if err := s.repo.CreateContact(contact); err != nil {
			return nil, err
		}
	} else {
		dirty := false
		if contact.WaID == "" {
			contact.WaID = jid
			dirty = true
		}
		if contact.Name == "" || strings.HasPrefix(contact.Name, "WA User ") {
			contact.Name = name
			dirty = true
		}
		if dirty {
			_ = s.repo.SaveContact(contact)
		}
	}

	// 3. Ticket: el abierto del contacto en NUESTRA sesión si lo hay (el
	//    candidato ya hablaba con nuestro número, o ya se transfirió antes);
	//    si no, uno nuevo.
	session := s.wahaSvc.GetSession()
	status := "already_exists"
	ticket, err := s.repo.GetOpenTicketByContact(contact.ID, session)
	if err != nil || ticket == nil {
		status = "created"
		ticket = &models.Ticket{
			ContactID:         &contact.ID,
			Origin:            string(models.ChannelWhatsApp),
			Session:           session,
			Title:             "WA: " + name,
			ExternalID:        externalID,
			Reason:            reason,
			ProfessionalPhone: phone,
			Stage:             models.StageNew,
			Status:            "open",
		}
		if err := s.repo.CreateTicket(ticket); err != nil {
			if isUniqueViolation(err) {
				if again, ferr := s.repo.FindByExternalID(externalID); ferr == nil && again != nil {
					return s.transferResult(again.ID, "already_exists"), nil
				}
			}
			return nil, err
		}
	}

	// 4. Los mensajes, con su hora original: todos con la hora de ahora saldrían
	//    apelotonados en las burbujas. Al anexar a un ticket que ya existía se
	//    saltan los que ya estén (mismo remitente, texto y hora), porque una
	//    segunda transferencia trae el chat entero otra vez con otro id.
	for i, m := range in.Messages {
		text := strings.TrimSpace(m.Text)
		if text == "" {
			continue
		}
		at := when
		if m.At != nil && !m.At.IsZero() {
			at = *m.At
		}
		sender := models.SenderType(m.From)
		if status != "created" {
			if dup, derr := s.repo.MessageExistsInTicket(ticket.ID, sender, text, at); derr == nil && dup {
				continue
			}
		}
		msg := &models.TicketMessage{
			TicketID:   ticket.ID,
			SenderType: sender,
			Channel:    models.ChannelWhatsApp,
			Content:    text,
			CreatedAt:  at,
			// Ya entregados por WhatsApp desde Obersuite: la bandeja de salida
			// no debe tocarlos.
			DeliveryStatus: deliveryStatusFor(sender == models.SenderTypeAgent),
			ExternalID:     fmt.Sprintf("%s-%d", externalID, i),
		}
		if _, err := s.repo.CreateMessageIfNew(msg); err != nil {
			return nil, err
		}
	}

	// 5. El mensaje de sistema: quién lo pasó, cuándo, por qué, y por qué
	//    número venía. Es además la marca de idempotencia de esta transferencia.
	var b strings.Builder
	fmt.Fprintf(&b, "Chat transferido desde Obersuite por %s el %s.", by, when.UTC().Format("02/01/2006 15:04 UTC"))
	if reason != "" {
		fmt.Fprintf(&b, " Motivo: %s.", reason)
	}
	if ses := strings.TrimSpace(in.WahaSession); ses != "" && ses != session {
		fmt.Fprintf(&b, " El candidato hablaba por la sesión «%s» de Obersuite; las respuestas desde aquí salen por el número de Obertrack.", ses)
	}
	if len(in.Messages) == 0 && strings.TrimSpace(in.Context) != "" {
		// Envío viejo (v1) sin mensajes estructurados: el respaldo legible se
		// conserva para que no se pierda el contexto, aunque no sea en burbujas.
		fmt.Fprintf(&b, "\n\nHistorial del chat:\n%s", strings.TrimSpace(in.Context))
	}
	marker := &models.TicketMessage{
		TicketID:       ticket.ID,
		SenderType:     models.SenderTypeSystem,
		Channel:        models.ChannelWhatsApp,
		Content:        b.String(),
		CreatedAt:      when,
		DeliveryStatus: models.DeliverySent,
		ExternalID:     externalID + transferMarkerSuffix,
	}
	if _, err := s.repo.CreateMessageIfNew(marker); err != nil {
		return nil, err
	}
	_ = s.repo.TouchTicket(ticket)
	broadcastTicketMessage(ticket.ID, marker)

	if s.supportNtfy != nil {
		s.supportNtfy.Notify(SupportTicketInfo{
			Type:        "Chat de WhatsApp transferido desde Obersuite",
			Requester:   name,
			Subject:     ticket.Title,
			Description: reason,
			Link:        fmt.Sprintf("/tickets/wa/%d", ticket.ID),
		})
	}
	return s.transferResult(ticket.ID, status), nil
}

func (s *ticketService) transferResult(id uint, status string) *ObersuiteTransferResult {
	return &ObersuiteTransferResult{ID: id, Status: status, URL: fmt.Sprintf("/tickets/wa/%d", id)}
}

// isTransferredFromObersuite reconoce un ticket que nació de una transferencia.
// Sirve a la guarda de contacto en frío: el chat ya existía en Obersuite —el
// reclutador ya le había escrito— así que responder desde aquí no es abordar a
// un desconocido, aunque el candidato nunca haya contestado.
func isTransferredFromObersuite(t *models.Ticket) bool {
	return t != nil && strings.HasPrefix(t.ExternalID, "obersuite-")
}
