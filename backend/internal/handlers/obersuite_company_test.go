package handlers

import (
	"net/http"
	"net/http/httptest"
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

// pseudoFiltrosDePantalla son chips que la pantalla muestra junto a las
// categorías pero que NO filtran la cronología: cambian de panel. No pueden
// viajar a Obersuite como categoría, porque ?category=<eso> devolvería el
// expediente entero (categoría desconocida = sin filtro) y parecería que
// funciona. El valor es el motivo, para que quede escrito.
var pseudoFiltrosDePantalla = map[string]string{
	"surveys":     "cambia el panel al informe de encuestas; no es una categoría del expediente",
	"testimonial": "cambia el panel a los testimonios de los profesionales de la empresa",
}

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
	// Los pseudo-filtros de la pantalla se descartan de forma EXPLÍCITA. Uno
	// nuevo que no esté aquí hace fallar la prueba, y eso es lo que se quiere:
	// obliga a decidir si es una categoría (y entonces va al backend) o un
	// cambio de panel (y entonces se declara aquí, con el motivo).
	filtrado := quiere[:0]
	for _, m := range quiere {
		if motivo, esPseudo := pseudoFiltrosDePantalla[m[1]]; esPseudo {
			t.Logf("se descarta %q: %s", m[1], motivo)
			continue
		}
		filtrado = append(filtrado, m)
	}
	quiere = filtrado

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

// La foto tiene que llegar como URL absoluta y por la ruta pública: la relativa
// se la pedía el navegador de Obersuite a SU dominio y salía rota, y
// /api/uploads exige sesión, así que tampoco se podía proxear con el token.
func TestPublicAvatarURL(t *testing.T) {
	t.Setenv("SERVICE_URL_BACKEND", "https://obertrack.com/")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	casos := map[string]string{
		"/api/uploads/18_abc.jpg": "https://obertrack.com/api/public/uploads/18_abc.jpg",
		"18_abc.jpg":              "https://obertrack.com/api/public/uploads/18_abc.jpg",
		"https://cdn.x.com/y.png": "https://cdn.x.com/y.png",
		"":                        "",
		"   ":                     "",
	}
	for in, want := range casos {
		if got := publicAvatarURL(c, in); got != want {
			t.Errorf("publicAvatarURL(%q) = %q, se esperaba %q", in, got, want)
		}
	}
}

// La base de las fotos la decide el dominio público, no Coolify.
//
// SERVICE_URL_BACKEND la inyecta Coolify sola con el host interno del contenedor
// (http://…nip.io). Con esa base, avatar_url salía con un host http que desde la
// página https de Obersuite el navegador bloquea como contenido mixto —y las
// imágenes de los correos igual, solo que ahí nadie lo veía—. BACKEND_URL, puesta
// a mano, tiene que ganar; y si no está, se sigue cayendo a la de Coolify para no
// romper los entornos que nunca la definieron.
func TestPublicAvatarURL_ElDominioPublicoGanaAlInternoDeCoolify(t *testing.T) {
	t.Setenv("SERVICE_URL_BACKEND", "http://backend-abc.194.163.170.245.nip.io")
	t.Setenv("BACKEND_URL", "https://obertrack.com/")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	got := publicAvatarURL(c, "/api/uploads/18_abc.jpg")
	if got != "https://obertrack.com/api/public/uploads/18_abc.jpg" {
		t.Fatalf("la foto sale con el host interno: %q", got)
	}
}

func TestPublicAvatarURL_SinDominioPublicoCaeAlDeCoolify(t *testing.T) {
	t.Setenv("SERVICE_URL_BACKEND", "http://backend-abc.194.163.170.245.nip.io")
	t.Setenv("BACKEND_URL", "")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	got := publicAvatarURL(c, "/api/uploads/18_abc.jpg")
	if got != "http://backend-abc.194.163.170.245.nip.io/api/public/uploads/18_abc.jpg" {
		t.Fatalf("sin BACKEND_URL debe seguir funcionando como antes: %q", got)
	}
}
