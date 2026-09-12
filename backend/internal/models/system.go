package models

// Usuario de sistema "Obertrack": publica DMs automáticos (tarea asignada,
// fecha cambiada, completada) en el chat interno. Es de tipo superadmin, así que
// GetActiveUsers lo excluye del selector de chat y del auto-join de canales
// públicos, y el frontend de Tareas lo excluye de asignables/menciones.
const (
	SystemBotEmail = "bot@obertrack.system"
	SystemBotName  = "Obertrack"
)

// Cuenta de servicio "Obersuite": el autor técnico de lo que Obersuite escribe
// en el Expediente (las notas de Reclutamiento). Existe porque cada entrada
// tiene un autor que es un usuario de Obertrack, y quien escribe desde
// Obersuite no lo es: el nombre real de esa persona viaja aparte, en
// author_name. Sistema y superadmin por lo mismo que el bot: no inicia sesión
// y no aparece en ningún selector.
const (
	ObersuiteServiceEmail = "obersuite@obertrack.system"
	ObersuiteServiceName  = "Obersuite"
)
