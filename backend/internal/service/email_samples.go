package service

import "fmt"

// Muestras para el botón "Enviar prueba" del panel de Correos. Donde existe un
// constructor real (chat, soporte, inducción) se reutiliza, para que la prueba
// enseñe EXACTAMENTE el correo que recibe la gente. Los demás se aproximan con
// el mismo contenido y datos de ejemplo. El encabezado y pie de marca los pone
// BrevoService.SendEmail, así que aquí solo va el cuerpo.
func sampleEmail(kind, toName string) (subject, body string) {
	switch kind {

	case EmailKindSupportTicket:
		n := &SupportNotifier{}
		rawHTML := n.buildHTML(SupportTicketInfo{
			Type:        "Solicitud de soporte",
			Requester:   "Laura Méndez",
			Company:     "Acme S.A.",
			Subject:     "No puedo adjuntar archivos a una tarea",
			Description: "Al intentar subir un PDF en la tarea semanal me sale un error.",
			Link:        "/tickets/soporte",
		})
		return "🎫 Nuevo ticket de soporte", brandedEmailShell("🎫 Nuevo ticket de soporte", rawHTML)

	case EmailKindInductionInvite:
		return "Invitación a tu inducción en Obertrack",
			BuildInductionInviteHTML(toName, frontendLink("/induccion"))

	case EmailKindTestimonialRequest:
		return "Nos gustaría conocer tu experiencia",
			BuildTestimonialRequestHTML(toName,
				"Nos encantaría conocer tu experiencia trabajando con nosotros. Te tomará menos de cinco minutos y nos ayuda muchísimo.",
				frontendLink("/testimonio"))

	case EmailKindWorkHourReport:
		return "Obertrack - Reporte de Jornadas (Julio 2026)", sampleBody(
			"Reporte de Jornadas — Julio 2026",
			"Aquí tienes el resumen de actividades del período. El detalle completo va en los archivos adjuntos (PDF y Excel).",
			[][2]string{{"Horas totales", "168.0 h"}, {"Aprobadas", "168.0 h"}, {"Ausencias", "2 (16.0 h)"}},
			"Abrir en Obertrack", "/reports",
			"En el correo real se adjuntan el PDF y el Excel del período.")

	case EmailKindInactivityAlert:
		return "⚠️ Profesionales sin registrar horas", sampleBody(
			"Profesionales sin actividad reciente",
			"Estas personas llevan 2 días o más sin registrar sus horas:",
			[][2]string{{"Laura Méndez (Acme S.A.)", "3 días"}, {"Diego Ramírez (Globex Corp)", "2 días"}},
			"Revisar actividad", "/admin",
			"")

	case EmailKindStaleCompany:
		return "⚠️ Empresas sin actividad en Obertrack", sampleBody(
			"Empresas sin abrir la aplicación",
			"Estas empresas llevan 14 días o más sin que ningún usuario inicie sesión:",
			[][2]string{{"Acme S.A.", "14 días sin actividad"}, {"Globex Corp", "16 días sin actividad"}},
			"Revisar empresas", "/admin",
			"Aviso automático al equipo de Customer Success.")

	case EmailKindWorkflow:
		return "⚡ Notificación de automatización", sampleBody(
			"Acción de automatización activada",
			"Se ha ejecutado la regla de automatización «Aviso a responsables al cambiar estado»:",
			[][2]string{{"Tablero", "Desarrollo & Proyectos"}, {"Tarjeta / Tarea", "Revisión de requerimientos"}, {"Disparador", "Movido a «En Pruebas»"}},
			"Ver tarjeta", "/tasks",
			"Este correo fue generado por una regla del tablero.")

	case EmailKindPasswordReset:
		return "Obertrack - Recuperar Contraseña", BuildPasswordResetHTML(toName, frontendLink("/reset-password?token=ejemplo"))

	case EmailKindAccountSetup:
		return "Obertrack - Crea tu contraseña", BuildPasswordSetupHTML(toName, toName+"@empresa.com", frontendLink("/set-password?token=ejemplo"))

	case EmailKindAccessCredentials:
		return "Obertrack - Tus datos de acceso", BuildCredentialsHTML(toName, toName+"@empresa.com", "Ejemplo-1234", frontendLink("/login"))

	case EmailKindIncidentBroadcast:
		return "Comunicado: sismo en la región", sampleBody(
			"Comunicado de incidente",
			"Nos comunicamos por el sismo reportado en tu zona. Por favor confirma que te encuentras bien respondiendo este correo o desde la plataforma.",
			nil, "Confirmar que estoy bien", "/",
			"Mensaje enviado por el equipo de Obertrack.")

	case EmailKindSurveyInvite:
		return "Nueva Encuesta: Clima laboral", BuildSurveyInviteHTML(toName, "Clima laboral", "Te invitamos a responder la encuesta «Clima laboral». Toma menos de 5 minutos.", frontendLink("/encuestas"))

	case EmailKindTicketReply:
		return "Respuesta a tu solicitud", sampleBody(
			"Respuesta del equipo de soporte",
			"Hola: revisamos tu caso y ya está resuelto. Si necesitas algo más, responde a este correo y seguimos por aquí.",
			nil, "", "",
			"")

	case EmailKindManualComposer:
		return "Seguimiento de actividad en Obertrack", sampleBody(
			"Mensaje del equipo",
			"Este es el formato de los correos que el equipo envía a mano desde las fichas, el Mapa o Incidentes. El contenido lo escribe quien envía (o sale de una plantilla).",
			nil, "Abrir Obertrack", "/",
			"")

	case EmailKindCampaign:
		return "Novedades de Obertrack", sampleBody(
			"Campaña de ejemplo",
			"Así se ve una campaña del constructor de correos. El contenido real lo define la plantilla elegida en Email Marketing.",
			nil, "Ver más", "/",
			"")

	case EmailKindObervoiceCredentials:
		return "📞 Tus datos de Obervoice", BuildObervoiceCredentialsHTML(
			toName,
			toName+"@empresa.com",
			"1001",
			"9",
			"+34 900 123 456",
			"", // la contraseña real nunca se usa en muestras
			"", // qr
			"https://voice.oberstaff.com/webrtc/",
		)
	}

	return "Correo de Obertrack", sampleBody("Muestra", "Correo de ejemplo del sistema.", nil, "", "", "")
}

