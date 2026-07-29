package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// mustLoc vive en report_mail_watcher_test.go: carga una zona IANA y falla el
// test si no está, que es la señal de que falta el import _ "time/tzdata".

func TestParseRecurrenceEmptyIsSingleSession(t *testing.T) {
	rec, err := ParseRecurrence("")
	if err != nil || rec != nil {
		t.Errorf("una regla vacía es una sesión única, no un error: rec=%v err=%v", rec, err)
	}
}

func TestParseRecurrenceRoundTrip(t *testing.T) {
	cases := map[string]string{
		"RRULE:FREQ=DAILY":                        "RRULE:FREQ=DAILY",
		"RRULE:FREQ=WEEKLY;INTERVAL=2":            "RRULE:FREQ=WEEKLY;INTERVAL=2",
		"RRULE:FREQ=MONTHLY;COUNT=6":              "RRULE:FREQ=MONTHLY;COUNT=6",
		"RRULE:FREQ=DAILY;UNTIL=20260930T235959Z": "RRULE:FREQ=DAILY;UNTIL=20260930T235959Z",
		// INTERVAL=1 es el defecto y se omite al normalizar, para que en la BD no
		// convivan dos formas de decir lo mismo.
		"RRULE:FREQ=DAILY;INTERVAL=1": "RRULE:FREQ=DAILY",
		// Sin el prefijo y en minúsculas también debe entenderse.
		"freq=weekly": "RRULE:FREQ=WEEKLY",
	}
	for input, want := range cases {
		rec, err := ParseRecurrence(input)
		if err != nil {
			t.Errorf("ParseRecurrence(%q): %v", input, err)
			continue
		}
		if got := rec.String(); got != want {
			t.Errorf("ParseRecurrence(%q).String() = %q, se esperaba %q", input, got, want)
		}
	}
}

func TestParseRecurrenceRejectsBadRules(t *testing.T) {
	bad := []string{
		"RRULE:FREQ=HOURLY",                               // frecuencia que no sabemos calcular
		"RRULE:FREQ=DAILY;BYDAY=MO",                       // opción no soportada
		"RRULE:FREQ=DAILY;INTERVAL=0",                     // intervalo sin sentido
		"RRULE:FREQ=DAILY;COUNT=0",                        // idem
		"RRULE:FREQ=DAILY;UNTIL=mañana",                   // fecha ilegible
		"RRULE:FREQ=DAILY;COUNT=5;UNTIL=20260930T235959Z", // dos finales a la vez
		"RRULE:DAILY",                                     // tramo sin '='
	}
	for _, rule := range bad {
		if _, err := ParseRecurrence(rule); !errors.Is(err, ErrMeetingValidation) {
			t.Errorf("ParseRecurrence(%q) devolvió %v, se esperaba ErrMeetingValidation", rule, err)
		}
	}
}

// El caso que motivó todo esto: una serie diaria debe seguir teniendo próxima
// ocurrencia días después de la primera.
func TestNextOccurrenceDailySurvivesFirstDay(t *testing.T) {
	loc := mustLoc(t, "America/Caracas")
	start := time.Date(2026, 7, 28, 15, 0, 0, 0, loc)
	rec, _ := ParseRecurrence("RRULE:FREQ=DAILY")

	// Tres días después, a las 20:00: la de hoy ya pasó, toca la de mañana.
	now := time.Date(2026, 7, 31, 20, 0, 0, 0, loc)
	next, ok := rec.NextOccurrence(start, time.Hour, loc, now)
	if !ok {
		t.Fatal("una serie diaria sin fin siempre tiene próxima ocurrencia")
	}
	want := time.Date(2026, 8, 1, 15, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Errorf("próxima = %s, se esperaba %s", next, want)
	}
}

// Una reunión ya empezada pero todavía en curso cuenta como próxima: es a la que
// la gente se va a unir.
func TestNextOccurrenceIncludesMeetingInProgress(t *testing.T) {
	loc := mustLoc(t, "America/Caracas")
	start := time.Date(2026, 7, 28, 15, 0, 0, 0, loc)
	rec, _ := ParseRecurrence("RRULE:FREQ=DAILY")

	now := time.Date(2026, 7, 28, 15, 30, 0, 0, loc) // dentro de la primera
	next, ok := rec.NextOccurrence(start, time.Hour, loc, now)
	if !ok || !next.Equal(start) {
		t.Errorf("próxima = %s (ok=%v), se esperaba la que está en curso (%s)", next, ok, start)
	}
}

// La razón de guardar la zona IANA y no un offset: la reunión debe seguir siendo
// a la misma hora local después del cambio de horario de verano.
func TestNextOccurrenceKeepsLocalTimeAcrossDST(t *testing.T) {
	loc := mustLoc(t, "America/New_York")
	// 1 de marzo de 2026, 09:00 EST. El horario de verano entra el 8 de marzo.
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, loc)
	rec, _ := ParseRecurrence("RRULE:FREQ=WEEKLY")

	now := time.Date(2026, 3, 9, 0, 0, 0, 0, loc)
	next, ok := rec.NextOccurrence(start, time.Hour, loc, now)
	if !ok {
		t.Fatal("la serie sigue viva")
	}
	if h, m, _ := next.Clock(); h != 9 || m != 0 {
		t.Errorf("la ocurrencia quedó a las %02d:%02d; debe seguir siendo a las 09:00 locales", h, m)
	}
}

