package service

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/repository"
)

func TestInactivityTitle_Plural(t *testing.T) {
	if got := inactivityTitle(1); !strings.HasPrefix(got, "1 profesional lleva ") {
		t.Errorf("singular: %q", got)
	}
	if got := inactivityTitle(69); !strings.HasPrefix(got, "69 profesionales llevan ") || strings.Contains(got, "(es)") {
		t.Errorf("plural: %q", got)
	}
}

// El resumen nombra a los tres más atrasados y cuenta el resto.
func TestInactivitySummary_LosMasAtrasadosYElResto(t *testing.T) {
	cases := []repository.InactiveUser{
		{ID: 1, Name: "Ana", DaysInactive: 5},
		{ID: 2, Name: "Carlos", DaysInactive: 81},
		{ID: 3, Name: "Daniela", DaysInactive: 60},
		{ID: 4, Name: "Johelys", DaysInactive: 59},
		{ID: 5, Name: "Simone", DaysInactive: 3},
	}
	got := inactivitySummary(cases)
	want := "Carlos (81 días), Daniela (60 días), Johelys (59 días) y 2 más. Revísalos en Actividad."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := inactivitySummary(cases[:2]); got != "Carlos (81 días), Ana (5 días). Revísalos en Actividad." {
		t.Fatalf("sin resto: %q", got)
	}
}
