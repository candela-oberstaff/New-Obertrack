package models

import "time"

// TaskStatusHistory es la bitácora de movimientos de una tarea entre columnas: una
// fila por cambio de estado, incluida la de creación (FromStatus vacío).
//
// Existe porque ninguna de las dos fuentes que ya había responde a "cuándo se movió":
// tasks.updated_at se pisa al editar cualquier campo (basta cambiar el título), y las
// entradas kind="data" de audit_logs guardan sólo el valor NUEVO y se escriben sin
// actor. Sobre esta tabla se apoyan la antigüedad en columna que ve el usuario y el
// disparador schedule.task_stale del motor de workflows.
//
// Alcance deliberadamente acotado al ESTADO. La idempotencia del motor NO se apoya
// aquí sino en tasks.revision: generalizar esto a un historial de cualquier campo
// obligaría a diferenciar conjuntos de asignados dentro de SyncAssignees y a decidir
// un formato de from/to para campos que no son escalares.
type TaskStatusHistory struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	TaskID   uint `gorm:"not null;index:idx_tsh_task_changed_at" json:"task_id"`
	TenantID uint `gorm:"not null;index" json:"tenant_id"`
	// FromStatus vacío significa que la tarea acaba de crearse.
	FromStatus string `gorm:"size:50" json:"from_status"`
	ToStatus   string `gorm:"size:50;not null" json:"to_status"`
	// ChangedBy nulo = el cambio no lo hizo una persona (worker, seed, motor).
	ChangedBy *uint     `gorm:"index" json:"changed_by,omitempty"`
	ChangedAt time.Time `gorm:"not null;index:idx_tsh_task_changed_at" json:"changed_at"`
	// GateWorkflowID y FormData registran el formulario con el que se cruzó una
	// PUERTA de fase: qué regla lo exigió y qué se respondió. Nulos en los
	// movimientos normales, que no pasan por ninguna puerta.
	//
	// Aquí es donde el concepto pedía que quedara rastro de "qué usuario aprobó,
	// cuándo lo hizo y qué datos adjuntó", y por eso este camino escribe la
	// bitácora DENTRO de la transacción del cambio de estado en vez de
	// best-effort: si el registro no se puede guardar, el movimiento no ocurre.
	GateWorkflowID *uint `gorm:"index" json:"gate_workflow_id,omitempty"`
	// FormData es PUNTERO y no cadena a propósito. La columna es jsonb, y la cadena
	// vacía no es JSON válido: con un string, cada movimiento SIN puerta —la inmensa
	// mayoría— intentaba insertar '' y Postgres rechazaba la fila entera. Como esta
	// escritura es best-effort, el error sólo quedaba en el log y la bitácora se
	// vaciaba sin que nada fallara a la vista.
	//
	// Con puntero, el valor por defecto es NULL, que es además lo que de verdad
	// significa: aquí no se cruzó ninguna puerta.
	FormData *string `gorm:"type:jsonb" json:"form_data,omitempty"`
}

func (TaskStatusHistory) TableName() string {
	return "task_status_history"
}
