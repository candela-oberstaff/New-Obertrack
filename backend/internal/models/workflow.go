package models

import (
	"time"

	"gorm.io/gorm"
)

// Disparadores soportados. El prefijo es la entidad; lo que sigue, el hecho.
const (
	TriggerTaskCreated         = "task.created"
	TriggerTaskStatusChanged   = "task.status_changed"
	TriggerTaskPriorityChanged = "task.priority_changed"
	TriggerTaskAssigned        = "task.assigned"
	// Los dos siguientes NO los provoca nadie: los emite un barrido al pasar el
	// tiempo. Sin ellos el motor sólo reaccionaba a que alguien tocara una tarjeta, y
	// una tarea que vence el martes y se queda quieta no producía nada — que es justo
	// la que hay que mirar.
	TriggerTaskOverdue = "task.overdue"
	TriggerTaskDueSoon = "task.due_soon"
)

// Acciones soportadas en la Fase 1: sólo avisos. Las que mutan la tarea llegan en
// la Fase 2 y necesitan antes el actor de sistema y la comprobación de obsolescencia.
const (
	ActionNotify = "notify"
	ActionChatDM = "chat_dm"
	ActionEmail  = "email"

	// Acciones que MUTAN la tarea. A diferencia de los avisos, éstas cambian el dato
	// y por eso comprueban antes que la tarea siga como estaba al dispararse: aplicar
	// sobre algo que ya cambió es peor que no aplicar nada.
	ActionSetPriority = "task_set_priority"
	ActionAssign      = "task_assign"
	ActionComment     = "task_comment"
	ActionSetStatus   = "task_set_status"
)

// Clases de destinatario. Se resuelven EN EJECUCIÓN, nunca al guardar la regla:
// así una regla sobrevive a renuncias, cambios de manager y reasignaciones de
// tablero sin que nadie tenga que reeditarla.
const (
	RecipientAssignees = "asignados"
	// RecipientNewAssignees son sólo los que ACABAN de sumarse. "Te asignaron esta
	// tarea" dirigido a quien ya la tenía desde hace una semana no dice nada.
	RecipientNewAssignees    = "nuevos_asignados"
	RecipientAssigneeManager = "manager_del_asignado"
	RecipientBoardSupervisor = "supervisor_del_tablero"
	RecipientTaskCreator     = "creador_de_la_tarea"
	RecipientBoardCreator    = "creador_del_tablero"
	RecipientEmployer        = "empleador"
	RecipientActor           = "actor"
	RecipientFixedUser       = "usuario_fijo"
	// RecipientLeastLoaded es quien está MÁS LIBRE del tablero: el miembro con menos
	// tareas abiertas de toda la empresa. Repartir por jerarquía amontona el trabajo
	// siempre en la misma persona; lo terminado no cuenta como carga, que es
	// justamente lo que hace que quien va al día vuelva a estar disponible.
	RecipientLeastLoaded = "menos_cargado"
	// RecipientProjectLead es la cadena de respaldo del "líder del proyecto":
	// manager del asignado → supervisor del tablero → creador del tablero →
	// empleador. Existe porque el modelo no tiene un rol de líder por tablero.
	RecipientProjectLead = "lider_del_proyecto"
)

// Estados de una ejecución.
const (
	WorkflowRunPending = "pending"
	WorkflowRunRunning = "running"
	WorkflowRunDone    = "done"
	WorkflowRunFailed  = "failed"
	// WorkflowRunSkipped es un final legítimo, no un error: las condiciones no se
	// cumplieron, no había a quién avisar, o la cadena causal llegó al tope.
	WorkflowRunSkipped = "skipped"
)

// Estados de un paso dentro de una ejecución.
const (
	WorkflowStepDone    = "done"
	WorkflowStepFailed  = "failed"
	WorkflowStepSkipped = "skipped"
)

const (
	// MaxWorkflowDepth acota la cadena causal: una acción puede provocar un evento
	// que dispare otra regla, pero no indefinidamente. Tres niveles cubren los
	// encadenamientos que tienen sentido para un usuario y cortan cualquier bucle.
	MaxWorkflowDepth = 3

	// WorkflowMaxAttempts es el tope de intentos de una ejecución antes de darla
	// por fallida.
	WorkflowMaxAttempts = 6
)

