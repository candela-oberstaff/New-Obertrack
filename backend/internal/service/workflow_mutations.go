package service

import (
	"fmt"
	"log"

	"github.com/obertrack/backend/internal/models"
)

// Acciones que MUTAN la tarea. Son las que convierten el motor en algo que hace, no
// sólo algo que avisa, y por eso llevan tres defensas que las de aviso no necesitan:
//
//  1. Escriben por ApplyAsSystem, el único camino con el privilegio de mutar sin ser
//     una persona. No se falsifica un usuario ni se reutiliza el del autor de la regla.
//  2. Comprueban antes que la tarea SIGA como estaba al dispararse. Entre el disparo
//     y la ejecución pueden pasar minutos si hubo reintentos, y aplicar sobre algo
//     que ya cambió es peor que no aplicar nada.
//  3. Arrastran la cadena causal, para que lo que provoquen cuente en el antibucle.

// TaskMutator es lo que el motor necesita del módulo de Tareas para mutar. Se inyecta
// como el resto de enganches; sin cablear, las acciones que mutan quedan saltadas con
// motivo en vez de fallar.
type TaskMutator interface {
	ApplyAsSystem(taskID uint, updates map[string]interface{}, assignees *[]uint, cause WorkflowCause) (*models.Task, error)
	AddComment(id uint, tenantID uint, userID uint, content string, isSuperadmin bool) (*models.Comment, error)
}

// SetTaskMutator cablea el módulo de Tareas para las acciones que escriben.
func (s *WorkflowService) SetTaskMutator(m TaskMutator) {
	s.mutator = m
}

// runMutation despacha las acciones que cambian la tarea.
func (s *WorkflowService) runMutation(step models.WorkflowStep, cfg stepConfig, run *models.WorkflowRun, ctx WorkflowContext) (map[string]any, string, error) {
	if s.mutator == nil {
		return nil, "las acciones que modifican tareas no están disponibles en esta instancia", nil
	}

	// Obsolescencia. El snapshot dice cómo estaba la tarea al dispararse; si desde
	// entonces alguien la movió o la cerró, la consecuencia calculada sobre aquel
	// estado ya no se sostiene. Se salta explicando por qué, que es información útil,
	// en vez de pisar el trabajo de una persona con una decisión caducada.
	if stale, why := s.taskIsStale(ctx); stale {
		return nil, why, nil
	}

	cause := WorkflowCause{
		RunID:      run.ID,
		WorkflowID: run.WorkflowID,
		Depth:      run.Depth,
	}

	switch step.ActionType {
	case models.ActionSetPriority:
		return s.mutatePriority(cfg, ctx, cause)
	case models.ActionSetStatus:
		return s.mutateStatus(cfg, ctx, cause)
	case models.ActionAssign:
		return s.mutateAssign(cfg, run, ctx, cause)
	case models.ActionComment:
		return s.mutateComment(cfg, ctx)
	}
	return nil, fmt.Sprintf("acción desconocida: %q", step.ActionType), nil
}

// taskIsStale compara el estado guardado en el snapshot con el actual.
//
// Sólo mira estado y prioridad: son lo que las acciones cambian y lo que las
// condiciones evalúan. Que hayan editado el título entre medias no invalida nada.
func (s *WorkflowService) taskIsStale(ctx WorkflowContext) (bool, string) {
	current, err := s.taskRepo.GetByID(ctx.Task.ID)
	if err != nil || current == nil {
		return true, "la tarea ya no existe"
	}
	if string(current.Status) != ctx.Task.Estado {
		return true, fmt.Sprintf("la tarea cambió de columna desde el disparo (ahora está en %q)", current.Status)
	}
	if string(current.Priority) != ctx.Task.Prioridad {
		return true, fmt.Sprintf("la prioridad de la tarea cambió desde el disparo (ahora es %q)", current.Priority)
	}
	return false, ""
}

func (s *WorkflowService) mutatePriority(cfg stepConfig, ctx WorkflowContext, cause WorkflowCause) (map[string]any, string, error) {
	nueva := interpolate(cfg.Priority, ctx)
	if !isValidPriority(nueva) {
		return nil, fmt.Sprintf("la prioridad configurada no es válida: %q", nueva), nil
	}
	// Ya la tiene: no se escribe. Además de ahorrar la escritura, evita emitir un
	// evento de "cambió la prioridad" que no cambió nada y que podría realimentar
	// otra regla.
	if nueva == ctx.Task.Prioridad {
		return nil, fmt.Sprintf("la tarea ya tenía prioridad %q", nueva), nil
	}

	if _, err := s.mutator.ApplyAsSystem(ctx.Task.ID, map[string]interface{}{"priority": nueva}, nil, cause); err != nil {
		return nil, "", err
	}
	return map[string]any{"prioridad": nueva, "prioridad_anterior": ctx.Task.Prioridad}, "", nil
}

// Columnas por su PAPEL en el tablero, no por su nombre. Una receta no puede llevar
// escrito "finalizado": cada tablero nombra sus columnas como quiere, y una regla que
// mueve tarjetas a una columna inexistente es peor que una regla que no hace nada.
const (
	statusBoardStart = "@inicial"
	statusBoardEnd   = "@final"
)

