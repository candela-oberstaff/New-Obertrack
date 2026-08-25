package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Lo que fijan estas pruebas es el contrato del motor que NO se puede romper sin
// que alguien reciba avisos de más, de menos o de otra empresa: idempotencia,
// ámbito, antibucle, evaluación de condiciones y resolución de destinatarios.

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type wfRepo struct {
	repository.WorkflowRepository
	rules []models.Workflow
	// queued guarda lo encolado y hace de índice único: un (workflow_id, dedup_key)
	// repetido devuelve ErrRunAlreadyQueued, igual que la restricción real.
	queued []models.WorkflowRun
	chain  []uint
}

func (r *wfRepo) ListEnabledByTrigger(tenantID uint, trigger string) ([]models.Workflow, error) {
	out := []models.Workflow{}
	for _, wf := range r.rules {
		if wf.TenantID == tenantID && wf.TriggerType == trigger && wf.Enabled {
			out = append(out, wf)
		}
	}
	return out, nil
}

func (r *wfRepo) EnqueueRun(run *models.WorkflowRun) error {
	for _, existing := range r.queued {
		if existing.WorkflowID == run.WorkflowID && existing.DedupKey == run.DedupKey {
			return repository.ErrRunAlreadyQueued
		}
	}
	run.ID = uint(len(r.queued) + 1)
	r.queued = append(r.queued, *run)
	return nil
}

func (r *wfRepo) CauseChainWorkflowIDs(_ uint) ([]uint, error) { return r.chain, nil }

// UpdateGate y DeleteWorkflow son las escrituras del constructor de puertas.
func (r *wfRepo) UpdateGate(id uint, name, triggerConfig, formSchema string, enabled bool) error {
	for i := range r.rules {
		if r.rules[i].ID == id {
			r.rules[i].Name = name
			r.rules[i].TriggerConfig = triggerConfig
			r.rules[i].FormSchema = formSchema
			r.rules[i].Enabled = enabled
			return nil
		}
	}
	return nil
}

func (r *wfRepo) DeleteWorkflow(id uint) error {
	for i := range r.rules {
		if r.rules[i].ID == id {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			return nil
		}
	}
	return nil
}

// TenantsWithTrigger es lo que consulta el barrido del tiempo antes de mirar tarea
// alguna: las empresas que tienen algo encendido para ese disparador.
func (r *wfRepo) TenantsWithTrigger(trigger string) ([]uint, error) {
	vistas := map[uint]bool{}
	out := []uint{}
	for _, wf := range r.rules {
		if wf.TriggerType == trigger && wf.Enabled && !vistas[wf.TenantID] {
			vistas[wf.TenantID] = true
			out = append(out, wf.TenantID)
		}
	}
	return out, nil
}

func (r *wfRepo) ListByBoard(tenantID, boardID uint) ([]models.Workflow, error) {
	out := []models.Workflow{}
	for _, wf := range r.rules {
		if wf.TenantID == tenantID && wf.BoardID == boardID {
			out = append(out, wf)
		}
	}
	return out, nil
}

func (r *wfRepo) FindByRecipe(tenantID, boardID uint, key string) (*models.Workflow, error) {
	for i := range r.rules {
		wf := &r.rules[i]
		if wf.TenantID == tenantID && wf.BoardID == boardID && wf.RecipeKey == key {
			return wf, nil
		}
	}
	return nil, nil
}

func (r *wfRepo) CreateWorkflow(wf *models.Workflow) error {
	wf.ID = uint(len(r.rules) + 100)
	r.rules = append(r.rules, *wf)
	return nil
}

func (r *wfRepo) GetWorkflow(id uint) (*models.Workflow, error) {
	for i := range r.rules {
		if r.rules[i].ID == id {
			wf := r.rules[i]
			return &wf, nil
		}
	}
	return nil, nil
}

func (r *wfRepo) SetTriggerConfig(id uint, cfg string) error {
	for i := range r.rules {
		if r.rules[i].ID == id {
			r.rules[i].TriggerConfig = cfg
			return nil
		}
	}
	return nil
}

func (r *wfRepo) SetEnabled(id uint, enabled bool) error {
	for i := range r.rules {
		if r.rules[i].ID == id {
			r.rules[i].Enabled = enabled
			return nil
		}
	}
	return nil
}

type wfUserRepo struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (r *wfUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, gormNotFound
}

func (r *wfUserRepo) GetByEmail(email string) (*models.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, gormNotFound
}

