package service

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// WorkflowService es el motor de automatizaciones. Tiene dos mitades bien separadas:
//
//   - El EMISOR (OnEvent), que corre dentro del request de quien tocó la tarea. Sólo
//     mira si hay reglas activas para (tenant, disparador) y encola una fila por
//     regla candidata. No evalúa condiciones ni envía nada: el request de un usuario
//     no puede quedar esperando a que salga un correo.
//   - El WORKER (Start/run), que toma las filas encoladas, evalúa condiciones y
//     ejecuta los pasos con reintentos.
//
// La cola vive en Postgres (workflow_runs) porque es la infraestructura que ya hay:
// no hay broker en el despliegue, y una tabla con backoff ya sostiene en producción
// la sincronización con Google Calendar y la bandeja de WhatsApp.
type WorkflowService struct {
	repo      repository.WorkflowRepository
	taskRepo  repository.TaskRepository
	boardRepo repository.BoardRepository
	userRepo  repository.UserRepository
	empRepo   repository.EmploymentRepository

	notifSvc NotificationService
	brevoSvc *BrevoService
	// postSystemDM publica un DM del bot. Inyectado como en taskService, para no
	// acoplar workflowService→channelService.
	postSystemDM func(recipientID uint, content string)
	// mutator es el módulo de Tareas para las acciones que ESCRIBEN. Inyectado por
	// SetTaskMutator; nil = las acciones que mutan quedan saltadas con motivo.
	mutator TaskMutator

	// nudge despierta al worker en cuanto se encola algo, sin esperar al tick.
	// Buffer 1 y envío no bloqueante: una ráfaga de mutaciones colapsa en un solo
	// aviso, que es justo lo que se quiere.
	nudge chan struct{}

	// quota frena una avalancha por empresa: una importación o un movimiento masivo
	// dispararían un aviso por tarjeta.
	quota *runQuota
}

// SetRunQuota fija el tope de ejecuciones por empresa y hora. 0 = sin límite.
func (s *WorkflowService) SetRunQuota(limit int) {
	s.quota = newRunQuota(limit)
}

const (
	// workflowTick es cada cuánto se revisa la cola aunque nadie la despierte. Es
	// la red de seguridad de los reintentos con backoff, que no generan nudge.
	workflowTick = 15 * time.Second
	// workflowBatch acota lo que se toma por vuelta.
	workflowBatch = 50
	// workflowStaleAfter es cuánto puede llevar una ejecución en 'running' antes de
	// darla por huérfana de un proceso que murió y devolverla a la cola.
	workflowStaleAfter = 15 * time.Minute
	// workflowRunHistory es cuántas ejecuciones devuelve el historial de una regla.
	workflowRunHistory = 50
)

func NewWorkflowService(
	repo repository.WorkflowRepository,
	taskRepo repository.TaskRepository,
	boardRepo repository.BoardRepository,
	userRepo repository.UserRepository,
	empRepo repository.EmploymentRepository,
	notifSvc NotificationService,
	brevoSvc *BrevoService,
) *WorkflowService {
	return &WorkflowService{
		repo:      repo,
		taskRepo:  taskRepo,
		boardRepo: boardRepo,
		userRepo:  userRepo,
		empRepo:   empRepo,
		notifSvc:  notifSvc,
		brevoSvc:  brevoSvc,
		nudge:     make(chan struct{}, 1),
		// Sin límite mientras nadie lo fije: el motor tiene que poder usarse en una
		// prueba sin configurar cupos.
		quota: newRunQuota(0),
	}
}

// SetSystemDM cablea el emisor de DMs del chat interno. Sin cablear, la acción
// chat_dm queda 'skipped' con motivo en vez de fallar.
func (s *WorkflowService) SetSystemDM(fn func(recipientID uint, content string)) {
	s.postSystemDM = fn
}

// ---------------------------------------------------------------------------
// Emisor (corre dentro del request)
// ---------------------------------------------------------------------------

