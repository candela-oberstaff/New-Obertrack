package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// EmailPreviewHandler sirve las plantillas de correo renderizadas en el
// navegador, con datos de ejemplo, para poder iterar el diseño sin enviar nada.
//
// Se registra SOLO fuera de release (ver routes): expone estructura de correos
// internos, así que no debe existir en producción.
type EmailPreviewHandler struct{}

func NewEmailPreviewHandler() *EmailPreviewHandler { return &EmailPreviewHandler{} }

// sampleLink es un enlace de ejemplo; no lleva a ninguna parte real.
const sampleLink = "https://obertrack.com/ejemplo?token=DEMO"

// previews es el catálogo: slug → (título legible, HTML de ejemplo).
var previews = []struct {
	Slug  string
	Title string
	Build func() string
}{
	{
		Slug:  "induction-invite",
		Title: "Invitación a la inducción",
		Build: func() string {
			return service.BuildInductionInviteHTML("Juan Pérez", sampleLink)
		},
	},
	{
		Slug:  "password-setup",
		Title: "Crear contraseña (aprobó la inducción)",
		Build: func() string {
			return service.BuildPasswordSetupHTML("Juan Pérez", "juan.perez@correo.com", sampleLink)
		},
	},
	{
		Slug:  "password-reset",
		Title: "Recuperar contraseña",
		Build: func() string {
			return service.BuildPasswordResetHTML("Juan Pérez", sampleLink)
		},
	},
}

// Index lista las plantillas disponibles con un enlace a cada una.
func (h *EmailPreviewHandler) Index(c *gin.Context) {
	html := `<!doctype html><html><head><meta charset="utf-8"><title>Vista previa de correos</title>
<style>body{font-family:system-ui,sans-serif;background:#f5f2fb;margin:0;padding:40px;color:#060b23}
h1{font-size:22px}ul{list-style:none;padding:0;max-width:520px}
li{margin:0 0 10px}a{display:block;padding:16px 20px;background:#fff;border:1px solid #ddd9ef;border-radius:12px;text-decoration:none;color:#060b23;font-weight:600}
a:hover{border-color:#cc33cc}small{display:block;color:#8880a8;font-weight:400;margin-top:4px}</style>
</head><body><h1>Vista previa de correos</h1><p>Solo disponible en desarrollo. Datos de ejemplo.</p><ul>`
	for _, p := range previews {
		html += `<li><a href="/api/dev/email-preview/` + p.Slug + `">` + p.Title +
			`<small>/api/dev/email-preview/` + p.Slug + `</small></a></li>`
	}
	html += `</ul></body></html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// Show renderiza una plantilla concreta por su slug.
func (h *EmailPreviewHandler) Show(c *gin.Context) {
	slug := c.Param("slug")
	for _, p := range previews {
		if p.Slug == slug {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(p.Build()))
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "plantilla no encontrada"})
}
