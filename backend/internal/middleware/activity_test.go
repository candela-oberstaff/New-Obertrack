package middleware

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// El mapa de módulos es lo que decide si el panel de Customer Success dice la
// verdad. Estos casos fijan las dos reglas que no son obvias leyendo la tabla.
func TestActivityModuleFor(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// Rutas distintas, mismo módulo de producto: la persona no sabe que
		// tableros y tarjetas viven en dos grupos de rutas.
		{"/api/tasks", models.ModuleTasks},
		{"/api/boards/:id", models.ModuleTasks},
		{"/api/channels/:id/messages", models.ModuleChat},
		{"/api/chat/messages", models.ModuleChat},
		{"/api/work-hours", models.ModuleHours},
		{"/api/me/wallet", models.ModuleWallet},

		// Rutas técnicas: cuentan como uso de la app, nunca como módulo.
		{"/api/uploads/avatar", ""},
		{"/api/metrics/usage", ""},
		{"/api/auth/login", ""},
	}
	for _, tc := range cases {
		if got := activityModuleFor(tc.path); got != tc.want {
			t.Errorf("activityModuleFor(%q) = %q, se esperaba %q", tc.path, got, tc.want)
		}
	}
}

// El sondeo del sidebar es la trampa de esta métrica: el badge de no leídos
// pregunta cada 30 segundos desde CUALQUIER pantalla. Si contara como chat, el
// "% de uso del chat" sería 100% para siempre y la métrica que más le importa a
// Customer Success sería exactamente la inservible.
func TestActivityPollPathsNoCuentanComoModulo(t *testing.T) {
	for path := range activityPollPaths {
		if !activityPollPaths[path] {
			t.Fatalf("%s debería estar marcada como sondeo", path)
		}
	}
	if !activityPollPaths["/api/channels/unread/total"] {
		t.Error("el badge de mensajes sin leer del sidebar debe quedar fuera del módulo chat")
	}
	if !activityPollPaths["/api/tutorials/pending"] {
		t.Error("la cola de Novedades del layout debe quedar fuera del módulo novedades")
	}
	// Y la ruta real del chat sí cuenta: excluir el sondeo no puede llevarse
	// por delante el uso de verdad.
	if activityPollPaths["/api/channels/:id/messages"] {
		t.Error("leer un canal es uso real del chat, no un sondeo")
	}
}