// OnEvent es el enganche que llaman los servicios de dominio. Es deliberadamente
// barato: una consulta indexada por (tenant_id, trigger_type) y, si hay reglas, un
// INSERT por regla. Todo lo caro ocurre después, en el worker.
//
// Best-effort de principio a fin: que una automatización no llegue a encolarse
// nunca puede hacer que falle el guardado de la tarea que la provocó.
func (s *WorkflowService) OnEvent(ev WorkflowEvent) {
	if s == nil || ev.Task == nil || ev.TenantID == 0 {
		return
	}

	// Tope de la cadena causal. Se comprueba ANTES de consultar reglas: si el
	// evento nace demasiado profundo no hay nada que encolar, se mire la regla que
	// se mire.
	if ev.Depth >= models.MaxWorkflowDepth {
		log.Printf("[workflow] evento %s sobre la tarea %d descartado: cadena de %d niveles",
			ev.Type, ev.Task.ID, ev.Depth)
		return
	}

	candidates, err := s.repo.ListEnabledByTrigger(ev.TenantID, ev.Type)
	if err != nil {
		log.Printf("[workflow] no se pudieron leer las reglas de %s para el tenant %d: %v",
			ev.Type, ev.TenantID, err)
		return
	}
	if len(candidates) == 0 {
		return
	}

	// El tablero se rellena aquí si la tarea no lo trae. taskRepository.GetByID
	// precarga creador, asignados, comentarios y adjuntos, pero NO Board, así que
	// una tarea recién recargada llega con Board en cero: sin esto, el snapshot
	// guardaba nombre vacío y creador 0, y el destinatario "creador del tablero" no
	// resolvía a nadie. Se hace después de comprobar que hay reglas candidatas, de
	// modo que una empresa sin automatizaciones no paga esta consulta.
	if ev.Task.Board.ID == 0 && ev.Task.BoardID != 0 {
		if b, berr := s.boardRepo.GetByID(ev.Task.BoardID); berr == nil && b != nil {
			ev.Task.Board = *b
		}
	}

	// El nombre del actor se resuelve UNA vez por evento, no una por regla.
	actorName := ""
	if ev.ActorID > 0 {
		if u, uerr := s.userRepo.GetByID(ev.ActorID); uerr == nil && u != nil {
			actorName = u.Name
		}
	}
	ctx := buildContext(ev, actorName)
	ctxJSON := mustJSON(ctx)
	key := dedupKeyFor(ev)

	queued := 0
	for _, wf := range candidates {
		// Una puerta sólo reacciona a SU formulario. Dos puertas en columnas
		// distintas del mismo tablero pasarían igual el filtro de ámbito, y la que
		// no se cruzó acabaría decidiendo sobre respuestas que no son suyas.
		if ev.GateWorkflowID != 0 && wf.ID != ev.GateWorkflowID {
			continue
		}
		// Ámbito: la regla vive en un tablero. Filtrarlo aquí evita encolar
		// trabajo que el worker sólo va a descartar.
		if !s.scopeMatches(wf, ev) {
			continue
		}
		// Una regla nunca se dispara a sí misma, ni siquiera a través de otras.
		if s.inCauseChain(wf.ID, ev.CauseRunID) {
			continue
		}

		// Cupo de la empresa. Se descuenta por EJECUCIÓN encolada y no por evento:
		// lo que satura es el trabajo que sale de aquí, y un evento con tres reglas
		// candidatas cuesta el triple que uno con una.
		if !s.quota.allow(time.Now(), ev.TenantID) {
			continue
		}

		run := &models.WorkflowRun{
			WorkflowID: wf.ID,
			TenantID:   ev.TenantID,
			DedupKey:   key,
			EntityType: "task",
			EntityID:   ev.Task.ID,
			Context:    ctxJSON,
			Status:     models.WorkflowRunPending,
			CauseRunID: ev.CauseRunID,
			Depth:      ev.Depth,
		}
		switch err := s.repo.EnqueueRun(run); {
		case err == nil:
			queued++
		case errors.Is(err, repository.ErrRunAlreadyQueued):
			// El mismo cambio ya estaba encolado para esta regla. Es el caso que
			// el índice único existe para cubrir, no un error.
		default:
			log.Printf("[workflow] no se pudo encolar la regla %d para la tarea %d: %v",
				wf.ID, ev.Task.ID, err)
		}
	}

	if queued > 0 {
		s.signal()
	}
}

