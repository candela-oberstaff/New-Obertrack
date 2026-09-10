package handlers

import (
	"net/http"
	"os"
	"runtime/debug"

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

// Version devuelve el commit desplegado y la versión del esquema del padrón.
//
// El commit sale de la información que Go incrusta al compilar desde un
// repositorio git; en una imagen construida sin el .git (como el Dockerfile de
// aquí) no está, y entonces manda la variable de entorno BUILD_COMMIT. Si no hay
// ninguna de las dos se dice "desconocido" en vez de callar: un campo ausente se
// interpreta como "no lo tienen", y uno vacío como que algo falló.
func (h *VersionHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":                "obertrack",
		"commit":                 buildCommit(),
		"payload_schema_version": PayloadSchemaVersion,
	})
}

func buildCommit() string {
	if v := os.Getenv("BUILD_COMMIT"); v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value
			}
		}
	}
	return "desconocido"
}