// workflowBackoff es la espera ANTES de cada reintento: la posición i es lo que se
// espera tras el intento i+1. Misma escalera que la cola de Google Calendar y por
// la misma razón: los fallos que se recuperan solos (una caída de Brevo, un
// incidente de red) duran minutos u horas, no segundos, y con un intervalo fijo los
// seis intentos se agotarían en dos minutos sin haber dado tiempo a nada.
//
// Se define aparte de calendarSyncBackoff a propósito: si algún día hay que ajustar
// la ventana de Google por una razón suya, no debe arrastrar a los workflows.
var workflowBackoff = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	8 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
}

// WorkflowRetryDelay devuelve cuánto esperar tras `attempts` intentos fallidos. Se
// satura en el último escalón, así que subir WorkflowMaxAttempts nunca desborda.
func WorkflowRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(workflowBackoff) {
		attempts = len(workflowBackoff)
	}
	return workflowBackoff[attempts-1]
}

// Workflow es una regla de automatización de una empresa: un disparador, unas
// condiciones y una lista de pasos que se ejecutan en orden.
//
// Nace SIEMPRE apagada (Enabled=false): una regla que empieza a avisar a gente en
// cuanto se guarda no da margen a revisarla.
type Workflow struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	TenantID    uint   `gorm:"not null;index:idx_wf_tenant_trigger" json:"tenant_id"`
	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Enabled     bool   `gorm:"not null;default:false;index" json:"enabled"`
	TriggerType string `gorm:"size:64;not null;index:idx_wf_tenant_trigger" json:"trigger_type"`
	// BoardID es el ámbito de la regla: vive dentro de UN tablero. Es columna real
	// y no un campo del JSON porque es lo que se consulta —para listar las reglas
	// de un tablero, y para comprobar el alcance de quien las crea— y porque un
	// ámbito escondido en un JSON es un ámbito que alguien acabará olvidando
	// filtrar. Que sea obligatorio es lo que hace imposible que una regla alcance
	// otro tablero, y por tanto otro tenant.
	BoardID uint `gorm:"not null;index:idx_wf_board_recipe" json:"board_id"`
	// RecipeKey vincula la regla con la receta de la que salió, o queda vacío si se
	// escribió a mano. Sirve para saber si una receta ya está materializada en un
	// tablero sin tener que adivinarlo por el nombre.
	RecipeKey string `gorm:"size:64;index:idx_wf_board_recipe" json:"recipe_key,omitempty"`
	// TriggerConfig afina el disparador dentro del tablero. Hoy sólo admite
	// phase_id ("sólo cuando caiga en esta columna"), guardado como id y no como
	// cadena de status para que renombrar la columna no deje la regla apuntando al
	// vacío.
	//   {"phase_id": 12}
	TriggerConfig string `gorm:"type:jsonb;not null;default:'{}'" json:"trigger_config"`
	// Conditions es un árbol {"all":[...]} / {"any":[...]} de comparaciones
	// {"field","op","value"}. Vacío = sin condiciones, la regla siempre aplica.
	Conditions string `gorm:"type:jsonb;not null;default:'{}'" json:"conditions"`
	// FormSchema es el formulario que exige una PUERTA de fase (trigger_type
	// task.entering_phase). Vacío en los disparadores reactivos, que no piden nada.
	// Viaja tal cual al cliente dentro de la respuesta 422, y eso es lo que permite
	// que un cliente sin actualizar dibuje un formulario que no conocía.
	FormSchema string `gorm:"type:jsonb;not null;default:'{}'" json:"form_schema"`
	// CreatedBy es de quién hereda el alcance la regla: el runner comprueba en
	// cada ejecución que esta persona siga alcanzando el tablero, porque puede
	// haberlo perdido después de crearla.
	CreatedBy uint           `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Steps []WorkflowStep `gorm:"foreignKey:WorkflowID" json:"steps,omitempty"`
}

func (Workflow) TableName() string { return "workflows" }

// WorkflowStep es una acción dentro de una regla. Se ejecutan en secuencia porque
// el orden importa: "sube la prioridad y LUEGO avisa" no es lo mismo al revés.
type WorkflowStep struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	WorkflowID uint   `gorm:"not null;index" json:"workflow_id"`
	Order      int    `gorm:"not null;default:0" json:"order"`
	ActionType string `gorm:"size:64;not null" json:"action_type"`
	// Config lleva lo propio de cada acción, con {{variables}} sin resolver:
	//   {"recipient":"manager_del_asignado","title":"...","message":"..."}
	Config string `gorm:"type:jsonb;not null;default:'{}'" json:"config"`
	// Conditions acota ESTE paso, además de las de la regla entera. Es lo que
	// permite que una misma regla haga una cosa u otra según lo que respondieran:
	// aprobar cierra la tarea, rechazar la devuelve. Sin esto harían falta dos
	// reglas sobre la misma columna, y por tanto dos formularios para una decisión.
	// Vacío o "{}" = el paso se ejecuta siempre.
	Conditions string         `gorm:"type:jsonb;not null;default:'{}'" json:"conditions"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (WorkflowStep) TableName() string { return "workflow_steps" }

