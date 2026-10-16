package mcp

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Página de consentimiento: HTML del servidor, sin React y sin JavaScript.
// Los botones son un <form method="post"> con campos ocultos.

type consentData struct {
	ClientName   string
	RedirectHost string
	UserName     string
	UserEmail    string
	Role         string
	ScopeText    string
	Params       authorizeParams
	CSRF         string
	Dev          bool
	Nonce        string
}

// securityHeaders van en la página de consentimiento y en la de error.
// form-action admite https: y los esquemas nativos porque, tras el POST, la
// redirección al asistente cuenta como navegación del formulario.
func securityHeaders(c *gin.Context, nonce string, dev bool) {
	formAction := "'self' https: " + strings.Join(nativeSchemesList(), " ")
	if dev {
		formAction += " http://localhost:* http://127.0.0.1:*"
	}
	c.Header("Content-Security-Policy",
		"default-src 'none'; style-src 'nonce-"+nonce+"'; script-src 'nonce-"+nonce+"'; img-src 'self' data:; "+
			"form-action "+formAction+"; frame-ancestors 'none'; base-uri 'none'")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
}

const pageStyle = `
:root{--bg:#f8fafc;--card:#fff;--text:#0f172a;--muted:#64748b;--line:#e2e8f0;--primary:#a21caf;--primary-ink:#fff;--soft:#fdf4ff}
@media (prefers-color-scheme:dark){:root{--bg:#0b1120;--card:#111827;--text:#e5e7eb;--muted:#94a3b8;--line:#1f2937;--soft:#2a1033}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:16px;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.card{width:100%;max-width:460px;background:var(--card);border:1px solid var(--line);border-radius:16px;padding:28px;box-shadow:0 10px 30px rgba(2,6,23,.08)}
.brand{font-weight:800;letter-spacing:-.01em;font-size:14px;color:var(--primary);margin:0 0 14px}
h1{font-size:20px;margin:0 0 6px;overflow-wrap:anywhere}
p{margin:0 0 12px;color:var(--muted)}
.box{background:var(--soft);border-radius:12px;padding:14px 16px;margin:16px 0}
.box dt{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.04em;color:var(--muted)}
.box dd{margin:2px 0 10px;overflow-wrap:anywhere}
.box dd:last-child{margin-bottom:0}
.ro{display:inline-block;font-size:12px;font-weight:700;padding:2px 8px;border-radius:999px;background:#dcfce7;color:#166534}
.actions{display:flex;gap:10px;margin-top:20px}
button{flex:1;font:inherit;font-weight:700;padding:11px 14px;border-radius:10px;cursor:pointer;border:1px solid var(--line);background:var(--card);color:var(--text)}
button.primary{background:var(--primary);border-color:var(--primary);color:var(--primary-ink)}
.fine{font-size:12.5px;margin-top:16px}
`

var consentTpl = template.Must(template.New("consent").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Autorizar asistente · Obertrack</title><style nonce="{{.Nonce}}">` + pageStyle + `</style></head>
<body><main class="card">
<p class="brand">Obertrack</p>
<h1>{{.ClientName}} quiere acceder a tus datos</h1>
<p>Al autorizar, este asistente de IA podrá consultar Obertrack en tu nombre. <span class="ro">Solo lectura</span></p>
<dl class="box">
<dt>Cuenta</dt><dd>{{.UserName}} · {{.UserEmail}}</dd>
<dt>Tu rol</dt><dd>{{.Role}}</dd>
<dt>Qué podrá ver</dt><dd>{{.ScopeText}}</dd>
<dt>Qué no podrá hacer</dt><dd>Crear, editar, aprobar ni borrar nada.</dd>
<dt>Volverá a</dt><dd>{{.RedirectHost}}</dd>
</dl>
<form method="post" action="/api/mcp/oauth/authorize">
<input type="hidden" name="client_id" value="{{.Params.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.Params.RedirectURI}}">
<input type="hidden" name="response_type" value="{{.Params.ResponseType}}">
<input type="hidden" name="state" value="{{.Params.State}}">
<input type="hidden" name="code_challenge" value="{{.Params.Challenge}}">
<input type="hidden" name="code_challenge_method" value="{{.Params.Method}}">
<input type="hidden" name="scope" value="{{.Params.Scope}}">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<div class="actions">
<button type="submit" name="decision" value="deny">Cancelar</button>
<button type="submit" name="decision" value="allow" class="primary">Autorizar</button>
</div>
</form>
<p class="fine">Puedes revocar este acceso cuando quieras desde Obertrack → Integraciones → Mis conexiones.</p>
</main></body></html>`))

var errorTpl = template.Must(template.New("error").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>No se pudo autorizar · Obertrack</title><style nonce="{{.Nonce}}">` + pageStyle + `</style></head>
<body><main class="card"><p class="brand">Obertrack</p><h1>No se pudo autorizar</h1><p>{{.Msg}}</p></main></body></html>`))

func renderConsent(c *gin.Context, d consentData) {
	d.Nonce = randomString(16)
	var buf bytes.Buffer
	if err := consentTpl.Execute(&buf, d); err != nil {
		log.Printf("[MCP] plantilla de consentimiento: %v", err)
		renderError(c, http.StatusInternalServerError, "No se pudo mostrar la autorización.")
		return
	}
	securityHeaders(c, d.Nonce, d.Dev)
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func renderError(c *gin.Context, status int, msg string) {
	nonce := randomString(16)
	var buf bytes.Buffer
	_ = errorTpl.Execute(&buf, map[string]string{"Nonce": nonce, "Msg": msg})
	securityHeaders(c, nonce, false)
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
	c.Abort()
}
