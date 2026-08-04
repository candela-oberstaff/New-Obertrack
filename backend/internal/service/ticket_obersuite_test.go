package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Las altas llegadas de Obersuite comparten bandeja, ficha y gestión con las
// alertas internas. Estos tests fijan las dos cosas que eso implica: que la
// bandeja las pida, y que el arreglo de las alertas de rechazo no se les
// aplique.

type fakeTicketRepo struct {
	repository.TicketRepository

	pedidos  []string
	tickets  []models.Ticket
	guardado *models.Ticket
	abierto  *models.Ticket
}

func (f *fakeTicketRepo) ListByOrigins(origins ...string) ([]models.Ticket, error) {
	f.pedidos = origins
	return f.tickets, nil
}

func (f *fakeTicketRepo) FindOpenByUserAndOrigin(userID uint, origin string) (*models.Ticket, error) {
	return f.abierto, nil
}

func (f *fakeTicketRepo) CreateTicket(t *models.Ticket) error {
	t.ID = 500
	f.guardado = t
	return nil
}

func (f *fakeTicketRepo) SaveTicket(t *models.Ticket) error {
	f.guardado = t
	return nil
}

// La bandeja de soporte enseña las dos cosas juntas: si solo pidiera las
// alertas internas, una incorporación no aparecería en ninguna pantalla.
func TestListInternal_PideAlertasYAltasDeObersuite(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := &ticketService{repo: repo}

	if _, err := svc.ListInternal(); err != nil {
		t.Fatalf("list: %v", err)
	}
	quiere := map[string]bool{models.OriginInternal: false, models.OriginObersuite: false}
	for _, o := range repo.pedidos {
		if _, ok := quiere[o]; ok {
			quiere[o] = true
		}
	}
	for origen, pedido := range quiere {
		if !pedido {
			t.Fatalf("la bandeja no pidió el origen %q (pidió %v)", origen, repo.pedidos)
		}
	}
}

// El apaño que rellena fechas y motivo mirando la descripción existe para las
// alertas de rechazo viejas. En un alta tomaría cualquier paréntesis del texto
// como fechas de jornada, y la ficha enseñaría un dato inventado.
func TestEnrichInternal_NoInventaFechasEnUnAltaDeObersuite(t *testing.T) {
	svc := &ticketService{}

	alta := &models.Ticket{
		Origin:      models.OriginObersuite,
		Description: "Contratado en Obersuite. No se le envió capacitación (la empresa la tiene desactivada). Motivo: revisar.",
	}
	svc.enrichInternal(alta)
	if alta.WorkDates != "" || alta.Reason != "" {
		t.Fatalf("no debía rellenar nada: fechas=%q motivo=%q", alta.WorkDates, alta.Reason)
	}

	// La alerta de rechazo sí lo necesita: es para lo que se escribió.
	rechazo := &models.Ticket{
		Origin:      models.OriginInternal,
		Description: "Jornadas rechazadas (12/07, 13/07). Motivo: sin reporte",
	}
	svc.enrichInternal(rechazo)
	if rechazo.WorkDates != "12/07, 13/07" || rechazo.Reason != "sin reporte" {
		t.Fatalf("el rechazo perdió su relleno: fechas=%q motivo=%q", rechazo.WorkDates, rechazo.Reason)
	}
}

// Una re-contratación no es un segundo caso que atender: si ya hay una
// incorporación abierta para esa persona, no se abre otra.
func TestCreateObersuiteHireAlert_NoDuplicaSiYaHayUnoAbierto(t *testing.T) {
	repo := &fakeTicketRepo{abierto: &models.Ticket{ID: 7, Origin: models.OriginObersuite}}
	svc := &ticketService{repo: repo}

	if err := svc.CreateObersuiteHireAlert(ObersuiteHireInput{ProfessionalID: 3, ProfessionalName: "Ana"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.guardado != nil {
		t.Fatalf("no debía crear un segundo ticket: %+v", repo.guardado)
	}
}

// Aprobar la capacitación es lo que cierra la incorporación.
func TestCloseObersuiteHireAlert_CierraElTicketAbierto(t *testing.T) {
	repo := &fakeTicketRepo{abierto: &models.Ticket{ID: 7, Origin: models.OriginObersuite, Stage: models.StageNew, Status: "open"}}
	svc := &ticketService{repo: repo}

	if err := svc.CloseObersuiteHireAlert(3); err != nil {
		t.Fatalf("close: %v", err)
	}
	if repo.guardado == nil || repo.guardado.Stage != models.StageClosed || repo.guardado.Status != "closed" {
		t.Fatalf("el ticket no quedó cerrado: %+v", repo.guardado)
	}
}

// Sin ticket abierto (altas anteriores a esto, o inducción desactivada) aprobar
// no puede fallar: el acceso del profesional no depende de un ticket.
func TestCloseObersuiteHireAlert_SinTicketNoFalla(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := &ticketService{repo: repo}

	if err := svc.CloseObersuiteHireAlert(3); err != nil {
		t.Fatalf("no debía fallar sin ticket: %v", err)
	}
}
