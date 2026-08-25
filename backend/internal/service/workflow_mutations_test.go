package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Las acciones que MUTAN son las primeras del motor que cambian el dato, así que lo
// que hay que fijar no es sólo que funcionen: es que no pisen el trabajo de nadie,
// que no se cuelen por una puerta y que lo que provoquen siga contando en el antibucle.

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type appliedChange struct {
	taskID    uint
	updates   map[string]interface{}
	assignees []uint
	cause     WorkflowCause
}

type fakeMutator struct {
	applied  []appliedChange
	comments []string
	// gateErr simula una puerta en la columna destino.
	gateErr error
}

func (m *fakeMutator) ApplyAsSystem(taskID uint, updates map[string]interface{}, assignees *[]uint, cause WorkflowCause) (*models.Task, error) {
	if m.gateErr != nil {
		return nil, m.gateErr
	}
	change := appliedChange{taskID: taskID, updates: updates, cause: cause}
	if assignees != nil {
		change.assignees = *assignees
	}
	m.applied = append(m.applied, change)
	return &models.Task{ID: taskID}, nil
}

func (m *fakeMutator) AddComment(_ uint, _ uint, _ uint, content string, _ bool) (*models.Comment, error) {
	m.comments = append(m.comments, content)
	return &models.Comment{ID: uint(len(m.comments))}, nil
}

// mutTaskRepo devuelve el estado ACTUAL de la tarea, que puede diferir del snapshot.
type mutTaskRepo struct {
	histTaskRepo
	current *models.Task
}

func (r *mutTaskRepo) GetByID(_ uint) (*models.Task, error) { return r.current, nil }

func mutSvc(current *models.Task, users map[uint]*models.User) (*WorkflowService, *fakeMutator) {
	repo := &wfRepo{}
	s := NewWorkflowService(
		repo, &mutTaskRepo{current: current},
		&wfBoardRepo{board: &models.Board{ID: 1, TenantID: 42}},
		&wfUserRepo{users: users},
		&wfEmpRepo{}, &fakeNotifSvc{}, nil,
	)
	m := &fakeMutator{}
	s.SetTaskMutator(m)
	return s, m
}

// snapshotCtx arma el contexto que el motor guardó al dispararse.
func snapshotCtx(estado, prioridad string, asignados ...uint) WorkflowContext {
	return WorkflowContext{
		Trigger: models.TriggerTaskStatusChanged,
		Task: workflowTaskCtx{
			ID: 100, Titulo: "Revisar informe",
			Estado: estado, Prioridad: prioridad, AsignadosIDs: asignados,
		},
		Board: workflowBoardCtx{ID: 1},
	}
}

func mutStep(action string, cfg stepConfig) models.WorkflowStep {
	return models.WorkflowStep{ID: 1, ActionType: action, Config: mustJSON(cfg)}
}

func mutRun() *models.WorkflowRun {
	return &models.WorkflowRun{ID: 7, WorkflowID: 3, TenantID: 42, Depth: 0}
}

// ---------------------------------------------------------------------------
// Obsolescencia
// ---------------------------------------------------------------------------

