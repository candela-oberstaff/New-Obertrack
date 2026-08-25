package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// ErrRunAlreadyQueued lo devuelve EnqueueRun cuando el cambio ya estaba encolado
// para esa regla. No es un fallo: es exactamente lo que el índice único tiene que
// impedir, y el llamador lo trata como "ya está hecho".
var ErrRunAlreadyQueued = errors.New("la ejecución ya estaba encolada")

type WorkflowRepository interface {
	// --- Reglas ---

	// ListEnabledByTrigger devuelve las reglas ACTIVAS de una empresa para un
	// disparador, con sus pasos ya cargados. Es la consulta del camino caliente:
	// corre dentro del request cada vez que alguien toca una tarea, así que va
	// por el índice (tenant_id, trigger_type) y no evalúa condiciones.
	ListEnabledByTrigger(tenantID uint, triggerType string) ([]models.Workflow, error)
	GetWorkflow(id uint) (*models.Workflow, error)
	CreateWorkflow(wf *models.Workflow) error
	SetEnabled(id uint, enabled bool) error
	// SetTriggerConfig cambia el ámbito fino de una regla ya creada: hoy, la columna
	// sobre la que actúa una puerta. Sin esto, equivocarse de columna al activarla
	// obligaba a borrarla y volver a crearla, perdiendo su historial de ejecuciones.
	SetTriggerConfig(id uint, triggerConfig string) error
	// UpdateGate reescribe una puerta propia en un solo statement: nombre, columna,
	// formulario e interruptor. Van juntos porque se guardan juntos desde el
	// constructor, y aplicar la mitad dejaría una puerta pidiendo un formulario en
	// una columna que ya no es la suya.
	UpdateGate(id uint, name, triggerConfig, formSchema string, enabled bool) error
	// DeleteWorkflow borra una regla. El borrado es lógico: su historial de
	// ejecuciones sigue explicando los movimientos que bloqueó.
	DeleteWorkflow(id uint) error
	// ListByBoard devuelve las reglas de un tablero (activas y apagadas), con sus
	// pasos. Es lo que alimenta la pantalla de automatizaciones.
	ListByBoard(tenantID, boardID uint) ([]models.Workflow, error)
	// FindByRecipe localiza la regla materializada de una receta en un tablero, o
	// nil si esa receta nunca se activó allí. Permite volver a encender una receta
	// apagada sin duplicarla.
	FindByRecipe(tenantID, boardID uint, recipeKey string) (*models.Workflow, error)

	// --- Cola de ejecuciones ---

	// EnqueueRun encola una ejecución. Devuelve ErrRunAlreadyQueued si el mismo
	// cambio ya estaba encolado para la misma regla.
	EnqueueRun(run *models.WorkflowRun) error
	// ClaimRuns toma hasta `limit` ejecuciones pendientes ya vencidas y las marca
	// 'running' en el MISMO statement, con FOR UPDATE SKIP LOCKED. Que la toma sea
	// atómica es lo que permite más de un worker: dos procesos que corran esto a
	// la vez se reparten filas en vez de ejecutar las mismas dos veces.
	ClaimRuns(limit int, now time.Time) ([]models.WorkflowRun, error)
	// RequeueStale devuelve a 'pending' las ejecuciones que quedaron 'running' de
	// un proceso que murió a mitad. Sin esto, un reinicio en mal momento las deja
	// colgadas para siempre, porque ClaimRuns sólo mira las pendientes.
	RequeueStale(olderThan time.Time) (int64, error)
	MarkRunDone(runID uint) error
	MarkRunSkipped(runID uint, reason string) error
	// MarkRunFailed guarda el error y decide el destino según retryAt: con fecha
	// vuelve a 'pending' para reintentarse; con nil se agotó y queda en 'failed'.
	// Que la decisión viaje como fecha hace imposible dejar una fila 'pending' sin
	// próxima fecha, o una 'failed' con ella.
	MarkRunFailed(runID uint, errMsg string, retryAt *time.Time) error
	GetRun(runID uint) (*models.WorkflowRun, error)
	ListRuns(workflowID uint, limit int) ([]models.WorkflowRun, error)

	// --- Pasos de una ejecución ---

	SaveStepRun(entry *models.WorkflowStepRun) error
	// DoneStepIDs devuelve los pasos que ya se completaron en esta ejecución, para
	// que un reintento no repita los avisos que ya salieron.
	DoneStepIDs(runID uint) (map[uint]bool, error)
	ListStepRuns(runID uint) ([]models.WorkflowStepRun, error)

	// TenantsWithTrigger devuelve las empresas que tienen alguna regla ACTIVA para un
	// disparador. Lo usa el barrido del tiempo para no recorrer las tareas de quien no
	// ha encendido nada: sin esto, cada hora se leerían todas las tareas vencidas de
	// la plataforma para descartarlas casi siempre.
	TenantsWithTrigger(triggerType string) ([]uint, error)

	// PurgeRunsBefore borra las ejecuciones terminadas anteriores a una fecha, con
	// sus pasos. Las FALLIDAS se conservan más tiempo: son las que alguien va a
	// consultar cuando pregunte por qué no llegó un aviso, y son pocas.
	PurgeRunsBefore(done, failed time.Time) (int64, error)

	// CauseChainWorkflowIDs devuelve los ids de regla de la cadena causal que
	// llevó a runID, empezando por la suya. Con esto el motor comprueba que una
	// regla no se dispare a sí misma aunque la cadena pase por otras.
	CauseChainWorkflowIDs(runID uint) ([]uint, error)
}

