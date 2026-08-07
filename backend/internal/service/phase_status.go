package service

import (
	"regexp"
	"strings"

	"github.com/obertrack/backend/internal/models"
)

// phaseStatusMaxLength es el límite de la columna tasks.status (varchar(50)).
// Debe coincidir con STATUS_MAX_LENGTH de frontend/src/components/Tasks/phaseStatus.ts.
const phaseStatusMaxLength = 50

// Clase de espacios equivalente al \s de JavaScript (el frontend deriva con
// /\s+/): tab, saltos, espacio, NBSP y los espacios Unicode Zs + FEFF. El \s de
// Go solo cubre ASCII; divergir aquí hace que backend y frontend calculen ids
// de columna distintos para el mismo nombre (p. ej. un NBSP pegado desde Word)
// y las tareas queden huérfanas.
var phaseWhitespace = regexp.MustCompile(`[\t\n\v\f\r \x{A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]+`)

// PhaseColumnID devuelve el identificador de columna de una fase: el valor que
// las tareas guardan en `status` cuando caen en esa columna. Réplica exacta de
// phaseStatusId() del frontend (phaseStatus.ts): las fases seeded traen Status
// ("por_hacer", ...); las custom lo derivan del nombre en minúsculas con
// espacios como guiones bajos, recortado al límite de la columna.
func PhaseColumnID(p models.Phase) string {
	base := p.Status
	if base == "" {
		base = phaseWhitespace.ReplaceAllString(strings.ToLower(p.Name), "_")
	}
	// Recorte por code points (runas), igual que el frontend (Array.from) y que
	// el varchar(50) de Postgres, que cuenta caracteres.
	runes := []rune(base)
	if len(runes) > phaseStatusMaxLength {
		return string(runes[:phaseStatusMaxLength])
	}
	return base
}