func TestNextOccurrenceRespectsCount(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 7, 28, 15, 0, 0, 0, loc)
	rec, _ := ParseRecurrence("RRULE:FREQ=DAILY;COUNT=3")

	// Dentro de la tercera y última: todavía hay ocurrencia.
	if _, ok := rec.NextOccurrence(start, time.Hour, loc, time.Date(2026, 7, 30, 15, 30, 0, 0, loc)); !ok {
		t.Error("la tercera ocurrencia todavía cuenta")
	}
	// Pasada la tercera: la serie se agotó.
	if _, ok := rec.NextOccurrence(start, time.Hour, loc, time.Date(2026, 7, 31, 0, 0, 0, 0, loc)); ok {
		t.Error("con COUNT=3 no debe haber una cuarta ocurrencia")
	}
}

func TestNextOccurrenceRespectsUntil(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 7, 28, 15, 0, 0, 0, loc)
	rec, _ := ParseRecurrence("RRULE:FREQ=DAILY;UNTIL=20260730T235959Z")

	if _, ok := rec.NextOccurrence(start, time.Hour, loc, time.Date(2026, 7, 30, 10, 0, 0, 0, loc)); !ok {
		t.Error("el 30 todavía entra en la serie")
	}
	if _, ok := rec.NextOccurrence(start, time.Hour, loc, time.Date(2026, 8, 1, 0, 0, 0, 0, loc)); ok {
		t.Error("pasado el UNTIL no debe haber más ocurrencias")
	}
}

// Sin regla, la "serie" es la propia sesión: hay próxima mientras no termine.
func TestNextOccurrenceWithoutRule(t *testing.T) {
	var rec *Recurrence
	start := time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)

	if _, ok := rec.NextOccurrence(start, time.Hour, time.UTC, start.Add(30*time.Minute)); !ok {
		t.Error("una sesión en curso sigue siendo próxima")
	}
	if _, ok := rec.NextOccurrence(start, time.Hour, time.UTC, start.Add(2*time.Hour)); ok {
		t.Error("una sesión única terminada no tiene próxima ocurrencia")
	}
}

func TestSeriesEnd(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 7, 28, 15, 0, 0, 0, loc)

	// Sin regla: termina cuando termina la sesión.
	var none *Recurrence
	if got := none.SeriesEnd(start, time.Hour, loc); got == nil || !got.Equal(start.Add(time.Hour)) {
		t.Errorf("sesión única: fin = %v, se esperaba %s", got, start.Add(time.Hour))
	}

	// Sin fin: NULL, que es lo que hace que el listado la considere viva siempre.
	infinite, _ := ParseRecurrence("RRULE:FREQ=DAILY")
	if got := infinite.SeriesEnd(start, time.Hour, loc); got != nil {
		t.Errorf("una serie sin fin debe dar nil, dio %s", got)
	}

	// COUNT=3 → termina al acabar la tercera.
	counted, _ := ParseRecurrence("RRULE:FREQ=DAILY;COUNT=3")
	want := time.Date(2026, 7, 30, 16, 0, 0, 0, loc)
	if got := counted.SeriesEnd(start, time.Hour, loc); got == nil || !got.Equal(want) {
		t.Errorf("COUNT=3: fin = %v, se esperaba %s", got, want)
	}

	// UNTIL: el fin real incluye la duración, o una reunión en curso el último
	// día quedaría fuera de "próximas".
	until, _ := ParseRecurrence("RRULE:FREQ=DAILY;UNTIL=20260730T150000Z")
	if got := until.SeriesEnd(start, time.Hour, loc); got == nil || !got.After(*until.Until) {
		t.Errorf("UNTIL: el fin debe incluir la duración, dio %v", got)
	}
}

// decorate es lo que hace que el listado muestre la fecha correcta y en orden.
func TestDecorateSortsByNextOccurrence(t *testing.T) {
	loc := "UTC"
	now := time.Now().UTC()

	// Serie diaria empezada hace un mes: su start_at es viejo, pero su próxima
	// reunión es hoy. Ordenar por start_at la pondría antes que todo.
	daily := models.MeetingSession{
		ID: 1, TimeZone: loc,
		StartAt:        now.AddDate(0, 0, -30).Truncate(time.Hour),
		EndAt:          now.AddDate(0, 0, -30).Truncate(time.Hour).Add(time.Hour),
		RecurrenceRule: "RRULE:FREQ=DAILY",
	}
	// Sesión única dentro de dos horas.
	soon := models.MeetingSession{
		ID: 2, TimeZone: loc,
		StartAt: now.Add(2 * time.Hour), EndAt: now.Add(3 * time.Hour),
	}
	// Serie ya agotada: sin próxima ocurrencia, va al final.
	expired := models.MeetingSession{
		ID: 3, TimeZone: loc,
		StartAt:        now.AddDate(0, 0, -10),
		EndAt:          now.AddDate(0, 0, -10).Add(time.Hour),
		RecurrenceRule: "RRULE:FREQ=DAILY;COUNT=2",
	}

	got := decorate([]models.MeetingSession{expired, soon, daily})

	if got[len(got)-1].ID != 3 {
		t.Errorf("la serie agotada debe quedar al final, quedó: %d", got[len(got)-1].ID)
	}
	if got[len(got)-1].NextStartAt != nil {
		t.Error("una serie agotada no debe tener próxima ocurrencia")
	}
	for _, s := range got {
		if s.ID == 1 && s.NextStartAt == nil {
			t.Fatal("la serie diaria debe tener próxima ocurrencia pese a su start_at viejo")
		}
		if s.ID == 1 && s.NextStartAt.Before(now.Add(-2*time.Hour)) {
			t.Errorf("la próxima de la serie diaria quedó en el pasado: %s", s.NextStartAt)
		}
	}
}
