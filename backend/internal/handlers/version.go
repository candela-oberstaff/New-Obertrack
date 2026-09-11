package handlers

import (
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PayloadSchemaVersion identifica la FORMA del padrón de empresas que sirve
// /integrations/obersuite/companies.
//
// Sube cuando se añade, quita o cambia de significado un campo. Es lo que le
// permite a Obersuite saber qué contrato está recibiendo sin deducirlo de las
// claves que le llegan — que es exactamente lo que tuvieron que hacer cuando no
// podían distinguir si un cambio nuestro estaba desplegado.
//
//	1 → ficha inicial (id, name, status, responsable, ubicación, contadores,
//	    last_contact como texto).
//	2 → se añaden last_contact_at (ISO), updated_at y ETag.
//	3 → se retiran los campos que nadie consumía: los de operación,
//	    last_activity_at, location, created_at, client_since, phone_number y
//	    open_tickets.
const PayloadSchemaVersion = 3

// VersionHandler expone QUÉ está desplegado.
//
// Existe porque el fallo más caro de esta integración no fue de código: fue que
// ninguno de los dos lados podía saber si un cambio había llegado a producción.
// Se infería mirando el JSON o contando bytes, y se llegó a dar por desplegado
// algo que seguía sin subir. Un endpoint que lo diga elimina esa clase entera de
// malentendido.
type VersionHandler struct{}

func NewVersionHandler() *VersionHandler { return &VersionHandler{} }

// processStart marca el arranque del proceso. Sirve para notar un reinicio que
// nadie pidió: si el número sube solo, algo se está cayendo y volviendo a
// levantar, y eso explica síntomas (sesiones perdidas, trabajos a medias) que de
// otro modo se persiguen por el lado equivocado.
var processStart = time.Now().UTC()

// Version devuelve el commit desplegado y la versión del esquema del padrón.
func (h *VersionHandler) Version(c *gin.Context) {
	commit, source := buildCommit()
	c.JSON(http.StatusOK, gin.H{
		"service":                "obertrack",
		"commit":                 commit,
		"commit_source":          source,
		"payload_schema_version": PayloadSchemaVersion,
		"started_at":             processStart.Format(time.RFC3339),
	})
}

// commitUnknown es lo que se contesta cuando el commit no aparece por ningún
// lado. Se dice en vez de callar: un campo ausente se lee como "no lo tienen" y
// uno vacío como que algo falló.
const commitUnknown = "desconocido"

// commitEnvVars son las variables donde puede venir el commit, en orden de
// preferencia. BUILD_COMMIT es la nuestra (la pasa el Dockerfile); las demás las
// publica sola la plataforma de despliegue, y probarlas evita depender de que
// alguien se acuerde de pasar el --build-arg.
var commitEnvVars = []string{
	"BUILD_COMMIT",  // nuestra, vía --build-arg
	"SOURCE_COMMIT", // Coolify
	"COMMIT_SHA",
	"GIT_COMMIT",
	"GITHUB_SHA", // GitHub Actions
}

// buildCommit devuelve el commit desplegado y DE DÓNDE salió.
//
// Lo segundo importa tanto como lo primero: un "desconocido" y un commit real se
// leen igual de bien, así que sin el origen no se puede distinguir "no hay
// commit" de "el despliegue no publica la variable" — que es exactamente el
// caso que nos tocó, y que costó una ronda entera de mensajes averiguar.
//
// Si no aparece por ningún lado se dice "desconocido" en vez de inventarse algo:
// un commit falso es peor que ninguno, porque se le cree.
func buildCommit() (commit, source string) {
	for _, name := range commitEnvVars {
		v := strings.TrimSpace(os.Getenv(name))
		// "desconocido" se descarta como si no estuviera: las imágenes
		// construidas antes de arreglar el Dockerfile llevan ese literal cocido
		// dentro, y darlo por bueno haría que el endpoint jurara que el commit
		// viene de BUILD_COMMIT cuando lo que hay ahí es un relleno.
		if v == "" || v == commitUnknown {
			continue
		}
		return v, name
	}
	// En desarrollo (go run desde el repo) Go incrusta el commit al compilar. En
	// la imagen no está, porque se construye sin el .git.
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, st := range info.Settings {
			if st.Key == "vcs.revision" && st.Value != "" {
				return st.Value, "vcs"
			}
		}
	}
	return commitUnknown, "ninguno"
}