// WorkflowRun es, a la vez, la bitácora de una ejecución y la cola que la sostiene:
// la fila ES el trabajo pendiente. Mismo contrato que CalendarSyncJob, que ya lleva
// tiempo en producción, con dos añadidos propios del motor: la clave de
// idempotencia y la cadena causal.
type WorkflowRun struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	WorkflowID uint `gorm:"not null;uniqueIndex:idx_wf_run_dedup" json:"workflow_id"`
	TenantID   uint `gorm:"not null;index" json:"tenant_id"`
	// DedupKey identifica el CAMBIO que disparó la ejecución:
	// sha1(disparador + entidad + id + revisión). El índice único con workflow_id
	// hace que encolar dos veces el mismo cambio para la misma regla sea
	// imposible, no sólo improbable, y que siga siéndolo tras un reinicio.
	DedupKey   string `gorm:"size:64;not null;uniqueIndex:idx_wf_run_dedup" json:"dedup_key"`
	EntityType string `gorm:"size:32;not null" json:"entity_type"`
	EntityID   uint   `gorm:"not null;index" json:"entity_id"`
	// Context es el snapshot del momento del disparo (antes, después, actor). Las
	// variables y las condiciones se resuelven CONTRA ESTO y no releyendo la base,
	// de forma que un reintento produzca exactamente el mismo resultado.
	Context string `gorm:"type:jsonb;not null;default:'{}'" json:"context"`

	Status        string     `gorm:"size:20;not null;default:'pending';index:idx_wf_run_claim" json:"status"`
	Attempts      int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt *time.Time `gorm:"index:idx_wf_run_claim" json:"next_attempt_at,omitempty"`
	LastError     string     `gorm:"type:text" json:"last_error,omitempty"`
	// SkipReason explica un final 'skipped' en lenguaje llano. Sin esto, una regla
	// que no hace nada es indistinguible de una regla rota.
	SkipReason string `gorm:"type:text" json:"skip_reason,omitempty"`

	// CauseRunID y Depth son el antibucle: una ejecución nacida de la acción de
	// otra hereda depth+1 y apunta a su causa. Con la cadena se comprueba además
	// que una regla no vuelva a dispararse a sí misma.
	CauseRunID *uint `gorm:"index" json:"cause_run_id,omitempty"`
	Depth      int   `gorm:"not null;default:0" json:"depth"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (WorkflowRun) TableName() string { return "workflow_runs" }

// WorkflowStepRun es el resultado de un paso. Hace de bitácora y, a la vez, de
// marca de reanudación: un reintento salta los pasos que ya quedaron 'done', que es
// lo que impide que reintentar una ejecución a medias reenvíe los avisos ya salidos.
type WorkflowStepRun struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	RunID  uint   `gorm:"not null;index:idx_wf_step_run" json:"run_id"`
	StepID uint   `gorm:"not null;index:idx_wf_step_run" json:"step_id"`
	Order  int    `json:"order"`
	Status string `gorm:"size:20;not null" json:"status"`
	// Output es lo que el paso deja para los siguientes; se expone como
	// {{pasos.N.*}} en las plantillas.
	Output    string    `gorm:"type:jsonb" json:"output,omitempty"`
	Error     string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (WorkflowStepRun) TableName() string { return "workflow_step_runs" }
