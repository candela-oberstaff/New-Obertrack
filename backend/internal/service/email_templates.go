package service

import "fmt"

// Plantillas de los correos transaccionales, separadas del envío para que se
// puedan previsualizar sin mandar nada (ver handlers/email_preview.go) y para
// que el diseño se cambie en un solo lugar.
//
// Se construyen por concatenación y no con fmt.Sprintf sobre el HTML completo:
// el CSS embebido está lleno de '%' (gradientes, anchos) y escaparlos todos es
// una fuente segura de errores.

const (
	emailBgColor     = "#f5f2fb"
	emailInkColor    = "#060b23"
	emailMutedColor  = "#8880a8"
	emailBodyColor   = "#5c5680"
	emailBorderColor = "#ddd9ef"
	emailLogoURL     = "https://obertrack.com/logos/Horizontal_Blanco.png"
)

// brandedEmailShell envuelve el contenido en la identidad de Obertrack: banner
// con logo, tarjeta blanca y pie. `heading` es el titular del banner.
func brandedEmailShell(heading, body string) string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: ` + emailBgColor + `; margin: 0; padding: 20px; color: ` + emailInkColor + `;">
	<div style="max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 20px; overflow: hidden; box-shadow: 0 10px 15px -3px rgba(6, 11, 35, 0.1), 0 4px 6px -2px rgba(6, 11, 35, 0.05); border: 1px solid ` + emailBorderColor + `;">

		<div style="background: linear-gradient(135deg, ` + emailInkColor + ` 0%, #cc33cc 100%); padding: 32px 24px; color: #ffffff; text-align: center;">
			<img src="` + emailLogoURL + `" alt="Obertrack" height="40" style="display: block; margin: 0 auto 12px auto; height: 40px; border: 0; outline: none;" />
			<h1 style="font-size: 20px; font-weight: 700; opacity: 0.95; margin: 0; color: #ffffff; font-family: sans-serif;">` + heading + `</h1>
		</div>

		<div style="padding: 32px 24px;">` + body + `</div>

		<div style="background: ` + emailBgColor + `; padding: 24px; text-align: center; font-size: 12px; color: ` + emailMutedColor + `; border-top: 1px solid ` + emailBorderColor + `; font-family: sans-serif;">
			Este es un correo automático generado de forma segura por <strong>Obertrack</strong>.<br>
			&copy; 2026 Obertrack. Todos los derechos reservados.
		</div>
	</div>
