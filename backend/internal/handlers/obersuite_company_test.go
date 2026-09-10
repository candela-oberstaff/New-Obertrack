package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
)

// rutaCategoriasFrontend es el archivo que define los filtros del expediente en
// nuestra pantalla. La prueba lo lee de verdad en vez de repetir aquí la lista:
// una copia que hay que mantener a mano es exactamente el problema que esto
// viene a detectar.
const rutaCategoriasFrontend = "../../../frontend/src/hooks/useTenantActivity.ts"

// Solo las líneas VIVAS: una entrada comentada (como quedaron "staff" y
// "management" al fusionarse dentro de Escala de tiempo) ya no es un filtro que
// la pantalla ofrezca, y no debe contar.
var reCategoria = regexp.MustCompile(`(?m)^\s*\{\s*value:\s*'([^']*)',\s*label:\s*'([^']*)'`)

// El bloque de categorías que se le manda a Obersuite tiene que decir lo mismo
// que ofrece nuestra pantalla.
//
// Esto no es celo por la simetría: es el fallo característico de un espejo. Si
// Obersuite escribe los filtros en su código, el día que aquí se renombre o se
// fusione una categoría —como pasó esta semana con "staff"— su pantalla sigue
// funcionando y enseñando un filtro que ya no devuelve nada. No falla, no avisa,
// y se descubre cuando alguien pregunta por qué una pestaña está vacía.
//
// Por eso las categorías viajan EN la respuesta. Y por eso esta prueba lee el
// archivo del frontend: si alguien toca una lista y no la otra, falla aquí y no
// en la pantalla de otro equipo dos semanas después.
func TestCategoriasDelExpediente_CoincidenConLasDelFrontend(t *testing.T) {
	ruta, err := filepath.Abs(rutaCategoriasFrontend)
	if err != nil {
		t.Fatalf("no se pudo resolver la ruta: %v", err)
	}
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Skipf("no se pudo leer %s (¿backend suelto, sin el frontend al lado?): %v", ruta, err)
	}

	quiere := reCategoria.FindAllStringSubmatch(string(crudo), -1)
	if len(quiere) == 0 {
		t.Fatalf("no se reconoció ninguna categoría en %s: ¿cambió el formato del archivo?", ruta)
	}

	// El "Todo" del backend no está en la lista del frontend (allí el chip vive
	// aparte, comentado), así que se compara a partir del segundo.
	tiene := obersuiteTimelineCategories()
	if len(tiene) == 0 || tiene[0]["value"] != "" {
		t.Fatal(`la primera categoría debe ser el "Todo" con value vacío`)
	}
	tiene = tiene[1:]

	if len(tiene) != len(quiere) {
		t.Fatalf("el backend ofrece %d categorías y la pantalla %d:\n  backend: %v\n  pantalla: %v",
			len(tiene), len(quiere), valores(tiene), valoresTS(quiere))
	}
	for i, m := range quiere {
		valor, etiqueta := m[1], m[2]
		if tiene[i]["value"] != valor {
			t.Errorf("categoría %d: el backend manda %q y la pantalla usa %q",
				i, tiene[i]["value"], valor)
		}
		if tiene[i]["label"] != etiqueta {
			t.Errorf("categoría %q: la etiqueta es %q en el backend y %q en la pantalla",
				valor, tiene[i]["label"], etiqueta)
		}
	}
}

func valores(cats []gin.H) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		v, _ := c["value"].(string)
		out = append(out, v)
	}
	return out
}

func valoresTS(m [][]string) []string {
	out := make([]string, 0, len(m))
	for _, x := range m {
		out = append(out, x[1])
	}
	return out
}

// El orden importa tanto como el contenido: es el que se pinta, y "Todo"
// tiene que ir primero o el filtro por defecto quedaría escondido al final.
func TestCategoriasDelExpediente_TodoVaPrimeroYSinRepetidos(t *testing.T) {
	vistos := map[string]bool{}
	for i, c := range obersuiteTimelineCategories() {
		v, _ := c["value"].(string)
		if i == 0 && v != "" {
			t.Errorf(`la primera es %q; el "Todo" (value vacío) va primero`, v)
		}
		if i > 0 && v == "" {
			t.Errorf("hay un segundo value vacío en la posición %d", i)
		}
		if vistos[v] {
			t.Errorf("la categoría %q está repetida", v)
		}
		vistos[v] = true
		if etiqueta, _ := c["label"].(string); etiqueta == "" {
			t.Errorf("la categoría %q no tiene etiqueta: Obersuite pintaría un filtro en blanco", v)
		}
	}
}
