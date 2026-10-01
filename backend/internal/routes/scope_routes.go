package routes

import "github.com/obertrack/backend/internal/middleware"

// scopeAllowlist son las únicas rutas que puede usar una sesión acotada
// (contrato CRM, §9). Todo lo demás responde 403.
//
// La lista de "tasks" sale de lo que consume de verdad la vista de Tareas
// (pages/Tasks.tsx y sus componentes), no de lo que cuelga de /tasks: el
// constructor de automatizaciones, las notificaciones, el chat o el selector de
// empresa quedan fuera aunque la página normal los use. scope_routes_test.go
// comprueba que cada entrada existe y que nada fuera de la lista pasa.
var scopeAllowlist = middleware.ScopeAllowlist{
	middleware.ScopeTasks: routeSet(
		// Quién es el usuario y qué permisos tiene.
		"GET /api/auth/me",

		// Tableros y fases.
		"GET /api/boards",
		"GET /api/boards/public",
		"POST /api/boards",
		"PUT /api/boards/:id",
		"DELETE /api/boards/:id",
		"POST /api/boards/:id/phases",
		"DELETE /api/boards/:id/phases/:phaseId",
		"PUT /api/boards/:id/phases/reorder",

		// Miembros, invitaciones y solicitudes de un tablero.
		"POST /api/boards/:id/invite",
		"POST /api/boards/:id/request",
		"GET /api/boards/:id/requests",
		"GET /api/boards/:id/invitations",
		"DELETE /api/boards/:id/members/:userId",
		"POST /api/boards/:id/leave",
		"GET /api/board-invitations/mine",
		"POST /api/board-invitations/:invId/accept",
		"POST /api/board-invitations/:invId/reject",
		"DELETE /api/board-invitations/:invId",

		// Tareas, comentarios y adjuntos.
		"GET /api/tasks",
		"GET /api/tasks/status-counts",
		"POST /api/tasks",
		"GET /api/tasks/:id",
		"GET /api/tasks/:id/history",
		"PUT /api/tasks/reorder",
		"PUT /api/tasks/:id",
		"DELETE /api/tasks/:id",
		"POST /api/tasks/:id/comments",
		"PUT /api/tasks/:id/comments/:commentId",
		"DELETE /api/tasks/:id/comments/:commentId",
		"POST /api/tasks/:id/attachments",
		"DELETE /api/tasks/:id/attachments/:attachmentId",

		// Archivos: imágenes pegadas en el editor, campos de archivo de las
		// puertas y descarga de adjuntos.
		"POST /api/uploads",
		"GET /api/uploads/:filename",

		// Selector de responsables y miembros.
		"GET /api/users",

		// Automatizaciones: nada. La vista embebida no enseña el constructor ni
		// su indicador; las puertas que exige mover una tarjeta llegan en la
		// respuesta 422 del propio PUT /api/tasks/:id.
	),

	// Vista de Horas (pages/WorkHours.tsx). A quién puede ver, editar o
	// aprobar cada uno lo sigue decidiendo el servicio, igual que en la vista
	// normal; aquí solo se cierra lo que la vista embebida no usa.
	middleware.ScopeHours: routeSet(
		// Quién es el usuario y qué permisos tiene.
		"GET /api/auth/me",

		// Registros, resumen del mes y pendientes de aprobar.
		"GET /api/work-hours",
		"GET /api/work-hours/summary",
		"GET /api/work-hours/pending",

		// Registrar, recuperar y editar una jornada.
		"POST /api/work-hours",
		"PUT /api/work-hours/:id",

		// Aprobar y rechazar (managers, supervisores y la cuenta de empresa).
		// El lote omite las jornadas propias: lo decide el servicio.
		"POST /api/work-hours/approve",
		"POST /api/work-hours/reject",

		// Filtro por profesional de la cuenta de empresa.
		"GET /api/users/employees",

		// Imágenes pegadas en el editor de actividades.
		"POST /api/uploads",
		"GET /api/uploads/:filename",

		// Fuera a propósito: POST /api/work-hours/send-report (manda correos;
		// decisión del contrato), GET /api/work-hours/report/pdf|excel (la
		// vista genera PDF y Excel en el navegador) y todo /api/admin (solo
		// superadmin y CS, que el canje rechaza siempre).
	),
}

func routeSet(routes ...string) map[string]bool {
	set := make(map[string]bool, len(routes))
	for _, r := range routes {
		set[r] = true
	}
	return set
}