</body>
</html>`
}

// emailButton es el botón de acción principal.
func emailButton(label, href string) string {
	return `<div style="text-align: center; margin: 32px 0;">
		<a href="` + href + `" style="display: inline-block; background: linear-gradient(135deg, #cc33cc 0%, #8a2be2 100%); color: #ffffff; text-decoration: none; padding: 14px 40px; border-radius: 12px; font-size: 15px; font-weight: 700; font-family: sans-serif; box-shadow: 0 4px 16px rgba(204, 51, 204, 0.35);">` + label + `</a>
	</div>`
}

// emailGreeting es el "Hola <nombre>,".
func emailGreeting(name string) string {
	return `<p style="font-size: 16px; line-height: 1.6; margin-bottom: 24px; color: ` + emailInkColor + `; font-family: sans-serif;">Hola <strong>` + name + `</strong>,</p>`
}

// emailParagraph es un párrafo del cuerpo.
func emailParagraph(text string) string {
	return `<p style="font-size: 15px; line-height: 1.6; margin-bottom: 24px; color: ` + emailBodyColor + `; font-family: sans-serif;">` + text + `</p>`
}

// emailNote es una nota secundaria (letra chica).
func emailNote(text string) string {
	return `<p style="font-size: 13px; line-height: 1.6; color: ` + emailMutedColor + `; font-family: sans-serif;">` + text + `</p>`
}

// emailFallbackLink muestra el enlace en texto por si el botón no funciona.
func emailFallbackLink(href string) string {
	return `<div style="background: ` + emailBgColor + `; border: 1px solid ` + emailBorderColor + `; border-radius: 12px; padding: 16px; margin-top: 24px;">
		<p style="font-size: 12px; color: ` + emailMutedColor + `; margin: 0 0 8px 0; font-family: sans-serif;">Si el botón no funciona, copia y pega este enlace en tu navegador:</p>
		<p style="font-size: 12px; color: #8a2be2; word-break: break-all; margin: 0; font-family: sans-serif;">` + href + `</p>
	</div>`
}

// --- Plantillas ---

// BuildPasswordResetHTML: recuperación de contraseña de una cuenta existente.
func BuildPasswordResetHTML(name, resetLink string) string {
	body := emailGreeting(name) +
		emailParagraph("Recibimos una solicitud para restablecer la contraseña de tu cuenta. Haz clic en el botón de abajo para crear una nueva contraseña.") +
		emailButton("Restablecer Contraseña", resetLink) +
		emailNote("Si no solicitaste este cambio, puedes ignorar este correo. Tu contraseña actual seguirá siendo la misma.") +
		emailNote("Este enlace expirará en <strong>1 hora</strong>.") +
		emailFallbackLink(resetLink)
	return brandedEmailShell("Recuperar Contraseña", body)
}

// BuildPasswordSetupHTML: alta de contraseña de un profesional que APROBÓ su
// inducción. Nunca tuvo una, así que no se habla de "restablecer".
func BuildPasswordSetupHTML(name, userEmail, setupLink string) string {
	body := emailGreeting(name) +
		emailParagraph("Completaste tu inducción y tu acceso a Obertrack ya está habilitado. Solo falta un paso: crear tu contraseña para entrar por primera vez.") +
		emailButton("Crear mi contraseña", setupLink) +
		emailNote("Tu usuario es <strong>"+userEmail+"</strong>. Este enlace es personal y expira en <strong>24 horas</strong>; si se vence, puedes pedir uno nuevo desde \"¿Olvidaste tu contraseña?\" en la pantalla de inicio de sesión.") +
		emailFallbackLink(setupLink)
	return brandedEmailShell("¡Aprobaste tu inducción!", body)
}

// BuildInductionInviteHTML: invitación a la landing pública de inducción. Es lo
// primero que recibe alguien contratado desde Obersuite.
func BuildInductionInviteHTML(name, landingLink string) string {
	body := emailGreeting(name) +
		emailParagraph("¡Bienvenido a Obertrack! Antes de darte acceso a la plataforma necesitamos que completes una breve inducción: un video de presentación y unas preguntas.") +
		emailParagraph("Cuando la apruebes, te enviaremos un correo para que crees tu contraseña y puedas entrar.") +
		emailButton("Comenzar mi inducción", landingLink) +
		emailNote("Este enlace es personal, no lo compartas.") +
		emailFallbackLink(landingLink)
	return brandedEmailShell("Completa tu inducción", body)
}

// BuildSupportAlertHTML: alerta interna al equipo de Soporte.
func BuildSupportAlertHTML(rows, link string) string {
	body := `<p style="margin:0 0 16px 0;font-family:sans-serif;">Se ha creado un nuevo ticket de soporte. Estos son los detalles:</p>
	<table style="width:100%;border-collapse:collapse;font-size:14px;font-family:sans-serif;">` + rows + `</table>` +
		emailButton("Abrir en Obertrack", link)
	return brandedEmailShell("🎫 Nuevo ticket de soporte", body)
}

// emailTableRow arma una fila etiqueta/valor para BuildSupportAlertHTML.
func emailTableRow(label, value string) string {
	return fmt.Sprintf(
		`<tr><td style="padding:6px 12px 6px 0;color:%s;font-weight:600;white-space:nowrap;vertical-align:top;">%s</td><td style="padding:6px 0;color:%s;">%s</td></tr>`,
		emailMutedColor, label, emailInkColor, value)
}