// scopeMatches comprueba el ámbito: el tablero y, opcionalmente, la columna.
//
// La columna se guarda como phase_id y se traduce aquí a su id de columna, no al
// revés: guardar la cadena de status dejaría la regla apuntando al vacío en cuanto
// alguien renombrara la columna.
func (s *WorkflowService) scopeMatches(wf models.Workflow, ev WorkflowEvent) bool {
	// Sin tablero no hay ámbito, y una regla sin ámbito podría alcanzar tableros
	// de cualquier parte del tenant. Se descarta: fail-closed.
	if wf.BoardID == 0 || wf.BoardID != ev.Task.BoardID {
		return false
	}

	var scope struct {
		PhaseID uint `json:"phase_id"`
	}
	if err := json.Unmarshal([]byte(nonEmptyJSON(wf.TriggerConfig)), &scope); err != nil {
		log.Printf("[workflow] regla %d con ámbito ilegible: %v", wf.ID, err)
		return false
	}
	if scope.PhaseID == 0 {
		return true
	}

	board, err := s.boardRepo.GetByID(wf.BoardID)
	if err != nil || board == nil {
		return false
	}
	for _, p := range board.Phases {
		if p.ID == scope.PhaseID {
			return PhaseColumnID(p) == string(ev.Task.Status)
		}
	}
	// La fase ya no existe en el tablero: la regla quedó huérfana y no aplica.
	return false
}

