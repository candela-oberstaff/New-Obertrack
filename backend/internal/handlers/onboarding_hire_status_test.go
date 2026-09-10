package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/obertrack/backend/internal/apperrors"
)

// El contrato de códigos acordado con Obersuite. Su lógica de reintentos
// depende literalmente de esta tabla: los 4xx no se reintentan, el 500 sí.
//
// La prueba existe porque ellos ya se quemaron una vez con esto: escribieron
// esExito(409) = true suponiendo qué significaba nuestro 409, y sus tests se
// quedaron en verde afirmando una suposición. Aquí el contrato queda fijado del
// lado que manda, que es el que decide los códigos.
func TestHireStatus(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		código int
	}{
		{"validación", apperrors.ErrInvalidInput, http.StatusBadRequest},
		{"empresa inexistente", apperrors.ErrNotFound, http.StatusNotFound},
		{"cuenta no convertible", apperrors.ErrConflict, http.StatusConflict},
		{"correo ya registrado", apperrors.ErrEmailTaken, http.StatusConflict},
		{"empresa suspendida", apperrors.ErrCompanySuspended, http.StatusUnprocessableEntity},
	}
	for _, tc := range casos {
		got, _ := hireStatus(tc.err)
		if got != tc.código {
			t.Errorf("%s: código %d, se esperaba %d", tc.nombre, got, tc.código)
		}
	}
}

// Lo que no reconocemos es un fallo NUESTRO hasta que se demuestre lo
// contrario. Darlo por 400 haría que Obersuite descartara en firme una
// contratación que sí podía salir al reintentarla.
func TestHireStatus_LoDesconocidoEs500(t *testing.T) {
	got, msg := hireStatus(errors.New("pq: connection refused"))
	if got != http.StatusInternalServerError {
		t.Errorf("código %d, se esperaba 500", got)
	}
	// Y el detalle técnico no puede llegar al reclutador: Obersuite muestra
	// nuestro mensaje tal cual a una persona.
	if msg == "pq: connection refused" {
		t.Error("el error crudo de la base de datos no debe salir en la respuesta")
	}
	if msg == "" {
		t.Error("un 500 tiene que explicar algo, aunque sea que se reintente")
	}
}

// Los mensajes de los 4xx SÍ viajan tal cual: son accionables para quien
// contrata ("la empresa está suspendida"), y sustituirlos por un texto genérico
// le quitaría la única pista de qué hacer.
func TestHireStatus_LosMensajesDe4xxLleganAlUsuario(t *testing.T) {
	err := &testHireErr{msg: "la empresa está suspendida", kind: apperrors.ErrCompanySuspended}
	got, msg := hireStatus(err)
	if got != http.StatusUnprocessableEntity {
		t.Fatalf("código %d", got)
	}
	if msg != "la empresa está suspendida" {
		t.Errorf("el mensaje llegó como %q", msg)
	}
}

type testHireErr struct {
	msg  string
	kind error
}

func (e *testHireErr) Error() string { return e.msg }
func (e *testHireErr) Unwrap() error { return e.kind }
