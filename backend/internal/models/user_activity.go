package models

import "time"

// Módulos de producto que se contabilizan en user_activity_daily. Son nombres
// de PRODUCTO (lo que la persona cree estar usando), no de ruta: /api/boards y
// /api/tasks son las dos "tareas". La traducción vive en middleware/activity.go.
const (
	// ModuleApp se anota en TODA petición autenticada, sea del módulo que sea.
	// Es el denominador de la adopción —"esta persona abrió la app este día"—,
	// así que nunca se filtra por ruido: si hubo petición, hubo uso.
	ModuleApp = "app"

	ModuleChat      = "chat"
	ModuleTasks     = "tareas"
	ModuleHours     = "horas"
	ModuleMeetings  = "sesiones"
	ModuleSupport   = "soporte"
	ModuleSurveys   = "encuestas"
	ModuleNews      = "novedades"
	ModuleAdmin     = "admin"
	ModuleCompany   = "empresa"
	ModulePeople    = "usuarios"
	ModuleEmail     = "correos"
	ModuleTestimony = "testimonios"
	ModuleWorkflows = "automatizaciones"
	ModuleProfile   = "perfil"
	ModuleInduction = "induccion"
	ModuleWallet    = "wallet"
	ModuleReports   = "reportes"
)

// UserActivityDaily es el contador de uso REAL de la app: una fila por
// (usuario, día, módulo) con cuántas peticiones hizo y cuándo fue la última.
//
// Existe porque audit_logs solo registra escrituras (POST/PUT/PATCH/DELETE):
// sirve para saber QUÉ cambió alguien, no si USA la app. Quien entra al chat
// todos los días a leer, o abre Horas a consultar, no dejaba rastro ninguno,
// así que cualquier "% de uso" salido de la auditoría subestimaba justo a los
// módulos de consulta —que son la mayoría del uso diario—.
//
// Se guarda agregado por día y no evento a evento a propósito: la pregunta que
// responde es "¿cuánta gente y qué empresas usan esto?", y para eso una fila
// por persona/día/módulo basta. Un registro por petición multiplicaría el
// volumen por cien sin contestar nada nuevo, y audit_logs ya crece sin poda.
//
// Hits sirve para ordenar (quién usa MÁS), nunca como métrica de portada: el
// número de peticiones depende de cómo esté escrita cada pantalla, no de lo
// que la persona hizo. La métrica de portada siempre es gente distinta.
type UserActivityDaily struct {
	ID     uint      `gorm:"primaryKey" json:"id"`
	UserID uint      `gorm:"not null;uniqueIndex:idx_activity_user_day_module,priority:1" json:"user_id"`
	Day    time.Time `gorm:"type:date;not null;uniqueIndex:idx_activity_user_day_module,priority:2;index" json:"day"`
	Module string    `gorm:"size:32;not null;uniqueIndex:idx_activity_user_day_module,priority:3" json:"module"`
	// TenantID es la empresa que tenía la sesión al momento de la petición. Se
	// guarda por comodidad de consulta; el agrupado por empresa de las métricas
	// se hace contra users, que es la fuente de verdad y sigue a la persona si
	// la cambian de empresa.
	TenantID uint      `gorm:"index" json:"tenant_id"`
	Hits     int       `gorm:"not null;default:0" json:"hits"`
	LastAt   time.Time `json:"last_at"`
}

func (UserActivityDaily) TableName() string {
	return "user_activity_daily"
}
