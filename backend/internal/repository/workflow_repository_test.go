package repository

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// workflow_step_runs.output es jsonb, y la cadena vacía NO es JSON válido: un paso
// saltado o fallido —que por definición no deja salida— reventaba el INSERT con
// "invalid input syntax for type json". Como el llamador descarta el error de la
// bitácora para no tumbar la ejecución, esos pasos desaparecían en silencio y una
// regla que decidió no actuar quedaba indistinguible de una que no llegó a correr.
//
// La normalización se prueba sin base de datos porque lo que hay que fijar es la
// invariante —nunca sale una cadena vacía hacia una columna jsonb— y no el INSERT.
func TestSaveStepRun_NormalizaLaSalidaVaciaAJSONValido(t *testing.T) {
	casos := []struct {
		nombre string
		output string
		quiero string
	}{
		{"paso saltado, sin salida", "", "{}"},
		{"sólo espacios", "   ", "{}"},
		{"salida real intacta", `{"notificados":[7]}`, `{"notificados":[7]}`},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			entry := &models.WorkflowStepRun{Output: c.output}
			// db nil: se comprueba la normalización previa al INSERT, que ocurre
			// antes de tocar la conexión.
			r := &workflowRepository{}
			func() {
				defer func() { _ = recover() }()
				_ = r.SaveStepRun(entry)
			}()
			if entry.Output != c.quiero {
				t.Fatalf("output esperado %q, got %q", c.quiero, entry.Output)
			}
		})
	}
}
