package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// queryRedactedPaths son las rutas cuyo query lleva una credencial y no debe
// llegar al log de acceso. /api/auth/crm recibe el token de un solo uso del
// CRM en ?token= (contrato CRM, §4). nginx ya lo omite en su access log; esto
// cubre el log del propio backend.
var queryRedactedPaths = map[string]bool{
	"/api/auth/crm": true,
}

// RedactedLogFormatter es el formato de log por defecto de Gin, salvo que en
// las rutas de queryRedactedPaths el query se sustituye por "[redacted]". Gin
// lee el query ANTES de ejecutar los handlers, así que no se puede limpiar
// desde un middleware: tiene que ser en el formateador.
func RedactedLogFormatter(param gin.LogFormatterParams) string {
	path := param.Path
	if i := strings.IndexByte(path, '?'); i >= 0 && queryRedactedPaths[path[:i]] {
		path = path[:i] + "?[redacted]"
	}

	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}
	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}
	return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		path,
		param.ErrorMessage,
	)
}