type wfBoardRepo struct {
	repository.BoardRepository
	board *models.Board
}

func (r *wfBoardRepo) GetByID(_ uint) (*models.Board, error) { return r.board, nil }

type wfEmpRepo struct {
	repository.EmploymentRepository
	// managerOf[userID] = manager por el puntero de employments.
	managerOf map[uint]uint
}

func (r *wfEmpRepo) GetActive(userID, _ uint) (*models.Employment, error) {
	if mid, ok := r.managerOf[userID]; ok {
		m := mid
		return &models.Employment{UserID: userID, ManagerID: &m}, nil
	}
	return nil, gormNotFound
}

func (r *wfEmpRepo) ListManagerIDs(userID, _ uint) ([]uint, error) {
	if mid, ok := r.managerOf[userID]; ok {
		return []uint{mid}, nil
	}
	return nil, nil
}

// Reutiliza el error "no encontrado" que ya declara tutorial_lifecycle_test.go: en
// el paquete de pruebas basta con uno.
var gormNotFound = errFakeNotFound

func activeUser(id uint, name string) *models.User {
	return &models.User{ID: id, Name: name, IsActive: true}
}

func newWfService(repo *wfRepo, board *models.Board, users map[uint]*models.User, managers map[uint]uint) *WorkflowService {
	return NewWorkflowService(
		repo, nil,
		&wfBoardRepo{board: board},
		&wfUserRepo{users: users},
		&wfEmpRepo{managerOf: managers},
		&fakeNotifSvc{}, nil,
	)
}

// taskFor devuelve una tarea del tablero 1 lista para emitir eventos sobre ella.
func taskFor(revision int, status models.TaskStatus, assignees ...models.User) *models.Task {
	return &models.Task{
		ID: 100, Title: "Revisar informe", BoardID: 1, TenantID: 42,
		Status: status, Priority: models.PriorityMedium,
		Revision:  revision,
		Assignees: assignees,
		Board:     models.Board{ID: 1, Name: "Proyecto", CreatedBy: 9, TenantID: 42},
	}
}

// rule construye una regla activa del tenant 42 acotada a un tablero. boardID 0
// representa una regla sin ámbito, que es un caso que el motor debe rechazar.
func rule(id uint, trigger string, boardID uint, conditions string) models.Workflow {
	return models.Workflow{
		ID: id, TenantID: 42, Enabled: true, TriggerType: trigger,
		BoardID: boardID, TriggerConfig: "{}", Conditions: conditions, CreatedBy: 42,
	}
}

// ---------------------------------------------------------------------------
// Emisor: idempotencia y ámbito
// ---------------------------------------------------------------------------

func TestOnEvent_EncolaUnaEjecucionPorReglaCandidata(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{
		rule(1, models.TriggerTaskStatusChanged, 1, ""),
		rule(2, models.TriggerTaskStatusChanged, 1, ""),
		// Otro disparador: no debe encolarse.
		rule(3, models.TriggerTaskAssigned, 1, ""),
	}}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{
		Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer", ActorID: 7,
	})

	if len(repo.queued) != 2 {
		t.Fatalf("se esperaban 2 ejecuciones (una por regla del disparador), hay %d", len(repo.queued))
	}
}

func TestOnEvent_ElMismoCambioNoSeEncolaDosVeces(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{rule(1, models.TriggerTaskStatusChanged, 1, "")}}
	s := newWfService(repo, nil, nil, nil)

	ev := WorkflowEvent{
		Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer", ActorID: 7,
	}
	// Dos entregas del MISMO cambio: un reintento del emisor, o el mismo guardado
	// llegando dos veces. La revisión es la misma, así que es un solo hecho.
	s.OnEvent(ev)
	s.OnEvent(ev)

	if len(repo.queued) != 1 {
		t.Fatalf("el mismo cambio debe producir UNA ejecución, hay %d", len(repo.queued))
	}
}

func TestOnEvent_UnCambioPosteriorSiSeEncola(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{rule(1, models.TriggerTaskStatusChanged, 1, "")}}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer"})
	// Otra revisión = otro cambio real: mover, devolver y volver a mover tiene que
	// disparar la regla las tres veces.
	s.OnEvent(WorkflowEvent{Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(6, models.TaskStatusDone), PrevStatus: "en_proceso"})

	if len(repo.queued) != 2 {
		t.Fatalf("dos cambios distintos deben producir dos ejecuciones, hay %d", len(repo.queued))
	}
}

