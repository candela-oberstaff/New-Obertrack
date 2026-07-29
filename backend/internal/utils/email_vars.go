package utils

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

// Variables de personalización de correos ("merge tags").
//
// La sustitución ocurre SIEMPRE sobre el HTML ya renderizado, nunca sobre los
// bloques ni sobre el código fuente. Gracias a eso el mismo motor sirve para
// las plantillas armadas en el editor visual y para las escritas a mano en el
// editor de código: cuando llega aquí, ambas ya son un string de HTML.
//
// Sintaxis: {{clave}} o {{clave|texto por defecto}}
//
// El catálogo de abajo es la ÚNICA fuente de verdad. El backend lo usa para
// resolver los valores al enviar y el frontend lo consume vía
// GET /api/email/variables para pintar el panel de inserción y el preview.

// EmailVariable describe una variable disponible para los redactores.
type EmailVariable struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// Example es el valor que se muestra en el preview del editor.
	Example string `json:"example"`
	// Fallback es lo que se imprime cuando el destinatario no tiene el dato y
	// la plantilla no declaró un valor por defecto con la barra vertical.
	Fallback string `json:"fallback"`
	Group    string `json:"group"`
}

var emailVariableCatalog = []EmailVariable{
	// Persona
	{Key: "nombre", Label: "Nombre completo", Description: "Nombre completo del destinatario", Example: "María González", Fallback: "colega", Group: "Persona"},
	{Key: "primer_nombre", Label: "Primer nombre", Description: "Solo la primera palabra del nombre — ideal para el saludo", Example: "María", Fallback: "colega", Group: "Persona"},
	{Key: "email", Label: "Correo", Description: "Dirección de correo del destinatario", Example: "maria@empresa.com", Fallback: "", Group: "Persona"},
	{Key: "telefono", Label: "Teléfono", Description: "Teléfono registrado en su perfil", Example: "+58 412 1234567", Fallback: "", Group: "Persona"},
	{Key: "cargo", Label: "Cargo", Description: "Puesto o título profesional", Example: "Diseñadora UX", Fallback: "", Group: "Persona"},

	// Empresa
	{Key: "empresa", Label: "Empresa", Description: "Empresa asociada al destinatario", Example: "Acme Corp", Fallback: "tu empresa", Group: "Empresa"},
	{Key: "industria", Label: "Industria", Description: "Sector de la empresa", Example: "Tecnología", Fallback: "", Group: "Empresa"},

	// Ubicación
	{Key: "pais", Label: "País", Description: "País del destinatario", Example: "Venezuela", Fallback: "", Group: "Ubicación"},
	{Key: "estado", Label: "Estado / Provincia", Description: "Estado o provincia del destinatario", Example: "Miranda", Fallback: "", Group: "Ubicación"},
	{Key: "ciudad", Label: "Ciudad", Description: "Ciudad del destinatario", Example: "Caracas", Fallback: "", Group: "Ubicación"},

	// Sistema
	{Key: "fecha", Label: "Fecha de envío", Description: "Fecha en que se despacha el correo", Example: "27 de julio de 2026", Fallback: "", Group: "Sistema"},
	{Key: "anio", Label: "Año", Description: "Año en que se despacha el correo", Example: "2026", Fallback: "", Group: "Sistema"},
}

// emailVarRegex captura {{clave}} y {{clave|valor por defecto}}. Tolera
// espacios alrededor de la clave porque los redactores los escriben.
var emailVarRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*(\|([^{}]*))?\}\}`)

// EmailVariables devuelve el catálogo completo (copia, para que nadie lo mute).
func EmailVariables() []EmailVariable {
	out := make([]EmailVariable, len(emailVariableCatalog))
	copy(out, emailVariableCatalog)
	return out
}

// HasEmailVariables indica si el texto contiene al menos un token. Permite
// saltarse la sustitución por destinatario cuando la plantilla no personaliza
// nada, que es el caso más común.
func HasEmailVariables(s string) bool {
	return strings.Contains(s, "{{") && emailVarRegex.MatchString(s)
}

// RenderVariablesHTML sustituye los tokens en un cuerpo HTML. Los valores se
// escapan porque provienen de datos de usuario y podrían romper el marcado.
func RenderVariablesHTML(body string, data map[string]string) string {
	return renderVariables(body, data, true)
}

// RenderVariablesText sustituye los tokens en texto plano (el asunto del
// correo). No escapa: un "&" en el asunto debe llegar como "&".
func RenderVariablesText(text string, data map[string]string) string {
	return renderVariables(text, data, false)
}

func renderVariables(text string, data map[string]string, escape bool) string {
	if text == "" {
		return text
	}
	return emailVarRegex.ReplaceAllStringFunc(text, func(match string) string {
		groups := emailVarRegex.FindStringSubmatch(match)
		key := strings.ToLower(groups[1])
		hasInlineDefault := groups[2] != ""

		value := strings.TrimSpace(data[key])
		if value == "" {
			if hasInlineDefault {
				value = strings.TrimSpace(groups[3])
			} else {
				value = catalogFallback(key)
			}
		}
		if escape {
			return html.EscapeString(value)
		}
		return value
	})
}

func catalogFallback(key string) string {
	for _, v := range emailVariableCatalog {
		if v.Key == key {
			return v.Fallback
		}
	}
	// Clave desconocida: se imprime vacío en vez de dejar el token crudo a la
	// vista del destinatario.
	return ""
}

// EmailRecipient es un destinatario ya resuelto, con los campos que alimentan
// las variables. Los nombres de columna coinciden con la tabla users para que
// GORM pueda escanear directamente sobre este struct.
type EmailRecipient struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	JobTitle    string `json:"job_title"`
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
	PhoneNumber string `json:"phone_number"`
	Country     string `json:"country"`
	State       string `json:"state"`
	City        string `json:"city"`
}

// VariableData arma el diccionario de sustitución para este destinatario.
func (r EmailRecipient) VariableData() map[string]string {
	now := time.Now()
	return map[string]string{
		"nombre":        r.Name,
		"primer_nombre": firstWord(r.Name),
		"email":         r.Email,
		"telefono":      r.PhoneNumber,
		"cargo":         r.JobTitle,
		"empresa":       r.CompanyName,
		"industria":     r.Industry,
		"pais":          r.Country,
		"estado":        r.State,
		"ciudad":        r.City,
		"fecha":         formatSpanishDate(now),
		"anio":          fmt.Sprintf("%d", now.Year()),
	}
}

// ExampleVariableData devuelve los valores de ejemplo del catálogo. Se usa
// para previsualizar sin destinatario real.
func ExampleVariableData() map[string]string {
	data := make(map[string]string, len(emailVariableCatalog))
	for _, v := range emailVariableCatalog {
		data[v.Key] = v.Example
	}
	return data
}

func firstWord(s string) string {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

var spanishMonths = [...]string{
	"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
}

func formatSpanishDate(t time.Time) string {
	return fmt.Sprintf("%d de %s de %d", t.Day(), spanishMonths[int(t.Month())-1], t.Year())
}
