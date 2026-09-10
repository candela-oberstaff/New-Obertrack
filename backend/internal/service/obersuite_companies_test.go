package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/repository"
)

// fakeCompaniesRepo devuelve una ficha fija y anota con qué corte se le pidió.
type fakeCompaniesRepo struct {
	repository.UserRepository
	record     repository.ObersuiteCompanyRecord
	gotSince   *time.Time
	sinceCalls int
}

func (f *fakeCompaniesRepo) GetObersuiteCompanies(updatedSince *time.Time) ([]repository.ObersuiteCompanyRecord, error) {
	f.gotSince = updatedSince
	f.sinceCalls++
	return []repository.ObersuiteCompanyRecord{f.record}, nil
}

func fichaCompleta() repository.ObersuiteCompanyRecord {
	alta := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	cliente := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	contacto := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	actividad := time.Date(2026, 9, 8, 18, 30, 0, 0, time.UTC)
	return repository.ObersuiteCompanyRecord{
		ID: 10, Name: "Acme S.A", IsActive: true,
		ResponsibleName: "Osvell Empresa", ResponsibleEmail: "osvell@oberstaff.com",
		Industry: "Tecnología", Country: "España", State: "Aragón", City: "Zaragoza",
		Address: "Calle Mayor 1", Location: "Las Lomas", PhoneNumber: "+34606904974",
		CreatedAt: alta, ClientSince: &cliente,
		ProfessionalsCount: 6, BoardsCount: 3, TasksCount: 32,
		HoursThisMonth: 8, PendingHours: 32, PendingCount: 4, RejectedCount: 2, OpenTickets: 1,
		LastContactAt: &contacto, LastActivityAt: &actividad,
		UpdatedAt: actividad,
	}
}

// La ficha viaja ENTERA hasta Obersuite. Es una copia campo a campo, y ese es
// justo el sitio donde se pierde uno sin que nada falle: el JSON sale con el
// campo en cero y del otro lado parece un dato real.
func TestListCompanies_LaFichaViajaCompleta(t *testing.T) {
	repo := &fakeCompaniesRepo{record: fichaCompleta()}
	svc := &onboardingService{userRepo: repo}

	companies, err := svc.ListCompanies(nil)
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("se esperaba 1 empresa, hubo %d", len(companies))
	}
	c := companies[0]

	casos := []struct {
		campo string
		got   interface{}
		want  interface{}
	}{
		{"location", c.Location, "Las Lomas"},
		{"phone_number", c.PhoneNumber, "+34606904974"},
		{"created_at", c.CreatedAt, fichaCompleta().CreatedAt},
		{"professionals_count", c.ProfessionalsCount, 6},
		{"boards_count", c.BoardsCount, 3},
		{"tasks_count", c.TasksCount, 32},
		{"hours_this_month", c.HoursThisMonth, 8.0},
		{"pending_hours", c.PendingHours, 32.0},
		{"pending_count", c.PendingCount, 4},
		{"rejected_count", c.RejectedCount, 2},
		{"open_tickets", c.OpenTickets, 1},
		{"updated_at", c.UpdatedAt, fichaCompleta().UpdatedAt},
	}
	for _, tc := range casos {
		if tc.got != tc.want {
			t.Errorf("%s = %v, se esperaba %v", tc.campo, tc.got, tc.want)
		}
	}

	// Las fechas opcionales viajan como puntero: nil significa "nunca pasó", y
	// aplanarlas a la fecha cero le diría a Obersuite que ocurrió en el año 1.
	if c.ClientSince == nil || !c.ClientSince.Equal(*fichaCompleta().ClientSince) {
		t.Errorf("client_since = %v", c.ClientSince)
	}
	if c.LastActivityAt == nil || !c.LastActivityAt.Equal(*fichaCompleta().LastActivityAt) {
		t.Errorf("last_activity_at = %v", c.LastActivityAt)
	}
	if c.LastContactAt == nil || !c.LastContactAt.Equal(*fichaCompleta().LastContactAt) {
		t.Errorf("last_contact_at = %v", c.LastContactAt)
	}
}

// Obersuite ya consumía `status` y `last_contact` (texto). Se conservan tal
// cual: renombrarlos o quitarlos le rompe la integración en silencio.
func TestListCompanies_ConservaLosCamposQueObersuiteYaConsumia(t *testing.T) {
	rec := fichaCompleta()
	rec.IsActive = false
	repo := &fakeCompaniesRepo{record: rec}
	svc := &onboardingService{userRepo: repo}

	companies, _ := svc.ListCompanies(nil)
	c := companies[0]

	if c.Status != "suspended" {
		t.Errorf("status = %q, se esperaba suspended", c.Status)
	}
	// El texto en español sigue saliendo, y ADEMÁS va la fecha en crudo: el
	// primero es para mostrar, la segunda para ordenar o comparar.
	if c.LastContact == "" {
		t.Error("last_contact no debería venir vacío")
	}
	if c.LastContactAt == nil {
		t.Error("last_contact_at debe acompañar al texto")
	}
}

// El corte incremental llega hasta el repositorio. Si se perdiera por el camino
// se devolvería el padrón entero y Obersuite creería estar recibiendo solo lo
// que cambió — el peor fallo posible aquí, porque no se nota.
func TestListCompanies_PasaElCorteIncrementalAlRepositorio(t *testing.T) {
	corte := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeCompaniesRepo{record: fichaCompleta()}
	svc := &onboardingService{userRepo: repo}

	if _, err := svc.ListCompanies(&corte); err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if repo.gotSince == nil {
		t.Fatal("el corte no llegó al repositorio")
	}
	if !repo.gotSince.Equal(corte) {
		t.Errorf("corte = %v, se esperaba %v", repo.gotSince, corte)
	}

	// Y sin corte se piden todas, que es como llamaba Obersuite hasta ahora.
	repo.gotSince = &corte
	if _, err := svc.ListCompanies(nil); err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if repo.gotSince != nil {
		t.Errorf("sin corte debía pasarse nil, se pasó %v", repo.gotSince)
	}
}