func TestOnEvent_UnaReglaDeOtroTableroNoSeEncola(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{rule(1, models.TriggerTaskStatusChanged, 99, "")}}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer"})

	if len(repo.queued) != 0 {
		t.Fatalf("una regla de otro tablero no debe alcanzar esta tarea, se encolaron %d", len(repo.queued))
	}
}

// Una regla sin board_id podría alcanzar cualquier tablero del tenant. Se descarta
// en vez de interpretarse como "todos": fail-closed.
func TestOnEvent_UnaReglaSinAmbitoSeDescarta(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{rule(1, models.TriggerTaskStatusChanged, 0, "")}}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer"})

	if len(repo.queued) != 0 {
		t.Fatalf("una regla sin tablero no debe ejecutarse, se encolaron %d", len(repo.queued))
	}
}

// Aislamiento por tenant: el emisor sólo pregunta por las reglas de la empresa del
// evento, así que una regla de la empresa 77 no puede verse afectada por un cambio
// en la 42 aunque el disparador coincida.
func TestOnEvent_NoAlcanzaReglasDeOtroTenant(t *testing.T) {
	otra := rule(1, models.TriggerTaskStatusChanged, 1, "")
	otra.TenantID = 77
	repo := &wfRepo{rules: []models.Workflow{otra}}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer"})

	if len(repo.queued) != 0 {
		t.Fatalf("una regla del tenant 77 no puede dispararse con un cambio del 42, se encolaron %d", len(repo.queued))
	}
}

// ---------------------------------------------------------------------------
// Antibucle
// ---------------------------------------------------------------------------

func TestOnEvent_CortaLaCadenaAlLlegarAlTope(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{rule(1, models.TriggerTaskStatusChanged, 1, "")}}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{
		Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer",
		Depth: models.MaxWorkflowDepth,
	})

	if len(repo.queued) != 0 {
		t.Fatalf("al tope de profundidad no debe encolarse nada, se encolaron %d", len(repo.queued))
	}
}

func TestOnEvent_UnaReglaNoSeDisparaASiMisma(t *testing.T) {
	cause := uint(1)
	repo := &wfRepo{
		rules: []models.Workflow{rule(7, models.TriggerTaskStatusChanged, 1, "")},
		// La cadena que llevó hasta aquí ya pasó por la regla 7.
		chain: []uint{7},
	}
	s := newWfService(repo, nil, nil, nil)

	s.OnEvent(WorkflowEvent{
		Type: models.TriggerTaskStatusChanged, TenantID: 42,
		Task: taskFor(5, models.TaskStatusInProcess), PrevStatus: "por_hacer",
		CauseRunID: &cause, Depth: 1,
	})

	if len(repo.queued) != 0 {
		t.Fatalf("una regla ya presente en la cadena causal no debe reencolarse, se encolaron %d", len(repo.queued))
	}
}

// ---------------------------------------------------------------------------
// Condiciones
// ---------------------------------------------------------------------------

func TestCondiciones(t *testing.T) {
	fields := map[string]any{
		"estado":            "en_proceso",
		"prioridad":         "urgent",
		"tiene_responsable": false,
		"tablero":           float64(1),
	}

	casos := []struct {
		nombre string
		json   string
		quiero bool
	}{
		{"árbol vacío aplica siempre", `{}`, true},
		{"all con todo cierto", `{"all":[{"field":"estado","op":"eq","value":"en_proceso"},{"field":"tiene_responsable","op":"eq","value":false}]}`, true},
		{"all con una falsa", `{"all":[{"field":"estado","op":"eq","value":"en_proceso"},{"field":"prioridad","op":"eq","value":"low"}]}`, false},
		{"any con una cierta", `{"any":[{"field":"prioridad","op":"eq","value":"low"},{"field":"prioridad","op":"eq","value":"urgent"}]}`, true},
		{"in", `{"all":[{"field":"prioridad","op":"in","value":["high","urgent"]}]}`, true},
		{"nin", `{"all":[{"field":"prioridad","op":"nin","value":["high","urgent"]}]}`, false},
		{"neq", `{"all":[{"field":"estado","op":"neq","value":"finalizado"}]}`, true},
		// El id del tablero puede venir como número o como cadena según quién
		// escribiera la regla; ninguna de las dos formas debe sorprender.
		{"número contra cadena", `{"all":[{"field":"tablero","op":"eq","value":"1"}]}`, true},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got, why := evalConditions(c.json, fields)
			if got != c.quiero {
				t.Fatalf("se esperaba %v y salió %v (motivo: %q)", c.quiero, got, why)
			}
			if !got && why == "" {
				t.Fatal("una condición que no se cumple debe explicar por qué")
			}
		})
	}
}

