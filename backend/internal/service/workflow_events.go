package service

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// WorkflowEvent es lo que los servicios de dominio le cuentan al motor cuando algo
// pasa. Lo emite taskService por callback inyectado (mismo patrón que SetCalendarSync)
// para no acoplar taskService→workflowService ni crear un ciclo de imports.
//
// Lleva la tarea YA RECARGADA, con asignados y tablero, porque el emisor la tiene a
// mano en ese punto y volver a leerla desde el motor abriría una ventana en la que
// el snapshot no coincide con el cambio que lo provocó.
type WorkflowEvent struct {
	Type     string
	TenantID uint
	Task     *models.Task

	// PrevStatus / PrevPriority son los valores ANTERIORES al cambio. Vacíos
	// cuando el disparador no los usa (p. ej. task.created).
	PrevStatus   string
	PrevPriority string
	// NewAssignees son los ids que acaban de sumarse a la tarea. Una edición puede
	// añadir a varias personas de una vez, y eso es UN evento, no varios.
	NewAssignees []uint

	// ActorID es quien provocó el cambio. 0 = no fue una persona.
	ActorID uint
	// ActorIsSystem marca los cambios hechos por el propio motor o por un worker.
	// Existe aparte de ActorID porque una regla suele querer decir "cuando lo
	// mueva alguien", no "cuando lo mueva otra automatización".
	ActorIsSystem bool

	// GateWorkflowID y GateAnswers vienen rellenos sólo cuando el movimiento cruzó
	// una PUERTA. Son lo que convierte una puerta en algo más que un peaje: la regla
	// que la puso recibe su propia ejecución con lo que la persona respondió, y sus
	// pasos deciden a partir de ahí. Sin el id, todas las puertas del tablero
	// reaccionarían al formulario de una sola de ellas.
	GateWorkflowID uint
	GateAnswers    map[string]any

	// CauseRunID y Depth vienen rellenos sólo cuando el cambio lo provocó una
	// acción del propio motor. En la Fase 1 ninguna acción muta la tarea, así que
	// llegan vacíos siempre; el antibucle se construye igualmente ahora porque
	// añadirlo después obligaría a migrar ejecuciones ya escritas.
	CauseRunID *uint
	Depth      int
}

// WorkflowContext es el snapshot que se guarda en workflow_runs.context. Es la
// ÚNICA fuente contra la que se evalúan condiciones y se resuelven variables: no se
// relee la base al ejecutar, para que un reintento tres horas después produzca
// exactamente el mismo resultado que el primer intento.
type WorkflowContext struct {
	Trigger string `json:"trigger"`
	// Empresa es el inquilino de la tarea. Va en el snapshot porque el enlace de los
	// avisos la necesita: quien lo recibe puede ser un superadmin, que tiene que
	// cambiar de empresa antes de poder ver el tablero.
	Empresa uint             `json:"empresa,omitempty"`
	Task    workflowTaskCtx  `json:"tarea"`
	Board   workflowBoardCtx `json:"tablero"`
	Actor   workflowActorCtx `json:"actor"`
	Steps   map[string]any   `json:"pasos,omitempty"`
	Extra   map[string]any   `json:"extra,omitempty"`
	// Respuestas es lo que una persona rellenó en la puerta. Va en el snapshot y no
	// se relee: la consecuencia de una revisión tiene que ser la de aquella
	// respuesta, aunque tres horas después alguien haya cambiado la tarea.
	Respuestas map[string]any `json:"respuestas,omitempty"`
}

type workflowTaskCtx struct {
	ID                uint   `json:"id"`
	Titulo            string `json:"titulo"`
	Estado            string `json:"estado"`
	EstadoAnterior    string `json:"estado_anterior"`
	Prioridad         string `json:"prioridad"`
	PrioridadAnterior string `json:"prioridad_anterior"`
	FechaFin          string `json:"fecha_fin"`
	Completada        bool   `json:"completada"`
	AsignadosIDs      []uint `json:"asignados_ids"`
	Asignados         string `json:"asignados"`
	NuevosAsignados   []uint `json:"nuevos_asignados,omitempty"`
	Enlace            string `json:"enlace"`
}

type workflowBoardCtx struct {
	ID        uint   `json:"id"`
	Nombre    string `json:"nombre"`
	CreadorID uint   `json:"creador_id"`
	// Las columnas de entrada y de cierre del tablero, resueltas al construir el
	// snapshot. Una receta no puede llevar escrito "finalizado": cada tablero nombra
	// sus columnas como quiere. Y resolverlas AHORA, y no al ejecutar, mantiene la
	// promesa del snapshot: un reintento mueve la tarjeta a donde se decidió
	// entonces, aunque entre medias hayan renombrado la columna.
	ColumnaInicial string `json:"columna_inicial,omitempty"`
	ColumnaFinal   string `json:"columna_final,omitempty"`
}

