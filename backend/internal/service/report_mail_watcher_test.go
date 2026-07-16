package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q) falló: %v (¿falta import _ \"time/tzdata\"?)", name, err)
	}
	return loc
}

// La zona IANA debe resolverse desde el binario (alpine no trae tzdata).
func TestLoadLocationCaracas(t *testing.T) {
	mustLoc(t, "America/Caracas")
}

func TestPeriodForDaily(t *testing.T) {
	loc := mustLoc(t, "America/Caracas")
	now := time.Date(2026, 7, 9, 8, 3, 0, 0, loc)

	start, end, key, title, label := periodFor(models.ReportFreqDaily, now, loc)

	if got := start.Format("2006-01-02"); got != "2026-07-08" {
		t.Errorf("start = %s, se esperaba 2026-07-08 (ayer)", got)
	}
	if !start.Equal(end) {
		t.Errorf("diario: start y end deben coincidir, got %v..%v", start, end)
	}
	if key != "d:2026-07-08" {
		t.Errorf("key = %q", key)
	}
	if title != "Reporte Diario de Jornadas" || label != "08/07/2026" {
		t.Errorf("title=%q label=%q", title, label)
	}
}

// Un jueves debe reportar el lunes..domingo de la semana ANTERIOR.
func TestPeriodForWeekly(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 7, 9, 8, 0, 0, 0, loc) // jueves

	start, end, key, _, _ := periodFor(models.ReportFreqWeekly, now, loc)

	if got := start.Format("2006-01-02"); got != "2026-06-29" {
		t.Errorf("start = %s, se esperaba 2026-06-29 (lunes previo)", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-07-05" {
		t.Errorf("end = %s, se esperaba 2026-07-05 (domingo previo)", got)
	}
	if start.Weekday() != time.Monday || end.Weekday() != time.Sunday {
		t.Errorf("el rango debe ir de lunes a domingo, got %v..%v", start.Weekday(), end.Weekday())
	}
	if key != "w:2026-W27" {
		t.Errorf("key = %q, se esperaba w:2026-W27", key)
	}
}

// Un domingo es el borde peligroso: Go pone Weekday()==0, y un cálculo ingenuo
// tomaría la semana equivocada.
func TestPeriodForWeeklyOnSunday(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 7, 5, 8, 0, 0, 0, loc) // domingo
	start, end, _, _, _ := periodFor(models.ReportFreqWeekly, now, loc)

	if got := start.Format("2006-01-02"); got != "2026-06-22" {
		t.Errorf("start = %s, se esperaba 2026-06-22", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-06-28" {
		t.Errorf("end = %s, se esperaba 2026-06-28", got)
	}
}

func TestPeriodForMonthly(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, loc)

	start, end, key, _, label := periodFor(models.ReportFreqMonthly, now, loc)

	if got := start.Format("2006-01-02"); got != "2026-06-01" {
		t.Errorf("start = %s, se esperaba 2026-06-01", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-06-30" {
		t.Errorf("end = %s, se esperaba 2026-06-30", got)
	}
	if key != "m:2026-06" {
		t.Errorf("key = %q", key)
	}
	if label != "Junio 2026" {
		t.Errorf("label = %q", label)
	}
}

// Enero debe reportar diciembre del año anterior.
func TestPeriodForMonthlyYearBoundary(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, loc)
	start, end, key, _, label := periodFor(models.ReportFreqMonthly, now, loc)

	if got := start.Format("2006-01-02"); got != "2025-12-01" {
		t.Errorf("start = %s, se esperaba 2025-12-01", got)
	}
	if got := end.Format("2006-01-02"); got != "2025-12-31" {
		t.Errorf("end = %s", got)
	}
	if key != "m:2025-12" || label != "Diciembre 2025" {
		t.Errorf("key=%q label=%q", key, label)
	}
}

func TestIsDue(t *testing.T) {
	loc := time.UTC
	daily := &models.ReportSchedule{Frequency: models.ReportFreqDaily, Hour: 8, Minute: 0}

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"justo en la hora", time.Date(2026, 7, 9, 8, 0, 0, 0, loc), true},
		{"dentro de la ventana", time.Date(2026, 7, 9, 8, 9, 0, 0, loc), true},
		{"fuera de la ventana", time.Date(2026, 7, 9, 8, 10, 0, 0, loc), false},
		{"antes de la hora", time.Date(2026, 7, 9, 7, 59, 0, 0, loc), false},
		{"mucho despues", time.Date(2026, 7, 9, 20, 0, 0, 0, loc), false},
	}
	for _, tc := range cases {
		if got := isDue(daily, tc.now); got != tc.want {
			t.Errorf("%s: isDue = %v, se esperaba %v", tc.name, got, tc.want)
		}
	}

	// Semanal: solo dispara el día configurado.
	weekly := &models.ReportSchedule{Frequency: models.ReportFreqWeekly, Hour: 8, Weekday: int(time.Monday)}
	if isDue(weekly, time.Date(2026, 7, 9, 8, 0, 0, 0, loc)) { // jueves
		t.Error("semanal no debe disparar un jueves si está configurado el lunes")
	}
	if !isDue(weekly, time.Date(2026, 7, 6, 8, 0, 0, 0, loc)) { // lunes
		t.Error("semanal debe disparar el lunes configurado")
	}

	// Mensual: solo el día del mes configurado.
	monthly := &models.ReportSchedule{Frequency: models.ReportFreqMonthly, Hour: 8, DayOfMonth: 1}
	if isDue(monthly, time.Date(2026, 7, 2, 8, 0, 0, 0, loc)) {
		t.Error("mensual no debe disparar el día 2 si está configurado el 1")
	}
	if !isDue(monthly, time.Date(2026, 7, 1, 8, 0, 0, 0, loc)) {
		t.Error("mensual debe disparar el día 1")
	}
}
