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
}

func routeSet(routes ...string) map[string]bool {
	set := make(map[string]bool, len(routes))
	for _, r := range routes {
		set[r] = true
	}
	return set
}
