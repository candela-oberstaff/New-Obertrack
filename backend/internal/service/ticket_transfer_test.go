package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
)

// Chats de WhatsApp transferidos desde Obersuite. Lo que se fija es el
// contrato: qué es 400 y con qué texto, que el mismo external_id no duplica,
// y que el ticket cae en la bandeja donde Customer Success ya mira (mismo
// origen que las altas) con la descripción en el orden en que se lee.

type fakeTransferRepo struct {
	fakeTicketRepo
	porExternal map[string]*models.Ticket
	createErr   error
	creados     int
	// apareceTrasCrear simula al otro escritor de una carrera: la fila que no
	// estaba antes de intentar crear y si esta despues.
	apareceTrasCrear *models.Ticket
	intentoCrear     bool
}

func (f *fakeTransferRepo) FindByExternalID(ext string) (*models.Ticket, error) {
	if t, ok := f.porExternal[ext]; ok {
		return t, nil
	}
	if f.intentoCrear && f.apareceTrasCrear != nil && f.apareceTrasCrear.ExternalID == ext {
		return f.apareceTrasCrear, nil
	}
	return nil, nil
}

func (f *fakeTransferRepo) CreateTicket(t *models.Ticket) error {
	f.intentoCrear = true
	if f.createErr != nil {
		return f.createErr
	}
	f.creados++
	t.ID = uint(700 + f.creados)
	f.guardado = t
	if f.porExternal == nil {
		f.porExternal = map[string]*models.Ticket{}
	}
	f.porExternal[t.ExternalID] = t
	return nil
}

func transferBase() ObersuiteTransferInput {
	return ObersuiteTransferInput{
		ExternalID:        "obersuite-chat-5491112345678@c.us-42",
		CandidateName:     "Juan Pérez",
		CandidatePhone:    "5491112345678",
		TransferredByName: "Lorena Moujalli",
		Reason:            "Necesita acompañamiento post-contratación",
		Context:           "[10/09 14:32] Juan Pérez: Hola, quería consultar...\n[10/09 14:33] Reclutador: Hola Juan...",
		Source:            "whatsapp_chat",
	}
}

func TestTransferencia_CaeEnLaBandejaDeObersuite(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := &ticketService{repo: repo}

	res, err := svc.CreateObersuiteTransfer(transferBase())

	if err != nil || res.Status != "created" {
		t.Fatalf("%v %v", res, err)
	}
	tk := repo.guardado
	if tk.Origin != models.OriginObersuite || tk.Stage != models.StageNew || tk.Status != "open" {
		t.Errorf("origen/etapa/estado: %s/%s/%s", tk.Origin, tk.Stage, tk.Status)
	}
	if tk.Title != "WA transferido: Juan Pérez" {
		t.Errorf("título %q", tk.Title)
	}
	if tk.ProfessionalPhone != "5491112345678" || tk.ExternalID != transferBase().ExternalID || tk.Reason != transferBase().Reason {
		t.Errorf("campos: phone=%q ext=%q reason=%q", tk.ProfessionalPhone, tk.ExternalID, tk.Reason)
	}
	// La descripción se lee de arriba abajo: quién y por qué, luego el chat.
	d := tk.Description
	iBy, iMotivo, iHist := strings.Index(d, "Lorena Moujalli"), strings.Index(d, "Motivo:"), strings.Index(d, "Historial del chat:")
	if iBy < 0 || iMotivo < 0 || iHist < 0 || !(iBy < iMotivo && iMotivo < iHist) {
		t.Errorf("la descripción no va en orden quién → motivo → historial:\n%s", d)
	}
	if !strings.Contains(d, "[10/09 14:33] Reclutador: Hola Juan...") {
		t.Error("el historial no está entero")
	}
}

// El mismo external_id no crea otro ticket: es lo que protege el reintento del
// 500 y una doble pulsación del manager.
func TestTransferencia_ElMismoIdNoDuplica(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := &ticketService{repo: repo}

	primera, _ := svc.CreateObersuiteTransfer(transferBase())
	segunda, err := svc.CreateObersuiteTransfer(transferBase())

	if err != nil || segunda.Status != "already_exists" || segunda.ID != primera.ID {
		t.Fatalf("el reintento debía devolver el mismo ticket: %v %v", segunda, err)
	}
	if repo.creados != 1 {
		t.Fatalf("se crearon %d tickets", repo.creados)
	}
}

func TestTransferencia_UnChoqueDeIndiceEsYaExiste(t *testing.T) {
	repo := &fakeTransferRepo{createErr: errors.New(`duplicate key value violates unique constraint "idx_tickets_external_id" (SQLSTATE 23505)`)}
	svc := &ticketService{repo: repo}

	// Sin fila del otro escritor: error (no era ese indice, o algo raro).
	if _, err := svc.CreateObersuiteTransfer(transferBase()); err == nil {
		t.Fatal("sin fila que devolver tras el choque, tiene que ser error")
	}

	// Con la fila del otro escritor visible tras el choque: already_exists.
	repo.apareceTrasCrear = &models.Ticket{ID: 42, ExternalID: transferBase().ExternalID}
	res, err := svc.CreateObersuiteTransfer(transferBase())
	if err != nil || res.Status != "already_exists" || res.ID != 42 {
		t.Fatalf("debia resolverse como already_exists: %v %v", res, err)
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
		{"sin motivo ni historial", func(i *ObersuiteTransferInput) { i.Reason = ""; i.Context = "" }, "reason o context"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			repo := &fakeTransferRepo{}
			svc := &ticketService{repo: repo}
			in := transferBase()
			c.muta(&in)

			_, err := svc.CreateObersuiteTransfer(in)

			if !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Fatalf("debía ser 400, got %v", err)
			}
			if !strings.Contains(err.Error(), c.frase) {
				t.Errorf("mensaje %q debía mencionar %q", err.Error(), c.frase)
			}
			if repo.creados != 0 {
				t.Error("un 400 no escribe nada")
			}
		})
	}
}

// El teléfono es opcional y el historial puede faltar si hay motivo (y al
// revés): un chat sin texto pero con motivo sigue siendo un ticket útil.
func TestTransferencia_LoOpcionalEsOpcional(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := &ticketService{repo: repo}
	in := transferBase()
	in.CandidatePhone = ""
	in.Context = ""

	if _, err := svc.CreateObersuiteTransfer(in); err != nil {
		t.Fatalf("debía aceptarse: %v", err)
	}
	if strings.Contains(repo.guardado.Description, "Historial del chat") {
		t.Error("sin historial no se pinta la sección")
	}
}

// Un chat de meses no cabe legible en un ticket: se conserva el FINAL, que es
// lo que Customer Success necesita, y se avisa del recorte.
func TestTransferencia_UnHistorialEnormeConservaElFinal(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := &ticketService{repo: repo}
	in := transferBase()
	in.Context = strings.Repeat("x", 30000) + "\nÚLTIMO MENSAJE"

	if _, err := svc.CreateObersuiteTransfer(in); err != nil {
		t.Fatal(err)
	}
	d := repo.guardado.Description
	if !strings.Contains(d, "ÚLTIMO MENSAJE") {
		t.Fatal("el final del chat tiene que sobrevivir al recorte")
	}
	if !strings.Contains(d, "historial recortado") {
		t.Fatal("y el recorte tiene que decirse")
	}
	if len([]rune(d)) > maxTransferContextRunes+500 {
		t.Fatalf("la descripción sigue siendo enorme: %d runas", len([]rune(d)))
	}
}
