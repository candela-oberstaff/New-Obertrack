package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Guardar una encuesta contestaba siempre "Failed to update survey", en inglés y sin
// decir nada. Detrás había dos situaciones muy distintas —una decisión de quien edita
// y una avería— y las dos se veían igual, así que no había forma de saber qué hacer.

type repoDeGuardado struct {
	repository.SurveyRepository
	err error
}

func (r *repoDeGuardado) UpdateSurvey(_ *models.Survey) error { return r.err }

func guardar(t *testing.T, err error) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/surveys/5",
		strings.NewReader(`{"id":5,"title":"Encuesta","questions":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "5"}}

	h := &SurveyHandler{repo: &repoDeGuardado{err: err}}
	h.UpdateSurvey(c)

	var cuerpo map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &cuerpo)
	motivo, _ := cuerpo["error"].(string)
	return rec.Code, motivo
}

// Quitar una pregunta ya contestada no es una avería: es una decisión con
// consecuencias, y el mensaje tiene que nombrar la pregunta y ofrecer la salida.
func TestGuardarEncuesta_LaPreguntaContestadaSeExplicaEnEspañol(t *testing.T) {
	code, motivo := guardar(t, repository.ErrPreguntaRespondida{
		Preguntas: 1, Respuestas: 3, Enunciado: "¿Qué tal el acompañamiento?",
	})

	if code != http.StatusConflict {
		t.Fatalf("un choque de datos no es un error del servidor: got %d", code)
	}
	if !strings.Contains(motivo, "¿Qué tal el acompañamiento?") {
		t.Fatalf("el mensaje debe nombrar la pregunta: %q", motivo)
	}
	if !strings.Contains(motivo, "3") {
		t.Fatalf("y cuántas respuestas se perderían: %q", motivo)
	}
}

// Cualquier otro fallo se cuenta tal cual. El mensaje genérico dejaba al usuario sin
// nada que mirar y al programador sin nada que buscar.
func TestGuardarEncuesta_ElFalloDeVerdadSeCuentaEnVezDeEsconderse(t *testing.T) {
	code, motivo := guardar(t, errors.New(`column "send_by_inapp" does not exist`))

	if code != http.StatusInternalServerError {
		t.Fatalf("got %d", code)
	}
	if strings.Contains(motivo, "Failed to update survey") {
		t.Fatalf("seguía escondiendo el motivo: %q", motivo)
	}
	if !strings.Contains(motivo, "send_by_inapp") {
		t.Fatalf("el motivo real tiene que llegar a pantalla: %q", motivo)
	}
}

func TestGuardarEncuesta_CuandoSaleBienDevuelveLaEncuesta(t *testing.T) {
	code, motivo := guardar(t, nil)

	if code != http.StatusOK {
		t.Fatalf("got %d (%s)", code, motivo)
	}
}

// El texto del aviso se lee en pantalla, así que se fija aquí: en español, diciendo
// qué pasó y qué se puede hacer en su lugar.
func TestPreguntaRespondida_ElTextoDiceQueHacer(t *testing.T) {
	una := repository.ErrPreguntaRespondida{Preguntas: 1, Respuestas: 2, Enunciado: "¿Recomendarías Oberstaff?"}
	if !strings.Contains(una.Error(), "¿Recomendarías Oberstaff?") {
		t.Fatalf("falta la pregunta: %q", una.Error())
	}
	if !strings.Contains(una.Error(), "crea una nueva") {
		t.Fatalf("falta la salida que puede tomar: %q", una.Error())
	}

	varias := repository.ErrPreguntaRespondida{Preguntas: 3, Respuestas: 9}
	if !strings.Contains(varias.Error(), "3 preguntas") {
		t.Fatalf("con varias hay que decir cuántas: %q", varias.Error())
	}
}