func (r *workflowRepository) PurgeRunsBefore(done, failed time.Time) (int64, error) {
	var borradas int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Qué ejecuciones caducaron. Las pendientes y las que están corriendo no se
		// tocan NUNCA, aunque sean viejas: una fila pendiente es trabajo por hacer, y
		// borrarla es perder el aviso, no limpiarlo.
		var ids []uint
		if err := tx.Model(&models.WorkflowRun{}).
			Where("(status IN ? AND created_at < ?) OR (status = ? AND created_at < ?)",
				[]string{models.WorkflowRunDone, models.WorkflowRunSkipped}, done,
				models.WorkflowRunFailed, failed).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		// Primero los pasos: son hijos de la ejecución y sin ella no significan nada.
		if err := tx.Where("run_id IN ?", ids).Delete(&models.WorkflowStepRun{}).Error; err != nil {
			return err
		}
		// La cadena causal apunta a ejecuciones anteriores. Se corta antes de borrar
		// para no dejar referencias a filas que ya no están.
		if err := tx.Model(&models.WorkflowRun{}).
			Where("cause_run_id IN ?", ids).
			Update("cause_run_id", nil).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", ids).Delete(&models.WorkflowRun{})
		borradas = res.RowsAffected
		return res.Error
	})
	return borradas, err
}

func (r *workflowRepository) UpdateGate(id uint, name, triggerConfig, formSchema string, enabled bool) error {
	return r.db.Model(&models.Workflow{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"name":           name,
			"trigger_config": triggerConfig,
			"form_schema":    formSchema,
			"enabled":        enabled,
		}).Error
}

func (r *workflowRepository) DeleteWorkflow(id uint) error {
	return r.db.Delete(&models.Workflow{}, id).Error
}

func (r *workflowRepository) TenantsWithTrigger(triggerType string) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.Workflow{}).
		Where("trigger_type = ? AND enabled = ? AND deleted_at IS NULL", triggerType, true).
		Distinct().
		Pluck("tenant_id", &ids).Error
	return ids, err
}

type workflowRepository struct {
	db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) WorkflowRepository {
	return &workflowRepository{db: db}
}

func (r *workflowRepository) ListEnabledByTrigger(tenantID uint, triggerType string) ([]models.Workflow, error) {
	var wfs []models.Workflow
	err := r.db.
		Where("tenant_id = ? AND trigger_type = ? AND enabled = ?", tenantID, triggerType, true).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("workflow_steps.\"order\" ASC, workflow_steps.id ASC")
		}).
		Find(&wfs).Error
	return wfs, err
}

