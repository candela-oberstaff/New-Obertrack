package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// fakeReportWHRepo devuelve las jornadas fijas del período.
type fakeReportWHRepo struct {
	repository.WorkHourRepository
	hours []models.WorkHour
}

func (f *fakeReportWHRepo) FindAll(map[string]interface{}, int, int) ([]models.WorkHour, int64, error) {
	return f.hours, int64(len(f.hours)), nil
}

// fakeMembersRepo devuelve los profesionales activos de la empresa.
type fakeMembersRepo struct {
	repository.EmploymentRepository
	members []repository.ActiveProfessional
}

func (f *fakeMembersRepo) ListActiveProfessionals(uint) ([]repository.ActiveProfessional, error) {
	return f.members, nil
}

// sendReportFor corre SendPeriodReport contra un Brevo falso y devuelve el
// correo que habría salido.
func sendReportFor(t *testing.T, hours []models.WorkHour, members []repository.ActiveProfessional) BrevoEmailRequest {
	t.Helper()
	var got BrevoEmailRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("payload ilegible: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"x"}`))
	}))
	t.Cleanup(srv.Close)

	s := &workHourService{
		repo:           &fakeReportWHRepo{hours: hours},
		employmentRepo: &fakeMembersRepo{members: members},
		brevoSvc:       &BrevoService{apiKey: "clave-de-prueba", apiURL: srv.URL, from: BrevoContact{Name: "Obertrack", Email: "noreply@obertrack.com"}},
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	recipient := &models.User{ID: 310, Name: "Rodrigo", Email: "rodrigo@empresa.com"}
	if err := s.SendPeriodReport(recipient, 310, "Reporte Mensual de Jornadas", "Septiembre 2026", start, end); err != nil {
		t.Fatalf("SendPeriodReport: %v", err)
	}
	return got
}

var julio = time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

// El caso que motivó el cambio: una empresa cuyos profesionales no registraron
// nada recibía el reporte con todo en 0 y lo leía como una avería.
func TestReport_EmptyPeriodSendsExplicitNotice(t *testing.T) {
	got := sendReportFor(t, nil, []repository.ActiveProfessional{
		{UserID: 438, Name: "Cris", StartedAt: julio},
		{UserID: 439, Name: "Newgen", StartedAt: julio},
		// Entró después de cerrar el período: no tenía nada que registrar.
		{UserID: 440, Name: "Nuevo de octubre", StartedAt: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)},
	})

	if !strings.Contains(got.Subject, "Sin jornadas registradas") {
		t.Errorf("el asunto debe avisar que no hay jornadas, got %q", got.Subject)
	}
	if len(got.Attachment) != 0 {
		t.Errorf("sin jornadas no hay que adjuntar PDF/Excel vacíos, got %d adjuntos", len(got.Attachment))
	}
	html := got.HTMLContent
	for _, want := range []string{"No hay jornadas registradas en Septiembre 2026", "Cris", "Newgen"} {
		if !strings.Contains(html, want) {
			t.Errorf("el correo debe incluir %q", want)
		}
	}
	if strings.Contains(html, "Nuevo de octubre") {
		t.Error("quien entró después del período no debe aparecer como sin registros")
	}
	if strings.Contains(html, "Horas Totales") {
		t.Error("el aviso no debe mostrar las tarjetas de cifras en 0")
	}
}

// Con jornadas, el reporte es el de siempre (cifras y adjuntos) y además
// señala a los profesionales activos que no registraron nada.
func TestReport_WithHoursListsIdleProfessionals(t *testing.T) {
	got := sendReportFor(t, []models.WorkHour{
		{UserID: 438, WorkType: models.WorkTypeComplete, HoursWorked: 8, Approved: true, WorkDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), User: models.User{Name: "Cris"}},
	}, []repository.ActiveProfessional{
		{UserID: 438, Name: "Cris", StartedAt: julio},
		{UserID: 439, Name: "Newgen", StartedAt: julio},
	})

	if !strings.Contains(got.Subject, "Reporte de Jornadas") {
		t.Errorf("con jornadas va el reporte normal, got %q", got.Subject)
	}
	if len(got.Attachment) != 2 {
		t.Errorf("el reporte normal lleva PDF y Excel, got %d adjuntos", len(got.Attachment))
	}
	html := got.HTMLContent
	if !strings.Contains(html, "8.0 h") || !strings.Contains(html, "Sin jornadas en este período") {
		t.Error("debe traer las cifras y la sección de profesionales sin registros")
	}
	idle := html[strings.Index(html, "Sin jornadas en este período"):]
	if !strings.Contains(idle, "Newgen") || strings.Contains(idle, "Cris") {
		t.Error("solo Newgen, que no registró nada, debe aparecer en la lista")
	}
}