// Fail-closed: unas condiciones ilegibles NO equivalen a "sin condiciones". Al
// revés, una regla mal guardada empezaría a avisar a gente que su autor excluyó.
func TestCondiciones_IlegiblesNoAplican(t *testing.T) {
	ok, why := evalConditions(`{"all": [`, map[string]any{})
	if ok {
		t.Fatal("unas condiciones rotas no deben dar por buena la regla")
	}
	if !strings.Contains(why, "no se pudieron interpretar") {
		t.Fatalf("el motivo debería señalar el problema, got %q", why)
	}
}

// Un campo que el motor no expone tampoco se da por bueno.
func TestCondiciones_CampoDesconocidoNoAplica(t *testing.T) {
	ok, _ := evalConditions(`{"all":[{"field":"color_favorito","op":"eq","value":"azul"}]}`, map[string]any{})
	if ok {
		t.Fatal("preguntar por un campo inexistente no puede evaluar a verdadero")
	}
}

// ---------------------------------------------------------------------------
// Variables
// ---------------------------------------------------------------------------

func TestInterpolacion(t *testing.T) {
	ctx := buildContext(WorkflowEvent{
		Type: models.TriggerTaskStatusChanged,
		Task: taskFor(5, models.TaskStatusInProcess,
			models.User{ID: 7, Name: "Ana"}, models.User{ID: 8, Name: "Luis"}),
		PrevStatus: "por_hacer",
	}, "Carlos")

	got := interpolate("{{tarea.titulo}} pasó de {{tarea.estado_anterior}} a {{tarea.estado}} · {{tarea.asignados}} · {{actor.nombre}} · #{{tarea.id}}", ctx)
	quiero := "Revisar informe pasó de por_hacer a en_proceso · Ana, Luis · Carlos · #100"
	if got != quiero {
		t.Fatalf("interpolación incorrecta:\n  got:    %q\n  quiero: %q", got, quiero)
	}
}