func (r *workflowRepository) GetWorkflow(id uint) (*models.Workflow, error) {
	var wf models.Workflow
	err := r.db.
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("workflow_steps.\"order\" ASC, workflow_steps.id ASC")
		}).
		First(&wf, id).Error
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

func (r *workflowRepository) CreateWorkflow(wf *models.Workflow) error {
	return r.db.Create(wf).Error
}

func (r *workflowRepository) ListByBoard(tenantID, boardID uint) ([]models.Workflow, error) {
	var wfs []models.Workflow
	// El filtro por tenant es redundante con el de tablero (un tablero pertenece a
	// un solo tenant) y se pone igualmente: es la barrera que hay que poder señalar
	// en cada consulta del motor sin depender de una invariante de otra tabla.
	err := r.db.
		Where("tenant_id = ? AND board_id = ?", tenantID, boardID).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("workflow_steps.\"order\" ASC, workflow_steps.id ASC")
		}).
		Order("id ASC").
		Find(&wfs).Error
	return wfs, err
}

func (r *workflowRepository) FindByRecipe(tenantID, boardID uint, recipeKey string) (*models.Workflow, error) {
	var wf models.Workflow
	err := r.db.
		Where("tenant_id = ? AND board_id = ? AND recipe_key = ?", tenantID, boardID, recipeKey).
		First(&wf).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

func (r *workflowRepository) SetEnabled(id uint, enabled bool) error {
	return r.db.Model(&models.Workflow{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (r *workflowRepository) SetTriggerConfig(id uint, triggerConfig string) error {
	return r.db.Model(&models.Workflow{}).Where("id = ?", id).Update("trigger_config", triggerConfig).Error
}

func (r *workflowRepository) EnqueueRun(run *models.WorkflowRun) error {
	err := r.db.Create(run).Error
	if err != nil && isUniqueViolation(err) {
		return ErrRunAlreadyQueued
	}
	return err
}

// isUniqueViolation reconoce el choque contra idx_wf_run_dedup. Se mira el texto
// porque el driver envuelve el error de Postgres y comprobar el SQLSTATE aquí
// obligaría a importar pgconn sólo para esto.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "sqlstate 23505") ||
		strings.Contains(msg, "unique constraint")
}

// ClaimRuns toma y marca en un solo statement. El SELECT interno bloquea las filas
// que va a tomar y SKIP LOCKED hace que otro worker simplemente pase de largo en vez
// de esperar, así que la cola escala a varios procesos sin coordinación externa.
//
// attempts se incrementa AL TOMAR, no al fallar: si el proceso muere a mitad de una
// ejecución, ese intento igualmente cuenta y la ejecución no puede quedar
// reintentándose eternamente por morir siempre en el mismo punto.
func (r *workflowRepository) ClaimRuns(limit int, now time.Time) ([]models.WorkflowRun, error) {
	var runs []models.WorkflowRun
	err := r.db.Raw(`
		UPDATE workflow_runs SET
			status = ?,
			attempts = attempts + 1,
			started_at = ?,
			updated_at = ?
		WHERE id IN (
			SELECT id FROM workflow_runs
			WHERE status = ?
			  AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
			ORDER BY id ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		RETURNING *`,
		models.WorkflowRunRunning, now, now,
		models.WorkflowRunPending, now, limit,
	).Scan(&runs).Error
	return runs, err
}

func (r *workflowRepository) RequeueStale(olderThan time.Time) (int64, error) {
	res := r.db.Model(&models.WorkflowRun{}).
		Where("status = ? AND started_at IS NOT NULL AND started_at <= ?",
			models.WorkflowRunRunning, olderThan).
		Updates(map[string]interface{}{
			"status":          models.WorkflowRunPending,
			"next_attempt_at": nil,
		})
	return res.RowsAffected, res.Error
}

func (r *workflowRepository) MarkRunDone(runID uint) error {
	now := time.Now()
	return r.db.Model(&models.WorkflowRun{}).Where("id = ?", runID).
		Updates(map[string]interface{}{
			"status":          models.WorkflowRunDone,
			"next_attempt_at": nil,
			"last_error":      "",
			"finished_at":     now,
		}).Error
}

func (r *workflowRepository) MarkRunSkipped(runID uint, reason string) error {
	now := time.Now()
	return r.db.Model(&models.WorkflowRun{}).Where("id = ?", runID).
		Updates(map[string]interface{}{
			"status":          models.WorkflowRunSkipped,
			"next_attempt_at": nil,
			"skip_reason":     reason,
			"finished_at":     now,
		}).Error
}

func (r *workflowRepository) MarkRunFailed(runID uint, errMsg string, retryAt *time.Time) error {
	updates := map[string]interface{}{
		// next_attempt_at se escribe siempre, incluido el nil de una ejecución
		// agotada: dejar la fecha del intento anterior en una fila 'failed' sólo
		// confunde a quien lea la tabla para diagnosticar.
		"next_attempt_at": retryAt,
		"last_error":      errMsg,
	}
	if retryAt == nil {
		updates["status"] = models.WorkflowRunFailed
		updates["finished_at"] = time.Now()
	} else {
		updates["status"] = models.WorkflowRunPending
	}
	return r.db.Model(&models.WorkflowRun{}).Where("id = ?", runID).Updates(updates).Error
}

func (r *workflowRepository) GetRun(runID uint) (*models.WorkflowRun, error) {
	var run models.WorkflowRun
	if err := r.db.First(&run, runID).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *workflowRepository) ListRuns(workflowID uint, limit int) ([]models.WorkflowRun, error) {
	var runs []models.WorkflowRun
	q := r.db.Where("workflow_id = ?", workflowID).Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return runs, q.Find(&runs).Error
}

// SaveStepRun normaliza Output antes de insertar. La columna es jsonb y la cadena
// vacía NO es JSON válido: un paso saltado o fallido —que por definición no deja
// salida— reventaba el INSERT con "invalid input syntax for type json". Como el
// llamador descarta el error de la bitácora para no tumbar la ejecución por ella,
// esos pasos desaparecían en silencio y una regla que decidió no actuar quedaba
// indistinguible de una que no llegó a ejecutarse.
func (r *workflowRepository) SaveStepRun(entry *models.WorkflowStepRun) error {
	if strings.TrimSpace(entry.Output) == "" {
		entry.Output = "{}"
	}
	return r.db.Create(entry).Error
}

func (r *workflowRepository) DoneStepIDs(runID uint) (map[uint]bool, error) {
	var ids []uint
	err := r.db.Model(&models.WorkflowStepRun{}).
		Where("run_id = ? AND status = ?", runID, models.WorkflowStepDone).
		Pluck("step_id", &ids).Error
	if err != nil {
		return nil, err
	}
	done := make(map[uint]bool, len(ids))
	for _, id := range ids {
		done[id] = true
	}
	return done, nil
}

func (r *workflowRepository) ListStepRuns(runID uint) ([]models.WorkflowStepRun, error) {
	var rows []models.WorkflowStepRun
	return rows, r.db.Where("run_id = ?", runID).Order("id ASC").Find(&rows).Error
}

// CauseChainWorkflowIDs sube por cause_run_id. El recorrido se acota a
// MaxWorkflowDepth+1 saltos: más arriba no puede haber nada porque el propio tope
// de profundidad lo impide, y así una cadena corrupta no cuelga al worker.
func (r *workflowRepository) CauseChainWorkflowIDs(runID uint) ([]uint, error) {
	ids := []uint{}
	current := &runID
	for i := 0; i <= models.MaxWorkflowDepth+1 && current != nil; i++ {
		var row struct {
			WorkflowID uint
			CauseRunID *uint
		}
		err := r.db.Model(&models.WorkflowRun{}).
			Select("workflow_id, cause_run_id").
			Where("id = ?", *current).
			Scan(&row).Error
		if err != nil {
			return ids, err
		}
		if row.WorkflowID == 0 {
			break
		}
		ids = append(ids, row.WorkflowID)
		current = row.CauseRunID
	}
	return ids, nil
}
