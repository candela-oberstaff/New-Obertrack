package handlers

import (
	"testing"

	"github.com/obertrack/backend/internal/middleware"
)

// El canje redirige a la vista de cada alcance (contrato CRM, §4).
func TestEmbedPathFor(t *testing.T) {
	cases := map[string]string{
		middleware.ScopeTasks: "/embed/tareas",
		middleware.ScopeHours: "/embed/horas",
	}
	for scope, want := range cases {
		if got := embedPathFor(scope); got != want {
			t.Errorf("embedPathFor(%q) = %q, esperaba %q", scope, got, want)
		}
	}
}
