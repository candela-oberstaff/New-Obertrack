package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// lookupFakeRepo solo implementa lo que toca LookupWhatsAppChat. Embebe la
// interfaz para que cualquier otra llamada reviente en vez de pasar inadvertida.
type lookupFakeRepo struct {
	repository.TicketRepository

	ticket  *models.Ticket
	findErr error

	gotDigits  string
	gotSession string

	hasInbound bool
	inboundErr error
}

func (f *lookupFakeRepo) FindWhatsAppTicketByPhoneDigits(digits, session string) (*models.Ticket, error) {
	f.gotDigits, f.gotSession = digits, session
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.ticket, nil
}

func (f *lookupFakeRepo) HasInboundMessage(uint) (bool, error) {
	return f.hasInbound, f.inboundErr
}

func newLookupSvc(repo repository.TicketRepository, requireInbound bool) *ticketService {
	return &ticketService{
		repo: repo,
		wahaSvc: &WahaService{
			session:        "session_1",
			client:         &http.Client{Timeout: time.Second},
			requireInbound: requireInbound,
		},
	}
}

func TestLookupWhatsAppChat_NormalizaElTelefonoAntesDeBuscar(t *testing.T) {
	repo := &lookupFakeRepo{ticket: &models.Ticket{ID: 9}, hasInbound: true}
	svc := newLookupSvc(repo, true)

	res, err := svc.LookupWhatsAppChat("+34 600 00 00 00")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	// El contacto guarda el número como lo da WAHA; si no se normaliza aquí,
	// el cruce no encuentra nunca la conversación.
	if repo.gotDigits != "34600000000" {
		t.Fatalf("digits buscados = %q", repo.gotDigits)
	}
	if repo.gotSession != "session_1" {
		t.Fatalf("session = %q; debe acotarse a la sesión activa", repo.gotSession)
	}
	if res.TicketID != 9 || !res.CanReply {
		t.Fatalf("res = %+v", res)
	}
}

// Sin conversación no es un error: es la respuesta más común.
func TestLookupWhatsAppChat_SinConversacionNoEsError(t *testing.T) {
	repo := &lookupFakeRepo{findErr: gorm.ErrRecordNotFound}
	svc := newLookupSvc(repo, true)

	res, err := svc.LookupWhatsAppChat("+34600000000")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if res.TicketID != 0 || res.CanReply {
		t.Fatalf("res = %+v; sin hilo no se puede responder", res)
	}
}

// La guarda de contacto en frío: existe el hilo pero lo abrimos nosotros y
// nadie contestó. Decirlo aquí evita ofrecer un cuadro de texto que daría 403.
func TestLookupWhatsAppChat_HiloSinEntranteNoPermiteResponder(t *testing.T) {
	repo := &lookupFakeRepo{ticket: &models.Ticket{ID: 4}, hasInbound: false}
	svc := newLookupSvc(repo, true)

	res, err := svc.LookupWhatsAppChat("34600000000")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if res.TicketID != 4 {
		t.Fatalf("debería devolver el hilo para poder consultarlo: %+v", res)
	}
	if res.CanReply {
		t.Fatal("no debería permitir responder sin mensaje entrante")
	}
}

// Con la guarda desactivada por configuración, cualquier hilo existente vale.
func TestLookupWhatsAppChat_SinGuardaCualquierHiloPermiteResponder(t *testing.T) {
	repo := &lookupFakeRepo{ticket: &models.Ticket{ID: 4}, hasInbound: false}
	svc := newLookupSvc(repo, false)

	res, _ := svc.LookupWhatsAppChat("34600000000")
	if !res.CanReply {
		t.Fatal("con WAHA_REQUIRE_INBOUND=false debería poder responderse")
	}
}

func TestLookupWhatsAppChat_SinTelefonoNoConsultaNada(t *testing.T) {
	repo := &lookupFakeRepo{}
	svc := newLookupSvc(repo, true)

	res, err := svc.LookupWhatsAppChat("  ")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if res.TicketID != 0 || res.CanReply || res.Digits != "" {
		t.Fatalf("res = %+v", res)
	}
	if repo.gotDigits != "" || repo.gotSession != "" {
		t.Fatal("no debería llegar a consultar la base sin número")
	}
}

// Un fallo real de base sí sube: no es lo mismo "no hay conversación" que "no
// se pudo mirar", y tratarlos igual escondería una caída.
func TestLookupWhatsAppChat_ErrorDeBaseSePropaga(t *testing.T) {
	repo := &lookupFakeRepo{findErr: errors.New("boom")}
	svc := newLookupSvc(repo, true)

	if _, err := svc.LookupWhatsAppChat("34600000000"); err == nil {
		t.Fatal("se esperaba error")
	}
}
