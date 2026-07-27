package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
)

func dateP(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// El 'end.date' de un evento all-day de Google es EXCLUSIVO: una tarea de un solo
// día (start==end) debe ir de ese día al siguiente, o Google la muestra sin
// duración / en el día equivocado.
func TestBuildEventPayloadSingleDay(t *testing.T) {
	ev := CalendarEventInput{
		Summary:   "Tarea",
		StartDate: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}
	p := buildEventPayload(ev)
	if p.Start.Date != "2026-07-23" {
		t.Errorf("start = %q, se esperaba 2026-07-23", p.Start.Date)
	}
	if p.End.Date != "2026-07-24" {
		t.Errorf("end = %q, se esperaba 2026-07-24 (exclusivo)", p.End.Date)
	}
}

func TestBuildEventPayloadMultiDay(t *testing.T) {
	ev := CalendarEventInput{
		StartDate: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
	}
	p := buildEventPayload(ev)
	if p.Start.Date != "2026-07-23" || p.End.Date != "2026-07-26" {
		t.Errorf("rango = %s..%s, se esperaba 2026-07-23..2026-07-26", p.Start.Date, p.End.Date)
	}
}

// Un end anterior al start (dato inconsistente) no debe producir un rango
// invertido que Google rechace: se colapsa a un solo día.
func TestBuildEventPayloadEndBeforeStart(t *testing.T) {
	ev := CalendarEventInput{
		StartDate: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
	}
	p := buildEventPayload(ev)
	if p.Start.Date != "2026-07-23" || p.End.Date != "2026-07-24" {
		t.Errorf("rango = %s..%s, se esperaba colapsar a 2026-07-23..2026-07-24", p.Start.Date, p.End.Date)
	}
}

func TestTaskHasDate(t *testing.T) {
	if taskHasDate(&models.Task{}) {
		t.Error("una tarea sin fechas no tiene fecha")
	}
	if !taskHasDate(&models.Task{StartDate: dateP(2026, 7, 23)}) {
		t.Error("una tarea con start_date sí tiene fecha")
	}
	if !taskHasDate(&models.Task{EndDate: dateP(2026, 7, 23)}) {
		t.Error("una tarea con solo end_date (vence) sí tiene fecha")
	}
	if taskHasDate(nil) {
		t.Error("nil no tiene fecha")
	}
}

// Una tarea con solo fecha de vencimiento debe generar un evento en ese día
// (no en el año cero por un start vacío).
func TestBuildTaskEventInputOnlyEndDate(t *testing.T) {
	task := &models.Task{
		Title:   "Entregar informe",
		EndDate: dateP(2026, 7, 30),
	}
	in := buildTaskEventInput(task)
	if in.StartDate.Format("2006-01-02") != "2026-07-30" {
		t.Errorf("start = %s, se esperaba 2026-07-30 (cae al end)", in.StartDate.Format("2006-01-02"))
	}
	if in.EndDate.Format("2006-01-02") != "2026-07-30" {
		t.Errorf("end = %s, se esperaba 2026-07-30", in.EndDate.Format("2006-01-02"))
	}
}

// El título completado lleva ✓ para distinguirlo de un vistazo en el calendario.
func TestBuildTaskEventInputCompletedPrefix(t *testing.T) {
	task := &models.Task{
		Title:     "Revisar PR",
		Completed: true,
		EndDate:   dateP(2026, 7, 23),
	}
	in := buildTaskEventInput(task)
	if in.Summary != "✓ Revisar PR" {
		t.Errorf("summary = %q, se esperaba con prefijo ✓", in.Summary)
	}

	task.Completed = false
	if got := buildTaskEventInput(task).Summary; got != "Revisar PR" {
		t.Errorf("summary sin completar = %q, no debe llevar ✓", got)
	}
}

// La descripción siempre lleva la firma de Obertrack; si la tarea trae texto, va
// antes, separado.
func TestBuildTaskEventInputDescription(t *testing.T) {
	withText := buildTaskEventInput(&models.Task{Title: "T", Description: "Detalle", EndDate: dateP(2026, 7, 23)})
	if withText.Description != "Detalle\n\n— Sincronizado desde Obertrack" {
		t.Errorf("descripción con texto inesperada: %q", withText.Description)
	}
	noText := buildTaskEventInput(&models.Task{Title: "T", EndDate: dateP(2026, 7, 23)})
	if noText.Description != "— Sincronizado desde Obertrack" {
		t.Errorf("descripción sin texto inesperada: %q", noText.Description)
	}
}

// El calendario destino cae a 'primary' cuando no se especifica, y el path se
// arma bien para ids con caracteres especiales (correos).
func TestCalendarEventsURL(t *testing.T) {
	if got := calendarEventsURL(""); got != "https://www.googleapis.com/calendar/v3/calendars/primary/events" {
		t.Errorf("URL con calendario vacío = %q", got)
	}
	got := calendarEventsURL("user@example.com")
	want := "https://www.googleapis.com/calendar/v3/calendars/user@example.com/events"
	if got != want {
		t.Errorf("URL = %q, se esperaba %q", got, want)
	}
}