// boardColumns resuelve las dos columnas con significado de un tablero: por dónde se
// empieza y dónde se da algo por terminado.
//
// Se busca primero por identificador canónico ("por_hacer", "finalizado"), que es lo
// que traen los tableros por defecto; si el tablero renombró sus columnas, se cae a
// la primera y la última, que es la lectura natural de un kanban. Un tablero de una
// sola columna no tiene "final" distinto del inicio: se devuelve vacío y la acción
// que dependa de ello se saltará explicándolo, en vez de mover la tarjeta a donde no
// debe.
func boardColumns(b models.Board) (inicial, final string) {
	if len(b.Phases) == 0 {
		return "", ""
	}
	for _, p := range b.Phases {
		switch PhaseColumnID(p) {
		case "por_hacer":
			inicial = PhaseColumnID(p)
		case "finalizado":
			final = PhaseColumnID(p)
		}
	}
	if inicial == "" {
		inicial = PhaseColumnID(b.Phases[0])
	}
	if final == "" && len(b.Phases) > 1 {
		final = PhaseColumnID(b.Phases[len(b.Phases)-1])
	}
	if final == inicial {
		final = ""
	}
	return inicial, final
}

type workflowActorCtx struct {
	ID        uint   `json:"id"`
	Nombre    string `json:"nombre"`
	EsSistema bool   `json:"es_sistema"`
}

// buildContext arma el snapshot a partir del evento. actorName llega resuelto desde
// fuera para no meter una consulta de usuario dentro del request por cada regla
// candidata: se resuelve una vez por evento.
func buildContext(ev WorkflowEvent, actorName string) WorkflowContext {
	ctx := WorkflowContext{
		Trigger: ev.Type,
		Empresa: ev.TenantID,
		Actor: workflowActorCtx{
			ID:        ev.ActorID,
			Nombre:    actorName,
			EsSistema: ev.ActorIsSystem,
		},
	}
	if ev.Task == nil {
		return ctx
	}

	names := make([]string, 0, len(ev.Task.Assignees))
	ids := make([]uint, 0, len(ev.Task.Assignees))
	for _, a := range ev.Task.Assignees {
		ids = append(ids, a.ID)
		names = append(names, a.Name)
	}

	due := ""
	if ev.Task.EndDate != nil {
		due = ev.Task.EndDate.Format("02/01/2006")
	}

	ctx.Task = workflowTaskCtx{
		ID:                ev.Task.ID,
		Titulo:            ev.Task.Title,
		Estado:            string(ev.Task.Status),
		EstadoAnterior:    ev.PrevStatus,
		Prioridad:         string(ev.Task.Priority),
		PrioridadAnterior: ev.PrevPriority,
		FechaFin:          due,
		Completada:        ev.Task.Completed,
		AsignadosIDs:      ids,
		Asignados:         strings.Join(names, ", "),
		NuevosAsignados:   ev.NewAssignees,
		// El enlace es relativo a propósito: la base del frontend depende del
		// despliegue y no tiene por qué quedar congelada dentro de un snapshot.
		Enlace: taskDeepLink(ev.Task.ID, ev.Task.BoardID, ev.TenantID),
	}
	inicial, final := boardColumns(ev.Task.Board)
	ctx.Board = workflowBoardCtx{
		ID:             ev.Task.BoardID,
		Nombre:         ev.Task.Board.Name,
		CreadorID:      ev.Task.Board.CreatedBy,
		ColumnaInicial: inicial,
		ColumnaFinal:   final,
	}
	ctx.Respuestas = ev.GateAnswers
	return ctx
}

// conditionFields aplana el contexto a los campos comparables por una condición.
// Se construye aparte del snapshot porque son cosas distintas: el snapshot guarda
// lo ocurrido, esto expone lo que una regla puede preguntar.
func conditionFields(ctx WorkflowContext) map[string]any {
	overdue := false
	if ctx.Task.FechaFin != "" && !ctx.Task.Completada {
		// La fecha ya viene formateada dd/mm/aaaa en el snapshot; recomponerla
		// aquí evita arrastrar el time.Time y sus problemas de zona al JSON.
		var d, m, y int
		if _, err := fmt.Sscanf(ctx.Task.FechaFin, "%d/%d/%d", &d, &m, &y); err == nil {
			overdue = isBeforeToday(y, m, d)
		}
	}
	fields := map[string]any{
		"tablero":            float64(ctx.Board.ID),
		"estado":             ctx.Task.Estado,
		"estado_anterior":    ctx.Task.EstadoAnterior,
		"prioridad":          ctx.Task.Prioridad,
		"prioridad_anterior": ctx.Task.PrioridadAnterior,
		"tiene_responsable":  len(ctx.Task.AsignadosIDs) > 0,
		"tiene_fecha_fin":    ctx.Task.FechaFin != "",
		"esta_vencida":       overdue,
		"completada":         ctx.Task.Completada,
		"actor_es_sistema":   ctx.Actor.EsSistema,
	}
	// Lo respondido en la puerta se expone como "respuesta.<campo>". Es lo que un
	// paso pregunta para saber si le toca: respuesta.veredicto == "aprobado".
	for k, v := range ctx.Respuestas {
		fields["respuesta."+k] = v
	}
	return fields
}