// frontendLink arma un enlace absoluto al frontend cuando hay base configurada.
func frontendLink(path string) string {
	if base := frontendBaseURL(); base != "" {
		return base + path
	}
	return path
}

// sampleBody arma un cuerpo con el estilo de los correos de Obertrack: título,
// texto, una tabla opcional de datos, botón opcional y una nota al pie.
func sampleBody(title, intro string, rows [][2]string, ctaLabel, ctaPath, footnote string) string {
	html := fmt.Sprintf(`<p style="margin:0 0 16px 0;line-height:1.6;font-size:15px;color:#060b23;font-family:sans-serif;">%s</p>`, intro)

	if len(rows) > 0 {
		html += `<table style="width:100%%;border-collapse:collapse;font-size:14px;margin-bottom:16px;font-family:sans-serif;">`
		for _, r := range rows {
			html += fmt.Sprintf(`<tr><td style="padding:6px 12px 6px 0;color:#8880a8;font-weight:600;white-space:nowrap;vertical-align:top;">%s</td><td style="padding:6px 0;color:#060b23;">%s</td></tr>`, r[0], r[1])
		}
		html += `</table>`
	}

	if ctaLabel != "" {
		html += emailButton(ctaLabel, frontendLink(ctaPath))
	}

	if footnote != "" {
		html += emailNote(footnote)
	}
	return brandedEmailShell(title, html)
}
