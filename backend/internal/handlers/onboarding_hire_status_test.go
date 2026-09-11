package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/service"
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

// fakeHireSvc devuelve lo que se le diga: sirve para provocar el 500 sin tumbar
// nada de verdad.
type fakeHireSvc struct{ err error }

func (f *fakeHireSvc) ListCompanies(_ *time.Time) ([]service.ObersuiteCompany, error) {
	return nil, nil
}
func (f *fakeHireSvc) Hire(_ service.HireRequest) (*service.HireResult, error) { return nil, f.err }

func postHire(t *testing.T, svc service.OnboardingService, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/hire", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	NewOnboardingHandler(svc).Hire(c)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

// Un 500 tiene que poder citarse. Obersuite pidió un identificador porque, sin
// él, encontrar "esas seis líneas" en nuestro log era reconstruirlas por hora y
// empresa. El id va en el cuerpo y en la línea de log, y es distinto cada vez.
func TestHire_El500LlevaUnRequestIDQueSePuedeCitar(t *testing.T) {
	cuerpo := `{"email":"a@x.com","name":"A","company_id":36}`

	w1, out1 := postHire(t, &fakeHireSvc{err: errors.New("pq: connection refused")}, cuerpo)
	_, out2 := postHire(t, &fakeHireSvc{err: errors.New("pq: connection refused")}, cuerpo)

	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("código %d, se esperaba 500", w1.Code)
	}
	rid, _ := out1["request_id"].(string)
	if !strings.HasPrefix(rid, "hire-") || len(rid) < 10 {
		t.Fatalf("el 500 no trae un request_id citable: %v", out1)
	}
	if rid == out2["request_id"] {
		t.Fatal("dos fallos distintos no pueden compartir request_id")
	}
}

// Y los 4xx no lo llevan: ahí el mensaje ya dice qué pasa y qué hacer, y un id
// invitaría a "mandárnoslo" cuando lo que toca es corregir el dato.
func TestHire_Los4xxNoLlevanRequestID(t *testing.T) {
	_, out := postHire(t, &fakeHireSvc{err: apperrors.ErrEmailTaken}, `{"email":"a@x.com","name":"A","company_id":36}`)
	if _, tiene := out["request_id"]; tiene {
		t.Fatalf("un 409 no lleva request_id: %v", out)
	}
}