// dedupKey identifica el CAMBIO, no el evento: dos disparadores distintos sobre la
// misma revisión (una edición que toca estado y prioridad a la vez) dan claves
// distintas, que es el comportamiento correcto. Dos intentos de encolar el mismo
// disparador sobre la misma revisión dan la misma clave y el índice único descarta
// el segundo.
func dedupKey(triggerType, entityType string, entityID uint, discriminante string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:%s:%d:%s", triggerType, entityType, entityID, discriminante)))
	return hex.EncodeToString(sum[:])
}

// dedupKeyFor elige QUÉ hace único a este evento.
//
// Para lo que provoca una persona, la revisión de la tarea: cada cambio es un hecho
// nuevo. Para lo que provoca el calendario, la FECHA DE FIN: una tarea vencida no
// vuelve a vencer porque alguien le corrija el título, y sin esto cada edición habría
// reabierto el aviso. Mover la fecha sí abre uno nuevo, que es lo correcto — es otro
// vencimiento.
func dedupKeyFor(ev WorkflowEvent) string {
	switch ev.Type {
	case models.TriggerTaskOverdue, models.TriggerTaskDueSoon:
		vence := "sin-fecha"
		if ev.Task.EndDate != nil {
			vence = ev.Task.EndDate.Format("2006-01-02")
		}
		return dedupKey(ev.Type, "task", ev.Task.ID, vence)
	default:
		return dedupKey(ev.Type, "task", ev.Task.ID, strconv.Itoa(ev.Task.Revision))
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// runQuota acota cuántas ejecuciones encola una empresa por hora.
//
// El antibucle impide que las reglas se llamen entre sí; esto es para el otro lado: una
// persona que importa doscientas tareas o arrastra media columna dispara un aviso por
// cada una, y nadie quiere doscientos correos. El cupo corta ahí.
//
// Vive en memoria a propósito. Contar en la base metería una consulta por evento en el
// camino caliente —dentro del request de quien mueve una tarjeta— para defenderse de
// algo que pasa una vez cada mucho. El precio es que un reinicio limpia el contador,
// que para un tope horario es un precio bajo.
type runQuota struct {
	mu     sync.Mutex
	limit  int
	window time.Time
	used   map[uint]int
	// avisado evita repetir el mismo aviso en el log una vez por evento descartado:
	// una avalancha llenaría el log de líneas idénticas justo cuando hay que leerlo.
	avisado map[uint]bool
}

func newRunQuota(limit int) *runQuota {
	return &runQuota{limit: limit, used: map[uint]int{}, avisado: map[uint]bool{}}
}

// allow descuenta una ejecución del cupo de la empresa. Devuelve false cuando ya no
// queda, y true —siempre— si no hay límite configurado.
func (q *runQuota) allow(now time.Time, tenantID uint) bool {
	if q == nil || q.limit <= 0 {
		return true
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	hora := now.Truncate(time.Hour)
	if hora.After(q.window) {
		q.window = hora
		q.used = map[uint]int{}
		q.avisado = map[uint]bool{}
	}
	if q.used[tenantID] >= q.limit {
		if !q.avisado[tenantID] {
			q.avisado[tenantID] = true
			log.Printf("[workflow] la empresa %d alcanzó el tope de %d ejecuciones en una hora: "+
				"las siguientes se descartan hasta la hora siguiente", tenantID, q.limit)
		}
		return false
	}
	q.used[tenantID]++
	return true
}

// WorkflowCause acompaña a todo cambio hecho POR el motor. Sin ella, un cambio
// automático es indistinguible de uno humano y la cadena causal se rompe: los
// eventos que provoque nacerían con profundidad 0 y el antibucle dejaría de contar.
type WorkflowCause struct {
	// RunID es la ejecución que provocó el cambio.
	RunID uint
	// WorkflowID es la regla a la que pertenece.
	WorkflowID uint
	// Depth es la profundidad de la ejecución que lo provoca; lo que ella emita
	// nacerá con Depth+1.
	Depth int
}
