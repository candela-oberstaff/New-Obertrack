package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// Chats de WhatsApp transferidos desde Obersuite, v2: un ticket de WhatsApp de
// verdad. Lo que se fija es el contrato y las dos decisiones que se tomaron
// mirando cómo envía Obertrack: la sesión del ticket es la nuestra, y una
// segunda transferencia se anexa al ticket abierto en vez de abrir otro.

type fakeTransferRepo struct {
	fakeTicketRepo
	contactsByWa    map[string]*models.Contact
	contactsByPhone map[string]*models.Contact
	openByContact   map[uint]*models.Ticket
	byExternal      map[string]*models.Ticket
	messages        []*models.TicketMessage
	creados         int
	createErr       error
}

func (f *fakeTransferRepo) GetContactByWaID(wa string) (*models.Contact, error) {
	if c, ok := f.contactsByWa[wa]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeTransferRepo) GetContactByPhone(p string) (*models.Contact, error) {
	if c, ok := f.contactsByPhone[p]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeTransferRepo) CreateContact(c *models.Contact) error {
	c.ID = uint(100 + len(f.contactsByWa))
	if f.contactsByWa == nil {
		f.contactsByWa = map[string]*models.Contact{}
	}
	f.contactsByWa[c.WaID] = c
	return nil
}
func (f *fakeTransferRepo) SaveContact(c *models.Contact) error { return nil }
func (f *fakeTransferRepo) FindByExternalID(ext string) (*models.Ticket, error) {
	return f.byExternal[ext], nil
}
func (f *fakeTransferRepo) TicketIDByMessageExternalID(ext string) (uint, bool, error) {
	for _, m := range f.messages {
		if m.ExternalID == ext {
			return m.TicketID, true, nil
		}
	}
	return 0, false, nil
}
func (f *fakeTransferRepo) GetOpenTicketByContact(contactID uint, session string) (*models.Ticket, error) {
	if t, ok := f.openByContact[contactID]; ok && t.Session == session {
		return t, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeTransferRepo) CreateTicket(t *models.Ticket) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.creados++
	t.ID = uint(700 + f.creados)
	f.guardado = t
	if f.byExternal == nil {
		f.byExternal = map[string]*models.Ticket{}
	}
	f.byExternal[t.ExternalID] = t
	if f.openByContact == nil {
		f.openByContact = map[uint]*models.Ticket{}
	}
	f.openByContact[*t.ContactID] = t
	return nil
}
func (f *fakeTransferRepo) MessageExistsInTicket(ticketID uint, sender models.SenderType, content string, at time.Time) (bool, error) {
	for _, m := range f.messages {
		if m.TicketID == ticketID && m.SenderType == sender && m.Content == content && m.CreatedAt.Truncate(time.Second).Equal(at.Truncate(time.Second)) {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeTransferRepo) CreateMessageIfNew(m *models.TicketMessage) (bool, error) {
	for _, e := range f.messages {
		if e.ExternalID == m.ExternalID {
			return false, nil
		}
	}
	m.ID = uint(len(f.messages) + 1)
	f.messages = append(f.messages, m)
	return true, nil
}
func (f *fakeTransferRepo) TouchTicket(_ *models.Ticket) error { return nil }

func (f *fakeTransferRepo) mensajesDe(ticketID uint) []*models.TicketMessage {
	out := []*models.TicketMessage{}
	for _, m := range f.messages {
		if m.TicketID == ticketID {
			out = append(out, m)
		}
	}
	return out
}

func newTransferSvc() (*ticketService, *fakeTransferRepo) {
	repo := &fakeTransferRepo{}
	return &ticketService{repo: repo, wahaSvc: &WahaService{session: "OsvellTest"}}, repo
}

func t0(s string) *time.Time { t, _ := time.Parse(time.RFC3339, s); return &t }

func transferBase() ObersuiteTransferInput {
	return ObersuiteTransferInput{
		ExternalID:        "obersuite-chat-5491112345678@c.us-42",
		CandidateName:     "Juan Pérez",
		CandidatePhone:    "5491112345678",
		WahaChatID:        "5491112345678@c.us",
		WahaSession:       "default",
		TransferredByName: "Lorena Moujalli",
		TransferredAt:     t0("2026-09-15T15:40:00Z"),
		Reason:            "Necesita acompañamiento post-contratación",
		Source:            "whatsapp_chat",
		Messages: []ObersuiteTransferMessage{
			{At: t0("2026-09-15T15:30:12Z"), From: "contact", Text: "Hola, quería consultar…"},
			{At: t0("2026-09-15T15:31:40Z"), From: "agent", Text: "Hola Juan…"},
		},
	}
}

func TestTransferencia_EsUnTicketDeWhatsAppDeVerdad(t *testing.T) {
	svc, repo := newTransferSvc()

	res, err := svc.CreateObersuiteTransfer(transferBase())

	if err != nil || res.Status != "created" || res.URL != "/tickets/wa/701" {
		t.Fatalf("%+v %v", res, err)
	}
	tk := repo.guardado
	if tk.Origin != "whatsapp" || tk.Title != "WA: Juan Pérez" || tk.ExternalID != transferBase().ExternalID {
		t.Errorf("ticket: origin=%s title=%q ext=%q", tk.Origin, tk.Title, tk.ExternalID)
	}
	// La sesión es la NUESTRA: la bandeja de WhatsApp filtra por ella y un
	// ticket con la de Obersuite no se vería.
	if tk.Session != "OsvellTest" {
		t.Errorf("session=%q, tenía que ser la de Obertrack", tk.Session)
	}
	c := repo.contactsByWa["5491112345678@c.us"]
	if c == nil || c.Phone != "5491112345678" || c.Name != "Juan Pérez" || tk.ContactID == nil || *tk.ContactID != c.ID {
		t.Errorf("contacto: %+v ticket.contact=%v", c, tk.ContactID)
	}
}

func TestTransferencia_LasBurbujasLlevanSuHoraOriginalYSuLado(t *testing.T) {
	svc, repo := newTransferSvc()
	svc.CreateObersuiteTransfer(transferBase())

	msgs := repo.mensajesDe(701)
	if len(msgs) != 3 { // dos del chat + el de sistema
		t.Fatalf("mensajes: %d", len(msgs))
	}
	if msgs[0].SenderType != models.SenderTypeContact || !msgs[0].CreatedAt.Equal(*t0("2026-09-15T15:30:12Z")) {
		t.Errorf("primero: %s %v", msgs[0].SenderType, msgs[0].CreatedAt)
	}
	if msgs[1].SenderType != models.SenderTypeAgent || msgs[1].DeliveryStatus != models.DeliverySent {
		t.Errorf("el del reclutador ya está entregado: %s %q", msgs[1].SenderType, msgs[1].DeliveryStatus)
	}
	if msgs[0].DeliveryStatus != "" {
		t.Errorf("un entrante no tiene estado de entrega: %q", msgs[0].DeliveryStatus)
	}
	if msgs[0].ExternalID != transferBase().ExternalID+"-0" || msgs[1].Channel != models.ChannelWhatsApp {
		t.Errorf("ids/canal: %q %q", msgs[0].ExternalID, msgs[1].Channel)
	}
	sys := msgs[2]
	if sys.SenderType != models.SenderTypeSystem || !strings.Contains(sys.Content, "Lorena Moujalli") || !strings.Contains(sys.Content, "Motivo:") {
		t.Errorf("mensaje de sistema: %s %q", sys.SenderType, sys.Content)
	}
	// La sesión de origen distinta de la nuestra se dice, con lo que implica.
	if !strings.Contains(sys.Content, "«default»") || !strings.Contains(sys.Content, "número de Obertrack") {
		t.Errorf("tiene que avisar del cambio de número: %q", sys.Content)
	}
}

func TestTransferencia_ElMismoIdNoDuplica(t *testing.T) {
	svc, repo := newTransferSvc()
	primera, _ := svc.CreateObersuiteTransfer(transferBase())
	segunda, err := svc.CreateObersuiteTransfer(transferBase())

	if err != nil || segunda.Status != "already_exists" || segunda.ID != primera.ID {
		t.Fatalf("%+v %v", segunda, err)
	}
	if repo.creados != 1 || len(repo.mensajesDe(701)) != 3 {
		t.Fatalf("tickets=%d mensajes=%d", repo.creados, len(repo.mensajesDe(701)))
	}
}

// Una SEGUNDA transferencia del mismo chat (otro id) no abre otro ticket: se
// anexa al abierto, con su propio mensaje de sistema, y sin repetir las
// burbujas que ya estaban.
func TestTransferencia_TransferirDosVecesAnexaSinDuplicar(t *testing.T) {
	svc, repo := newTransferSvc()
	svc.CreateObersuiteTransfer(transferBase())

	otra := transferBase()
	otra.ExternalID = "obersuite-chat-5491112345678@c.us-43"
	otra.TransferredAt = t0("2026-09-16T10:00:00Z")
	otra.Messages = append(otra.Messages, ObersuiteTransferMessage{At: t0("2026-09-16T09:50:00Z"), From: "contact", Text: "¿Novedades?"})
	res, err := svc.CreateObersuiteTransfer(otra)

	if err != nil || res.Status != "already_exists" || res.ID != 701 {
		t.Fatalf("%+v %v", res, err)
	}
	if repo.creados != 1 {
		t.Fatalf("abrió otro ticket: %d", repo.creados)
	}
	msgs := repo.mensajesDe(701)
	// 2 originales + sistema + 1 nuevo + sistema de la segunda = 5
	if len(msgs) != 5 {
		for _, m := range msgs {
			t.Logf("  %s %q", m.SenderType, m.Content)
		}
		t.Fatalf("mensajes: %d (las burbujas repetidas no se duplican)", len(msgs))
	}
	// Y reintentar la segunda tampoco duplica.
	svc.CreateObersuiteTransfer(otra)
	if len(repo.mensajesDe(701)) != 5 {
		t.Fatal("el reintento de la segunda transferencia duplicó mensajes")
	}
}

func TestTransferencia_ContactoYaConocidoPorJIDNoSeDuplica(t *testing.T) {
	svc, repo := newTransferSvc()
	repo.contactsByWa = map[string]*models.Contact{"5491112345678@c.us": {ID: 9, WaID: "5491112345678@c.us", Phone: "5491112345678", Name: "WA User 5491112345678"}}

	svc.CreateObersuiteTransfer(transferBase())

	if len(repo.contactsByWa) != 1 {
		t.Fatal("creó un segundo contacto para el mismo JID")
	}
	if repo.contactsByWa["5491112345678@c.us"].Name != "Juan Pérez" {
		t.Error("el nombre genérico 'WA User …' se sustituye por el real")
	}
	if *repo.guardado.ContactID != 9 {
		t.Error("el ticket tiene que apuntar al contacto existente")
	}
}

func TestTransferencia_SinJIDSeDerivaDelTelefono(t *testing.T) {
	svc, repo := newTransferSvc()
	in := transferBase()
	in.WahaChatID = ""
	if _, err := svc.CreateObersuiteTransfer(in); err != nil {
		t.Fatal(err)
	}
	if repo.contactsByWa["5491112345678@c.us"] == nil {
		t.Fatal("con teléfono pero sin JID, el JID se deriva")
	}
}

func TestTransferencia_Los400DelContrato(t *testing.T) {
	casos := []struct {
		nombre string
		muta   func(*ObersuiteTransferInput)
		frase  string
	}{
		{"sin external_id", func(i *ObersuiteTransferInput) { i.ExternalID = " " }, "external_id"},
		{"sin candidate_name", func(i *ObersuiteTransferInput) { i.CandidateName = "" }, "candidate_name"},
		{"sin transferred_by_name", func(i *ObersuiteTransferInput) { i.TransferredByName = "" }, "transferred_by_name"},
		{"sin JID ni teléfono", func(i *ObersuiteTransferInput) { i.WahaChatID = ""; i.CandidatePhone = "" }, "waha_chat_id"},
		{"from desconocido", func(i *ObersuiteTransferInput) { i.Messages[0].From = "bot" }, "messages[0].from"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			svc, repo := newTransferSvc()
			in := transferBase()
			c.muta(&in)
			_, err := svc.CreateObersuiteTransfer(in)
			if !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Fatalf("debía ser 400, got %v", err)
			}
			if !strings.Contains(err.Error(), c.frase) {
				t.Errorf("mensaje %q debía mencionar %q", err.Error(), c.frase)
			}
			if repo.creados != 0 || len(repo.messages) != 0 {
				t.Error("un 400 no escribe nada")
			}
		})
	}
}

