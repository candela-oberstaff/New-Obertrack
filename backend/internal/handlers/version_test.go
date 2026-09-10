package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// El orden de las variables no es decorativo: BUILD_COMMIT es la nuestra y tiene
// que ganarle a las que publica la plataforma. Si se invirtiera, un despliegue
// manual con --build-arg quedaría tapado por lo que Coolify dejara puesto.
func TestBuildCommit_PrefiereLaVariablePropia(t *testing.T) {
	t.Setenv("BUILD_COMMIT", "aaa111")
	t.Setenv("SOURCE_COMMIT", "bbb222")

	commit, source := buildCommit()
	if commit != "aaa111" {
		t.Errorf("commit = %q, se esperaba aaa111", commit)
	}
	if source != "BUILD_COMMIT" {
		t.Errorf("origen = %q, se esperaba BUILD_COMMIT", source)
	}
}

// Sin la nuestra manda la de la plataforma. Es lo que hace que el endpoint diga
// algo útil aunque nadie se acuerde de pasar el --build-arg al construir.
func TestBuildCommit_CaeALaDeLaPlataforma(t *testing.T) {
	t.Setenv("BUILD_COMMIT", "")
	t.Setenv("SOURCE_COMMIT", "bbb222")

	commit, source := buildCommit()
	if commit != "bbb222" || source != "SOURCE_COMMIT" {
		t.Errorf("commit/origen = %q/%q, se esperaba bbb222/SOURCE_COMMIT", commit, source)
	}
}

// Una variable puesta pero vacía es lo mismo que no tenerla. La diferencia
// importa porque un despliegue puede exportar la variable sin valor, y tomarla
// por buena dejaría el commit en blanco: se leería como un fallo del endpoint en
// vez de como lo que es, que la plataforma no lo publicó.
func TestBuildCommit_IgnoraLoVacio(t *testing.T) {
	for _, name := range commitEnvVars {
		t.Setenv(name, "   ")
	}
	commit, source := buildCommit()
	// En las pruebas Go sí incrusta vcs.revision, así que el origen puede ser
	// "vcs"; lo que NO puede es venir de una variable vacía.
	if strings.TrimSpace(commit) == "" {
		t.Error("el commit no puede quedar en blanco")
	}
	for _, name := range commitEnvVars {
		if source == name {
			t.Errorf("se tomó por bueno %s, que está vacía", name)
		}
	}
}

// El relleno del Dockerfile no cuenta como commit.
//
// Esta prueba nace de un fallo real: el ARG del Dockerfile traía
// "desconocido" por defecto, así que la variable llegaba SIEMPRE definida y el
// endpoint contestaba commit_source=BUILD_COMMIT con un commit que nadie había
// puesto. Es decir, la señal que existe para detectar "el despliegue no publica
// el commit" afirmaba lo contrario. Se ve solo ejecutándolo.
func TestBuildCommit_ElRellenoNoCuentaComoCommit(t *testing.T) {
	for _, name := range commitEnvVars {
		t.Setenv(name, "")
	}
	t.Setenv("BUILD_COMMIT", commitUnknown)
	t.Setenv("SOURCE_COMMIT", "bbb222")

	commit, source := buildCommit()
	if commit != "bbb222" || source != "SOURCE_COMMIT" {
		t.Errorf("commit/origen = %q/%q, se esperaba bbb222/SOURCE_COMMIT", commit, source)
	}
}

// Y si el relleno es lo ÚNICO que hay, el origen tiene que ser "ninguno": decir
// BUILD_COMMIT mandaría a buscar el fallo al sitio equivocado.
func TestBuildCommit_SoloElRellenoEsOrigenNinguno(t *testing.T) {
	for _, name := range commitEnvVars {
		t.Setenv(name, "")
	}
	t.Setenv("BUILD_COMMIT", commitUnknown)

	commit, source := buildCommit()
	if source == "BUILD_COMMIT" {
		t.Error("el relleno no puede figurar como origen del commit")
	}
	if commit == commitUnknown && source != "ninguno" {
		t.Errorf("origen = %q, se esperaba \"ninguno\"", source)
	}
}

// No inventarse un commit es el punto entero del endpoint: uno falso es peor que
// ninguno, porque se le cree y se da por desplegado algo que no lo está.
func TestBuildCommit_SinNadaDiceDesconocido(t *testing.T) {
	for _, name := range commitEnvVars {
		t.Setenv(name, "")
	}
	commit, source := buildCommit()
	if commit == "" {
		t.Error("el commit nunca debe venir vacío: un campo vacío parece un fallo")
	}
	if commit == "desconocido" && source != "ninguno" {
		t.Errorf("si el commit es desconocido el origen debe ser \"ninguno\", fue %q", source)
	}
}

// La respuesta completa, que es el contrato con Obersuite. Se comprueba sobre el
// JSON servido y no sobre la struct: es ahí donde se rompe si alguien renombra
// una clave.
func TestVersion_LaRespuestaLlevaLoQueObersuiteNecesita(t *testing.T) {
	t.Setenv("BUILD_COMMIT", "abc123")
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/version", nil)
	(&VersionHandler{}).Version(c)

	if w.Code != http.StatusOK {
		t.Fatalf("código %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("la respuesta no es JSON válido: %v", err)
	}

	for clave, want := range map[string]any{
		"service":                "obertrack",
		"commit":                 "abc123",
		"commit_source":          "BUILD_COMMIT",
		"payload_schema_version": float64(PayloadSchemaVersion), // JSON no tiene enteros
	} {
		if body[clave] != want {
			t.Errorf("%s = %v, se esperaba %v", clave, body[clave], want)
		}
	}
	if _, ok := body["started_at"].(string); !ok {
		t.Error("falta started_at: es lo que delata un reinicio que nadie pidió")
	}
}

// El endpoint va DENTRO del grupo protegido por el token de servicio. Si el
// token se colara en la respuesta, quien ya lo tiene no gana nada, pero
// cualquier log, captura o pegado del JSON lo filtraría — y es un token estático
// que solo se cambia a mano.
func TestVersion_NoFiltraElToken(t *testing.T) {
	t.Setenv("OBERSUITE_SERVICE_TOKEN", "token-secreto-de-prueba")
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/version", nil)
	c.Request.Header.Set("X-Service-Token", "token-secreto-de-prueba")
	(&VersionHandler{}).Version(c)

	if strings.Contains(w.Body.String(), "token-secreto-de-prueba") {
		t.Error("el token de servicio salió en el cuerpo de /version")
	}
}