func (s *WorkflowService) mutateStatus(cfg stepConfig, ctx WorkflowContext, cause WorkflowCause) (map[string]any, string, error) {
	destino := interpolate(cfg.Status, ctx)
	switch destino {
	case statusBoardStart:
		destino = ctx.Board.ColumnaInicial
		if destino == "" {
			return nil, "el tablero no tiene una columna de entrada reconocible", nil
		}
	case statusBoardEnd:
		destino = ctx.Board.ColumnaFinal
		if destino == "" {
			return nil, "el tablero no tiene una columna de cierre: añade una o mueve la tarjeta a mano", nil
		}
	}
	if destino == "" {
		return nil, "la acción no dice a qué columna mover", nil
	}
	if destino == ctx.Task.Estado {
		return nil, fmt.Sprintf("no hacía falta moverla: ya estaba en %q", destino), nil
	}

	if _, err := s.mutator.ApplyAsSystem(ctx.Task.ID, map[string]interface{}{"status": destino}, nil, cause); err != nil {
		// Una puerta en la columna destino devuelve GateRequiredError. No es un
		// fallo que merezca reintentos: reintentar no va a rellenar un formulario.
		if gate, ok := err.(*GateRequiredError); ok {
			// Y se le dice a quien lo provocó. Si no, la tarjeta simplemente no se
			// mueve: quien acaba de aprobar una revisión ve que no pasa nada y da por
			// rota la automatización, cuando lo que ocurre es que falta un formulario
			// que sólo una persona puede rellenar.
			s.avisarPuertaPendiente(ctx, destino, gate.Workflow)
			return nil, fmt.Sprintf("la columna %q exige un formulario (%s) y una automatización no puede rellenarlo", destino, gate.Workflow), nil
		}
		return nil, "", err
	}
	return map[string]any{"estado": destino, "estado_anterior": ctx.Task.Estado}, "", nil
}

func (s *WorkflowService) mutateAssign(cfg stepConfig, run *models.WorkflowRun, ctx WorkflowContext, cause WorkflowCause) (map[string]any, string, error) {
	// Reutiliza la resolución de destinatarios: "asignar al manager del responsable"
	// se expresa igual que "avisar al manager del responsable".
	nuevos, why := s.resolveRecipients(cfg, run, ctx)
	if len(nuevos) == 0 {
		if why == "" {
			why = "no hay a quién asignar"
		}
		return nil, why, nil
	}

	// Se SUMAN a los que ya estaban. Reemplazarlos silenciosamente sacaría de la
	// tarea a gente que sí está trabajando en ella.
	final := append([]uint{}, ctx.Task.AsignadosIDs...)
	yaEstaban := make(map[uint]bool, len(final))
	for _, id := range final {
		yaEstaban[id] = true
	}
	añadidos := []uint{}
	for _, id := range nuevos {
		if !yaEstaban[id] {
			final = append(final, id)
			añadidos = append(añadidos, id)
		}
	}
	if len(añadidos) == 0 {
		return nil, "quien correspondía ya estaba asignado", nil
	}

	if _, err := s.mutator.ApplyAsSystem(ctx.Task.ID, map[string]interface{}{}, &final, cause); err != nil {
		return nil, "", err
	}
	return map[string]any{"asignados": añadidos}, "", nil
}

func (s *WorkflowService) mutateComment(cfg stepConfig, ctx WorkflowContext) (map[string]any, string, error) {
	texto := interpolate(cfg.Content, ctx)
	if texto == "" {
		return nil, "la acción no tiene texto que comentar", nil
	}

	// El comentario se firma con el bot: es un mensaje del sistema y atribuirlo a una
	// persona haría creer que lo escribió alguien.
	bot, err := s.userRepo.GetByEmail(models.SystemBotEmail)
	if err != nil || bot == nil {
		return nil, "no existe la cuenta de sistema con la que firmar el comentario", nil
	}

	comment, err := s.mutator.AddComment(ctx.Task.ID, 0, bot.ID, texto, true)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{"comentario_id": comment.ID}, "", nil
}

func isValidPriority(p string) bool {
	switch models.TaskPriority(p) {
	case models.PriorityLow, models.PriorityMedium, models.PriorityHigh, models.PriorityUrgent:
		return true
	}
	return false
}

// avisarPuertaPendiente le cuenta a quien provocó la ejecución que la tarjeta se
// quedó a las puertas de la columna destino, y por qué.
//
// Sin esto, el motor "no hacer nada" y "no poder hacerlo" se ven igual desde fuera. El
// aviso lleva el enlace a la tarjeta para que mover a mano —y rellenar el formulario
// que falta— sea el siguiente clic y no una búsqueda.
func (s *WorkflowService) avisarPuertaPendiente(ctx WorkflowContext, destino, puerta string) {
	// Sin persona detrás no hay a quién avisar: lo provocó el calendario o el propio
	// motor, y ahí el registro de la regla es el sitio correcto.
	if s.notifSvc == nil || ctx.Actor.ID == 0 || ctx.Actor.EsSistema {
		return
	}
	if err := s.notifSvc.CreateNotification(
		ctx.Actor.ID,
		"workflow",
		"La tarjeta no pasó de columna",
		fmt.Sprintf(`"%s" no pudo pasar a %s: esa columna pide el formulario de %q. Muévela tú para completarlo.`,
			ctx.Task.Titulo, destino, puerta),
		map[string]interface{}{
			"task_id":  ctx.Task.ID,
			"board_id": ctx.Board.ID,
			"link":     taskDeepLink(ctx.Task.ID, ctx.Board.ID, ctx.Empresa),
		},
	); err != nil {
		log.Printf("[workflow] no se pudo avisar de la puerta pendiente en la tarea %d: %v", ctx.Task.ID, err)
	}
}
