package service

import (
	"strings"
	"testing"
)

// La capacitación no es una bienvenida: le llega a quien ya trabaja y no
// cambia su acceso.
func TestBuildTrainingInviteHTML(t *testing.T) {
	html := BuildTrainingInviteHTML("Marta", "Seguridad 2026", 3, "https://obertrack.com/induccion/tok")
	for _, want := range []string{"Seguridad 2026", "3 bloques", "tu acceso a la plataforma no cambia", "https://obertrack.com/induccion/tok"} {
		if !strings.Contains(html, want) {
			t.Errorf("el correo debe incluir %q", want)
		}
	}
	if strings.Contains(html, "Bienvenido") || strings.Contains(html, "contraseña") {
		t.Error("una capacitación no debe hablar de bienvenida ni de crear contraseña")
	}
	if !strings.Contains(BuildTrainingInviteHTML("Marta", "X", 1, "l"), "1 bloque") {
		t.Error("con un bloque, en singular")
	}
}