// La defensa que más importa: entre el disparo y la ejecución pueden pasar minutos si
// hubo reintentos. Si alguien movió la tarea mientras tanto, la consecuencia calculada
// sobre el estado viejo ya no se sostiene y aplicarla pisaría trabajo de una persona.
func TestMutacion_NoAplicaSobreUnaTareaQueYaCambio(t *testing.T) {
	// El snapshot dice "en_proceso"; la tarea ya está en "finalizado".
	actual := &models.Task{ID: 100, Status: models.TaskStatusDone, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)

	_, why, err := s.runAction(
		mutStep(models.ActionSetPriority, stepConfig{Priority: "urgent"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	if err != nil {
		t.Fatal(err)
	}
	if len(m.applied) != 0 {
		t.Fatalf("no debería haber escrito nada, aplicó %+v", m.applied)
	}
	if !strings.Contains(why, "cambió de columna desde el disparo") {
		t.Fatalf("el motivo debería explicar la obsolescencia, got %q", why)
	}
}

func TestMutacion_TampocoSiCambioLaPrioridad(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityUrgent}
	s, m := mutSvc(actual, nil)

	_, why, _ := s.runAction(
		mutStep(models.ActionSetStatus, stepConfig{Status: "finalizado"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	if len(m.applied) != 0 {
		t.Fatal("no debería haber escrito nada")
	}
	if !strings.Contains(why, "prioridad") {
		t.Fatalf("el motivo debería señalar la prioridad, got %q", why)
	}
}

// ---------------------------------------------------------------------------
// Prioridad
// ---------------------------------------------------------------------------

func TestMutacion_SubeLaPrioridadYArrastraLaCadena(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)

	run := mutRun()
	run.Depth = 1
	out, why, err := s.runAction(
		mutStep(models.ActionSetPriority, stepConfig{Priority: "urgent"}),
		run, snapshotCtx("en_proceso", "medium"))

	if err != nil || why != "" {
		t.Fatalf("debería aplicarse: why=%q err=%v", why, err)
	}
	if len(m.applied) != 1 || m.applied[0].updates["priority"] != "urgent" {
		t.Fatalf("no se aplicó la prioridad, got %+v", m.applied)
	}
	if out["prioridad"] != "urgent" || out["prioridad_anterior"] != "medium" {
		t.Fatalf("la salida del paso debería registrar el cambio, got %v", out)
	}

	// La cadena causal es lo que hace que el antibucle siga contando cuando esta
	// acción provoque un evento nuevo.
	c := m.applied[0].cause
	if c.RunID != 7 || c.WorkflowID != 3 || c.Depth != 1 {
		t.Fatalf("la cadena causal no viajó, got %+v", c)
	}
}

// Poner la prioridad que ya tenía no se escribe: además de ahorrar la escritura, evita
// emitir un "cambió la prioridad" que no cambió nada y que podría realimentar otra regla.
func TestMutacion_NoReescribeLoQueYaEsta(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityUrgent}
	s, m := mutSvc(actual, nil)

	_, why, _ := s.runAction(
		mutStep(models.ActionSetPriority, stepConfig{Priority: "urgent"}),
		mutRun(), snapshotCtx("en_proceso", "urgent"))

	if len(m.applied) != 0 {
		t.Fatal("no debería escribir si el valor ya es el pedido")
	}
	if !strings.Contains(why, "ya tenía prioridad") {
		t.Fatalf("motivo inesperado: %q", why)
	}
}

func TestMutacion_RechazaUnaPrioridadInventada(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)

	_, why, _ := s.runAction(
		mutStep(models.ActionSetPriority, stepConfig{Priority: "urgentísima"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	if len(m.applied) != 0 {
		t.Fatal("una prioridad inválida no puede llegar a la base")
	}
	if !strings.Contains(why, "no es válida") {
		t.Fatalf("motivo inesperado: %q", why)
	}
}

// ---------------------------------------------------------------------------
// Puertas
// ---------------------------------------------------------------------------

// Una acción suelta NO atraviesa una columna cerrada. Dejarla pasar abriría justo el
// agujero que la puerta previene, y reintentarlo no serviría: una automatización no
// puede rellenar un formulario.
func TestMutacion_NoSeCuelaPorUnaPuerta(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)
	m.gateErr = &GateRequiredError{WorkflowID: 9, Workflow: "Cierre con reporte", ToStatus: "finalizado"}

	_, why, err := s.runAction(
		mutStep(models.ActionSetStatus, stepConfig{Status: "finalizado"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	// Saltada con motivo, NO fallida: reintentar seis veces no cambiaría nada.
	if err != nil {
		t.Fatalf("no debería tratarse como error reintentable: %v", err)
	}
	if !strings.Contains(why, "exige un formulario") || !strings.Contains(why, "Cierre con reporte") {
		t.Fatalf("el motivo debería nombrar la puerta, got %q", why)
	}
}

// Pero una consecuencia nacida de una puerta YA cruzada sí pasa: la justificación
// existe, la dio una persona, y viaja con la cadena.
func TestMutacion_LaConsecuenciaDeUnaPuertaSiAtraviesaOtra(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)

	ctx := snapshotCtx("en_proceso", "medium")
	ctx.Trigger = models.TriggerTaskEnteringPhase

	if _, why, err := s.runAction(
		mutStep(models.ActionSetStatus, stepConfig{Status: "finalizado"}),
		mutRun(), ctx); err != nil || why != "" {
		t.Fatalf("debería aplicarse: why=%q err=%v", why, err)
	}
	if !m.applied[0].cause.GateJustified {
		t.Fatal("la consecuencia de una puerta debe viajar justificada")
	}
}

// ---------------------------------------------------------------------------
// Asignación y comentario
// ---------------------------------------------------------------------------

// Asignar SUMA, no reemplaza: sacar de la tarea a quien sí está trabajando en ella
// sería una pérdida silenciosa de información.
func TestMutacion_AsignarSumaSinEcharANadie(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	users := map[uint]*models.User{
		7: activeUser(7, "Ana"), 9: activeUser(9, "Jefe"),
	}
	s, m := mutSvc(actual, users)

	ctx := snapshotCtx("en_proceso", "medium", 7)
	ctx.Board.CreadorID = 9

	_, why, err := s.runAction(
		mutStep(models.ActionAssign, stepConfig{Recipient: models.RecipientBoardCreator}),
		mutRun(), ctx)
	if err != nil || why != "" {
		t.Fatalf("debería aplicarse: why=%q err=%v", why, err)
	}

	final := m.applied[0].assignees
	if len(final) != 2 || final[0] != 7 || final[1] != 9 {
		t.Fatalf("debería conservar al 7 y sumar al 9, got %v", final)
	}
}

func TestMutacion_NoAsignaAQuienYaEstaba(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, map[uint]*models.User{9: activeUser(9, "Jefe")})

	ctx := snapshotCtx("en_proceso", "medium", 9)
	ctx.Board.CreadorID = 9

	_, why, _ := s.runAction(
		mutStep(models.ActionAssign, stepConfig{Recipient: models.RecipientBoardCreator}),
		mutRun(), ctx)

	if len(m.applied) != 0 {
		t.Fatal("no debería reescribir la asignación")
	}
	if !strings.Contains(why, "ya estaba asignado") {
		t.Fatalf("motivo inesperado: %q", why)
	}
}

// El comentario lo firma el bot: atribuirlo a una persona haría creer que lo escribió
// alguien.
func TestMutacion_ComentaFirmandoElBot(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	bot := &models.User{ID: 149, Name: models.SystemBotName, Email: models.SystemBotEmail, IsActive: true, IsSystem: true}
	s, m := mutSvc(actual, map[uint]*models.User{149: bot})

	_, why, err := s.runAction(
		mutStep(models.ActionComment, stepConfig{Content: "Movida automáticamente: {{tarea.titulo}}"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	if err != nil || why != "" {
		t.Fatalf("debería comentar: why=%q err=%v", why, err)
	}
	if len(m.comments) != 1 || !strings.Contains(m.comments[0], "Revisar informe") {
		t.Fatalf("el comentario debería llevar las variables resueltas, got %v", m.comments)
	}
}

// Sin el módulo de Tareas cableado, las acciones que mutan se saltan con motivo en vez
// de reventar: es lo que mantiene el resto de la ejecución en pie.
func TestMutacion_SinModuloDeTareasSeSalta(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, _ := mutSvc(actual, nil)
	s.SetTaskMutator(nil)

	_, why, err := s.runAction(
		mutStep(models.ActionSetPriority, stepConfig{Priority: "urgent"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if !strings.Contains(why, "no están disponibles") {
		t.Fatalf("motivo inesperado: %q", why)
	}
}

// Un error real de escritura SÍ es reintentable: eso lo distingue de una puerta.
func TestMutacion_UnFalloDeEscrituraSeReintenta(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)
	m.gateErr = errors.New("conexión perdida")

	_, why, err := s.runAction(
		mutStep(models.ActionSetPriority, stepConfig{Priority: "urgent"}),
		mutRun(), snapshotCtx("en_proceso", "medium"))

	if err == nil {
		t.Fatalf("un fallo de escritura debe propagarse para reintentarse, why=%q", why)
	}
}