// inCauseChain indica si la regla ya aparece en la cadena que llevó hasta aquí.
func (s *WorkflowService) inCauseChain(workflowID uint, causeRunID *uint) bool {
	if causeRunID == nil {
		return false
	}
	ids, err := s.repo.CauseChainWorkflowIDs(*causeRunID)
	if err != nil {
		// Sin poder comprobar la cadena, lo seguro es NO encolar: un bucle cuesta
		// mucho más caro que una automatización que no se ejecutó una vez.
		log.Printf("[workflow] no se pudo verificar la cadena causal de la ejecución %d: %v", *causeRunID, err)
		return true
	}
	for _, id := range ids {
		if id == workflowID {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Worker
// ---------------------------------------------------------------------------

// Start lanza el bucle. Idempotente frente a reinicios: lo pendiente sobrevive en la
// base, y lo que quedó a medias lo rescata RequeueStale.
func (s *WorkflowService) Start() {
	// El vigilante del tiempo y la limpieza del historial van con el worker: son el
	// trabajo de fondo del motor y no tiene sentido tener uno sin los otros.
	s.StartSweep()
	s.StartPurge()
	go s.run()
	log.Println("[workflow] worker iniciado")
}

func (s *WorkflowService) run() {
	ticker := time.NewTicker(workflowTick)
	defer ticker.Stop()
	for {
		s.processBatch()
		select {
		case <-ticker.C:
		case <-s.nudge:
		}
	}
}

func (s *WorkflowService) signal() {
	select {
	case s.nudge <- struct{}{}:
	default: // ya hay un aviso pendiente; no hace falta otro.
	}
}

func (s *WorkflowService) processBatch() {
	if n, err := s.repo.RequeueStale(time.Now().Add(-workflowStaleAfter)); err != nil {
		log.Printf("[workflow] no se pudieron rescatar ejecuciones colgadas: %v", err)
	} else if n > 0 {
		log.Printf("[workflow] %d ejecuciones colgadas devueltas a la cola", n)
	}

	runs, err := s.repo.ClaimRuns(workflowBatch, time.Now())
	if err != nil {
		log.Printf("[workflow] no se pudieron tomar ejecuciones: %v", err)
		return
	}
	for i := range runs {
		s.processRun(&runs[i])
	}
}

func (s *WorkflowService) processRun(run *models.WorkflowRun) {
	wf, err := s.repo.GetWorkflow(run.WorkflowID)
	if err != nil || wf == nil {
		s.skip(run, "la regla ya no existe")
		return
	}

	// Se relee el estado de la regla en cada intento, no se confía en el que tenía
	// al encolarse: apagar una regla tiene que surtir efecto sobre lo que ya estaba
	// en cola, o "apagar" no significaría nada durante los siguientes minutos.
	if !wf.Enabled {
		s.skip(run, "la regla está apagada")
		return
	}

	var ctx WorkflowContext
	if err := json.Unmarshal([]byte(nonEmptyJSON(run.Context)), &ctx); err != nil {
		s.skip(run, "el contexto de la ejecución no se pudo interpretar")
		return
	}

	// Alcance del autor, comprobado AHORA y no al guardar la regla: quien la creó
	// puede haber perdido el acceso al tablero desde entonces, y una regla no debe
	// convertirse en una puerta trasera a un tablero que su autor ya no alcanza.
	if ok, why := s.authorStillInScope(wf); !ok {
		s.skip(run, why)
		return
	}

	if ok, why := evalConditions(wf.Conditions, conditionFields(ctx)); !ok {
		s.skip(run, why)
		return
	}

	done, err := s.repo.DoneStepIDs(run.ID)
	if err != nil {
		s.fail(run, err)
		return
	}

	// Las salidas de los pasos ya ejecutados se reponen en el contexto para que un
	// reintento pueda seguir resolviendo {{pasos.N.*}} igual que la primera vez.
	if ctx.Steps == nil {
		ctx.Steps = map[string]any{}
	}
	if prev, perr := s.repo.ListStepRuns(run.ID); perr == nil {
		for _, sr := range prev {
			if sr.Status != models.WorkflowStepDone || sr.Output == "" {
				continue
			}
			var out map[string]any
			if json.Unmarshal([]byte(sr.Output), &out) == nil {
				ctx.Steps[stepKey(sr.Order)] = out
				applyStepEffect(&ctx, out)
			}
		}
	}

	// Los campos comparables se calculan una vez: son los mismos para todos los
	// pasos de esta ejecución.
	fields := conditionFields(ctx)

	for _, step := range refreshedSteps(wf) {
		if done[step.ID] {
			continue // ya salió en un intento anterior; no se repite.
		}

		// Condiciones DEL PASO. Es lo que permite que una misma regla haga una cosa
		// u otra según lo respondido en la puerta. Saltarlo se registra igual que
		// cualquier otro salto: en la actividad tiene que verse que el paso existía
		// y por qué no le tocaba, no que no existía.
		if ok, _ := evalConditions(step.Conditions, fields); !ok {
			s.recordStep(&models.WorkflowStepRun{
				RunID: run.ID, StepID: step.ID, Order: step.Order,
				Status: models.WorkflowStepSkipped,
				Error:  "este paso no aplica a lo que se respondió",
			})
			continue
		}

		output, skipReason, aerr := s.runAction(step, run, ctx)
		if aerr != nil {
			s.recordStep(&models.WorkflowStepRun{
				RunID: run.ID, StepID: step.ID, Order: step.Order,
				Status: models.WorkflowStepFailed, Error: aerr.Error(),
			})
			s.fail(run, aerr)
			return
		}
		if skipReason != "" {
			// Un paso saltado no aborta la ejecución: los demás pueden tener
			// sentido igualmente (avisar al manager aunque no haya responsable).
			s.recordStep(&models.WorkflowStepRun{
				RunID: run.ID, StepID: step.ID, Order: step.Order,
				Status: models.WorkflowStepSkipped, Error: skipReason,
			})
			continue
		}

		s.recordStep(&models.WorkflowStepRun{
			RunID: run.ID, StepID: step.ID, Order: step.Order,
			Status: models.WorkflowStepDone, Output: mustJSON(output),
		})
		ctx.Steps[stepKey(step.Order)] = output
		applyStepEffect(&ctx, output)
	}

	if err := s.repo.MarkRunDone(run.ID); err != nil {
		log.Printf("[workflow] no se pudo cerrar la ejecución %d: %v", run.ID, err)
	}
}

// applyStepEffect anota en el snapshot lo que acaba de hacer ESTA ejecución.
//
// La comprobación de obsolescencia existe para no pisar el trabajo de una persona con
// una decisión caducada, y compara el snapshot con la tarea real. Sin esto, el primer
// paso que mueve la tarjeta dejaría "obsoletos" a todos los que van detrás: la regla
// que mueve y luego comenta acabaría moviendo sin comentar, que es exactamente el
// caso de una revisión con veredicto.
//
// Sólo se anota lo que la comprobación mira —estado y prioridad—, y sólo cuando el
// paso lo devolvió: el resto del snapshot sigue congelado, que es su razón de ser.
func applyStepEffect(ctx *WorkflowContext, out map[string]any) {
	if v, ok := out["estado"].(string); ok && v != "" {
		ctx.Task.EstadoAnterior = ctx.Task.Estado
		ctx.Task.Estado = v
	}
	if v, ok := out["prioridad"].(string); ok && v != "" {
		ctx.Task.PrioridadAnterior = ctx.Task.Prioridad
		ctx.Task.Prioridad = v
	}
}

// authorStillInScope aplica la regla de alcance de §4.3 del plan: el empleador
// alcanza todo su tenant; un manager o supervisor, sólo los tableros donde es
// miembro. Es más estricto que el módulo de Tareas, y a propósito: una regla puede
// avisar a mucha gente sin que nadie vuelva a mirarla.
func (s *WorkflowService) authorStillInScope(wf *models.Workflow) (bool, string) {
	author, err := s.userRepo.GetByID(wf.CreatedBy)
	if err != nil || author == nil {
		return false, "quien creó la regla ya no existe"
	}
	if !author.IsActive {
		return false, "quien creó la regla ya no está activo"
	}
	if author.IsSuperadmin {
		return true, ""
	}
	// La cuenta empleador ES el tenant: alcanza todos sus tableros.
	if author.ID == wf.TenantID {
		return true, ""
	}

	board, err := s.boardRepo.GetByID(wf.BoardID)
	if err != nil || board == nil {
		return false, "el tablero de la regla ya no existe"
	}
	if board.TenantID != wf.TenantID {
		return false, "el tablero ya no pertenece a la empresa de la regla"
	}
	if board.CreatedBy == author.ID {
		return true, ""
	}
	for _, m := range board.Members {
		if m.ID == author.ID {
			return true, ""
		}
	}
	return false, "quien creó la regla ya no es miembro del tablero"
}

func (s *WorkflowService) skip(run *models.WorkflowRun, reason string) {
	if err := s.repo.MarkRunSkipped(run.ID, reason); err != nil {
		log.Printf("[workflow] no se pudo marcar la ejecución %d como saltada: %v", run.ID, err)
	}
}

func (s *WorkflowService) fail(run *models.WorkflowRun, cause error) {
	// run.Attempts ya viene incrementado por ClaimRuns, así que refleja este intento.
	var retryAt *time.Time
	if run.Attempts < models.WorkflowMaxAttempts {
		at := time.Now().Add(models.WorkflowRetryDelay(run.Attempts))
		retryAt = &at
	}
	if err := s.repo.MarkRunFailed(run.ID, cause.Error(), retryAt); err != nil {
		log.Printf("[workflow] no se pudo marcar la ejecución %d como fallida: %v", run.ID, err)
	}
	if retryAt == nil {
		log.Printf("[workflow] ejecución %d (regla %d) agotada tras %d intentos: %v",
			run.ID, run.WorkflowID, run.Attempts, cause)
		return
	}
	log.Printf("[workflow] ejecución %d (regla %d) falló en el intento %d, reintento en %s: %v",
		run.ID, run.WorkflowID, run.Attempts, models.WorkflowRetryDelay(run.Attempts), cause)
}

// recordStep guarda el resultado de un paso. El fallo NO aborta la ejecución —la
// bitácora no puede tumbar el trabajo que documenta— pero SÍ se registra: descartarlo
// en silencio fue lo que permitió que un INSERT inválido pasara desapercibido y que
// los pasos saltados desaparecieran sin dejar rastro.
func (s *WorkflowService) recordStep(entry *models.WorkflowStepRun) {
	if err := s.repo.SaveStepRun(entry); err != nil {
		log.Printf("[workflow] no se pudo registrar el paso %d de la ejecución %d (%s): %v",
			entry.StepID, entry.RunID, entry.Status, err)
	}
}

// stepKey es la clave con la que la salida de un paso queda disponible para los
// siguientes como {{pasos.N.*}}. Se numera desde 1, como los ve el usuario.
func stepKey(order int) string {
	return strconv.Itoa(order + 1)
}
