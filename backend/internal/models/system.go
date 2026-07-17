package models

// Usuario de sistema "Obertrack": publica DMs automáticos (tarea asignada,
// fecha cambiada, completada) en el chat interno. Es de tipo superadmin, así que
// GetActiveUsers lo excluye del selector de chat y del auto-join de canales
// públicos, y el frontend de Tareas lo excluye de asignables/menciones.
const (
	SystemBotEmail = "bot@obertrack.system"
	SystemBotName  = "Obertrack"
)