// Un chat donde el candidato nunca contestó no trae mensajes de contact. Sin
// esta excepción, la guarda de contacto en frío bloquearía a Customer Success
// para siempre en un chat que el reclutador ya había abierto.
func TestTransferencia_SinRespuestaDelCandidatoSePuedeContestar(t *testing.T) {
	ticket := &models.Ticket{ID: 5, ExternalID: "obersuite-chat-549@c.us-1", Origin: "whatsapp"}
	if !isTransferredFromObersuite(ticket) {
		t.Fatal("un ticket nacido de una transferencia tiene que reconocerse")
	}
	if isTransferredFromObersuite(&models.Ticket{ID: 6, Origin: "whatsapp"}) {
		t.Fatal("uno del webhook, no")
	}
}

// Los mensajes vacíos ("[multimedia]" sí cuenta, el vacío no) no crean burbujas
// vacías, y sin mensajes estructurados el respaldo legible no se pierde.
func TestTransferencia_SinMensajesElRespaldoSeConserva(t *testing.T) {
	svc, repo := newTransferSvc()
	in := transferBase()
	in.Messages = nil
	in.Context = "[15/09/2026, 12:30] Juan Pérez: hola"
	svc.CreateObersuiteTransfer(in)
	msgs := repo.mensajesDe(701)
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "Historial del chat") {
		t.Fatalf("mensajes: %d %q", len(msgs), msgs[0].Content)
	}
}
