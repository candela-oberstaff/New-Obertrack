package mcp

import (
	"net/url"
	"strings"
)

// allowedRedirectDomains son los dominios de los asistentes que pueden recibir
// un código de autorización. Se admite el dominio y cualquier subdominio.
var allowedRedirectDomains = []string{
	"chatgpt.com", "openai.com",
	"claude.ai", "claude.com", "anthropic.com",
	"google.com", "googleapis.com",
	"grok.com", "x.ai",
	"perplexity.ai",
	"github.com",
	"cursor.com", "cursor.sh",
}

// allowedNativeSchemes son los esquemas de apps de escritorio (claude://…).
var allowedNativeSchemes = map[string]bool{
	"claude": true, "cursor": true, "vscode": true, "vscode-insiders": true,
	"windsurf": true, "chatgpt": true,
}

// NativeSchemes en el orden en que van al form-action del CSP.
func nativeSchemesList() []string {
	return []string{"claude:", "cursor:", "vscode:", "vscode-insiders:", "windsurf:", "chatgpt:"}
}

// ParseExtraDomains convierte el texto del panel (uno por línea o separados
// por comas) en dominios limpios en minúsculas.
func ParseExtraDomains(raw string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	}) {
		d := strings.ToLower(strings.TrimSpace(part))
		d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
		d = strings.TrimSuffix(strings.TrimPrefix(d, "*."), "/")
		if d != "" && !strings.ContainsAny(d, "/:@?#") {
			out = append(out, d)
		}
	}
	return out
}

// RedirectAllowed decide si una redirect_uri se puede registrar.
//
//   - https en un dominio de la lista (o de los extra del admin);
//   - un esquema nativo de la lista;
//   - en desarrollo, también http://localhost y http://127.0.0.1 (MCP
//     Inspector, Gemini CLI local).
//
// Nunca se admite un fragmento, ni usuario/contraseña en la URL.
func RedirectAllowed(raw string, extraDomains []string, dev bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.User != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if allowedNativeSchemes[scheme] {
		return true
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if scheme == "http" {
		return dev && (host == "localhost" || host == "127.0.0.1")
	}
	if scheme != "https" {
		return false
	}
	domains := append(append([]string{}, allowedRedirectDomains...), extraDomains...)
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
