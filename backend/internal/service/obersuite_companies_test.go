package service

import (
	"encoding/json"
	"strings"
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
	contacto := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	cambio := time.Date(2026, 9, 8, 18, 30, 0, 0, time.UTC)
	return repository.ObersuiteCompanyRecord{
		ID: 10, Name: "Acme S.A", IsActive: true,
		ResponsibleName: "Osvell Empresa", ResponsibleEmail: "osvell@oberstaff.com",
		Industry: "Tecnología", Country: "España", State: "Aragón", City: "Zaragoza",
		Address:            "Calle Mayor 1",
		ProfessionalsCount: 6, BoardsCount: 3, TasksCount: 32,
		LastContactAt: &contacto,
		UpdatedAt:     cambio,
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
		{"name", c.Name, "Acme S.A"},
		{"responsible_email", c.ResponsibleEmail, "osvell@oberstaff.com"},
		{"industry", c.Industry, "Tecnología"},
		{"city", c.City, "Zaragoza"},
		{"address", c.Address, "Calle Mayor 1"},
		{"professionals_count", c.ProfessionalsCount, 6},
		{"boards_count", c.BoardsCount, 3},
		{"tasks_count", c.TasksCount, 32},
		{"updated_at", c.UpdatedAt, fichaCompleta().UpdatedAt},
	}
	for _, tc := range casos {
		if tc.got != tc.want {
			t.Errorf("%s = %v, se esperaba %v", tc.campo, tc.got, tc.want)
		}
	}

	// Las fechas opcionales viajan como puntero: nil significa "nunca pasó", y
	// aplanarlas a la fecha cero le diría a Obersuite que ocurrió en el año 1.
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

// Los campos retirados, y por qué — que es lo que se pierde si solo queda la
// lista:
//
//   - last_activity_at y los de operación (horas, pendientes, rechazadas):
//     nadie los consume, y last_activity_at además rompía el ETag, porque lo
//     alimenta el contador de uso, que escribe cada 30 segundos. Encima
//     exponían lo que trabaja la gente de cada cliente.
//   - location: texto libre que la gente rellena a mano; ya viajan
//     country/state/city, que sí están normalizados.
//   - created_at, client_since, phone_number y open_tickets: Obersuite
//     confirmó que no los lee en ningún sitio. open_tickets era el peor de
//     los cuatro: costaba una subconsulta con normalización de teléfonos y
//     habría rotado el ETag para llenar una pestaña que hoy enseña un número
//     escrito a mano.
//
// Esta prueba mira el JSON de salida y no la struct: es ahí donde se nota la
// reaparición. Falla en cuanto vuelva cualquiera de ellos.
func TestListCompanies_NoSalenLosCamposRetirados(t *testing.T) {
	repo := &fakeCompaniesRepo{record: fichaCompleta()}
	svc := &onboardingService{userRepo: repo}

	companies, err := svc.ListCompanies(nil)
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	crudo, err := json.Marshal(companies[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, campo := range []string{
		"last_activity_at", "hours_this_month", "pending_hours",
		"pending_count", "rejected_count", "location",
		"created_at", "client_since", "phone_number", "open_tickets",
	} {
		if strings.Contains(string(crudo), campo) {
			t.Errorf("%q volvió a salir en el JSON hacia Obersuite", campo)
		}
	}
}