// Una variable mal escrita se deja visible en vez de desaparecer: así el error se
// ve y se puede corregir, en lugar de parecer un fallo del sistema.
func TestInterpolacion_VariableDesconocidaQuedaVisible(t *testing.T) {
	ctx := buildContext(WorkflowEvent{Task: taskFor(1, models.TaskStatusTodo)}, "")
	got := interpolate("vence {{tarea.fecha_finn}}", ctx)
	if got != "vence {{tarea.fecha_finn}}" {
		t.Fatalf("la variable errónea debería quedar a la vista, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Destinatarios
// ---------------------------------------------------------------------------

// La cadena del "líder del proyecto" baja por aproximaciones. Con manager, gana el
// manager del asignado.
func TestLiderDelProyecto_PrefiereElManagerDelAsignado(t *testing.T) {
	users := map[uint]*models.User{
		7: activeUser(7, "Ana"), 3: activeUser(3, "Marta"), 9: activeUser(9, "Jefe"), 42: activeUser(42, "Empresa"),
	}
	s := newWfService(&wfRepo{}, &models.Board{ID: 1, CreatedBy: 9}, users, map[uint]uint{7: 3})

	ctx := buildContext(WorkflowEvent{Task: taskFor(1, models.TaskStatusInProcess, models.User{ID: 7, Name: "Ana"})}, "")
	got, why := s.projectLead(ctx, 42)

	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("debería resolver al manager del asignado (3), got %v (%s)", got, why)
	}
}

// Sin manager y sin supervisores, cae al creador del tablero.
func TestLiderDelProyecto_CaeAlCreadorDelTablero(t *testing.T) {
	users := map[uint]*models.User{7: activeUser(7, "Ana"), 9: activeUser(9, "Jefe"), 42: activeUser(42, "Empresa")}
	s := newWfService(&wfRepo{}, &models.Board{ID: 1, CreatedBy: 9}, users, nil)

	ctx := buildContext(WorkflowEvent{Task: taskFor(1, models.TaskStatusInProcess, models.User{ID: 7, Name: "Ana"})}, "")
	got, why := s.projectLead(ctx, 42)

	if len(got) != 1 || got[0] != 9 {
		t.Fatalf("debería caer al creador del tablero (9), got %v (%s)", got, why)
	}
}

// Cuando ningún nivel resuelve, se explica el recorrido en vez de callar.
func TestLiderDelProyecto_SinNadieExplicaElMotivo(t *testing.T) {
	s := newWfService(&wfRepo{}, &models.Board{ID: 1, CreatedBy: 0}, map[uint]*models.User{}, nil)

	ctx := buildContext(WorkflowEvent{Task: taskFor(1, models.TaskStatusInProcess)}, "")
	got, why := s.projectLead(ctx, 0)

	if len(got) != 0 {
		t.Fatalf("no debería resolver a nadie, got %v", got)
	}
	if !strings.Contains(why, "no hay líder de proyecto") {
		t.Fatalf("el motivo debería explicar el recorrido, got %q", why)
	}
}

// Un usuario dado de baja no recibe avisos, que es justo el motivo de resolver los
// destinatarios en ejecución y no al guardar la regla.
func TestDestinatarios_DescartanCuentasInactivasYDeSistema(t *testing.T) {
	users := map[uint]*models.User{
		7: {ID: 7, Name: "Ana", IsActive: true},
		8: {ID: 8, Name: "Baja", IsActive: false},
		9: {ID: 9, Name: "Obertrack", IsActive: true, IsSystem: true},
	}
	s := newWfService(&wfRepo{}, nil, users, nil)

	got := s.activeOnly([]uint{7, 8, 9, 0, 7})
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("sólo debería quedar el usuario activo y no de sistema, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

// El contexto viaja como JSON y se reinterpreta al ejecutar: lo que se guarda tiene
// que poder releerse tal cual, o un reintento evaluaría condiciones distintas.
func TestContexto_SobreviveAlViajePorJSON(t *testing.T) {
	original := buildContext(WorkflowEvent{
		Type:       models.TriggerTaskStatusChanged,
		Task:       taskFor(5, models.TaskStatusInProcess, models.User{ID: 7, Name: "Ana"}),
		PrevStatus: "por_hacer",
		ActorID:    3,
	}, "Carlos")

	var vuelta WorkflowContext
	if err := json.Unmarshal([]byte(mustJSON(original)), &vuelta); err != nil {
		t.Fatal(err)
	}

	if vuelta.Task.Estado != "en_proceso" || vuelta.Task.EstadoAnterior != "por_hacer" {
		t.Fatalf("el estado no sobrevivió: %+v", vuelta.Task)
	}
	if vuelta.Actor.Nombre != "Carlos" || vuelta.Board.CreadorID != 9 {
		t.Fatalf("actor o tablero no sobrevivieron: %+v / %+v", vuelta.Actor, vuelta.Board)
	}
	if got := conditionFields(vuelta)["tiene_responsable"]; got != true {
		t.Fatalf("las condiciones sobre el contexto releído deberían ver al responsable, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Regresiones encontradas al probar el circuito completo
// ---------------------------------------------------------------------------

// taskRepository.GetByID precarga creador, asignados, comentarios y adjuntos, pero
// NO Board. Sin rellenarlo, el snapshot guardaba el tablero en cero: {{tablero.nombre}}
// salía vacío en los avisos y el destinatario "creador del tablero" no resolvía a
// nadie. Ninguna prueba con fakes lo detectó porque los fakes sí traían el tablero.
func TestOnEvent_RellenaElTableroCuandoLaTareaNoLoTrae(t *testing.T) {
	repo := &wfRepo{rules: []models.Workflow{rule(1, models.TriggerTaskCreated, 1, "")}}
	board := &models.Board{ID: 1, Name: "Operación Acme", CreatedBy: 199, TenantID: 42}
	s := newWfService(repo, board, nil, nil)

	// Tarea tal como la devuelve el repositorio real: con board_id pero con la
	// asociación Board vacía.
	task := taskFor(1, models.TaskStatusTodo)
	task.Board = models.Board{}

	s.OnEvent(WorkflowEvent{Type: models.TriggerTaskCreated, TenantID: 42, Task: task, ActorID: 199})

	if len(repo.queued) != 1 {
		t.Fatalf("se esperaba una ejecución, hay %d", len(repo.queued))
	}
	var ctx WorkflowContext
	if err := json.Unmarshal([]byte(repo.queued[0].Context), &ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Board.Nombre != "Operación Acme" {
		t.Fatalf("el nombre del tablero debería haberse resuelto, got %q", ctx.Board.Nombre)
	}
	if ctx.Board.CreadorID != 199 {
		t.Fatalf("el creador del tablero debería haberse resuelto, got %d", ctx.Board.CreadorID)
	}
}
