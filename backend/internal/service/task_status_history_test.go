package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Fase 0 del motor de workflows: la bitácora de movimientos de columna y el sello
// tasks.status_changed_at. Lo que se fija aquí es que la bitácora sea FIEL —una
// fila por movimiento real, ninguna por un guardado que no movió nada— porque de
// ella dependen la antigüedad que ve el usuario y el disparador schedule.task_stale.
//
// Mismo patrón de fakes que task_system_dm_test.go: cada uno embebe la interfaz real
// y sobrescribe sólo lo que el camino bajo prueba invoca.

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type histTaskRepo struct {
	repository.TaskRepository
	initial *models.Task // 1ª GetByID (estado previo)
	final   *models.Task // GetByID siguientes (estado recargado)

	history []models.TaskStatusHistory // filas anotadas
	// txEntries recoge lo escrito por el camino TRANSACCIONAL (puertas de fase),
	// separado de history para poder distinguir en las pruebas qué camino se usó.
	txEntries []models.TaskStatusHistory
	updates []map[string]interface{}   // mapas que llegaron a Update
	created *models.Task               // instancia pasada a Create
	getCall int
}

func (r *histTaskRepo) Create(task *models.Task) error {
	task.ID = 100
	r.created = task
	return nil
}

func (r *histTaskRepo) Update(_ *models.Task, updates map[string]interface{}) error {
	r.updates = append(r.updates, updates)
	return nil
}

func (r *histTaskRepo) SyncAssignees(_ *models.Task, _ []uint) error { return nil }
func (r *histTaskRepo) NextOrder(_ uint, _ string) int               { return 0 }

func (r *histTaskRepo) AddStatusHistory(entry *models.TaskStatusHistory) error {
	r.history = append(r.history, *entry)
	return nil
}

func (r *histTaskRepo) UpdateWithStatusHistory(_ *models.Task, updates map[string]interface{}, entry *models.TaskStatusHistory) error {
	r.updates = append(r.updates, updates)
	r.txEntries = append(r.txEntries, *entry)
	return nil
}

func (r *histTaskRepo) GetByID(_ uint) (*models.Task, error) {
	r.getCall++
	if r.initial != nil && r.getCall == 1 {
		return r.initial, nil
	}
	return r.final, nil
}

func (r *histTaskRepo) GetByIDAndTenant(_, _ uint) (*models.Task, error) {
	return r.GetByID(0)
}

func newHistTaskService(repo *histTaskRepo, board *models.Board) *taskService {
	return &taskService{
		repo:      repo,
		userRepo:  &dmUserRepo{users: map[uint]*models.User{}},
		boardRepo: &dmBoardRepo{board: board},
		notifSvc:  &fakeNotifSvc{},
	}
}

// lastUpdate devuelve el último mapa que llegó al repositorio.
func lastUpdate(r *histTaskRepo) map[string]interface{} {
	if len(r.updates) == 0 {
		return nil
	}
	return r.updates[len(r.updates)-1]
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreate_SellaEntradaEnColumnaYAbreBitacora(t *testing.T) {
	board := &models.Board{
		ID: 1, CreatedBy: 9, TenantID: 42,
		Phases:  []models.Phase{{ID: 1, Name: "Por hacer", Status: "por_hacer", Order: 0}},
		Members: []models.User{{ID: 7}},
	}
	repo := &histTaskRepo{final: &models.Task{ID: 100, BoardID: 1, TenantID: 42}}
	s := newHistTaskService(repo, board)

	if _, _, err := s.Create(9, true, 42, "Nueva", "", "medium", nil, nil, 1); err != nil {
		t.Fatal(err)
	}

	if repo.created == nil || repo.created.StatusChangedAt == nil {
		t.Fatal("una tarea nueva debe nacer con status_changed_at sellado, o aparecería sin antigüedad hasta que alguien la mueva")
	}
	if len(repo.history) != 1 {
		t.Fatalf("la creación debe abrir la bitácora con una fila, hay %d", len(repo.history))
	}
	entry := repo.history[0]
	if entry.FromStatus != "" {
		t.Fatalf("la fila de creación se distingue por FromStatus vacío, got %q", entry.FromStatus)
	}
	if entry.ToStatus != "por_hacer" {
		t.Fatalf("ToStatus debería ser la primera fase del tablero, got %q", entry.ToStatus)
	}
	if entry.ChangedBy == nil || *entry.ChangedBy != 9 {
		t.Fatalf("ChangedBy debería ser el creador (9), got %v", entry.ChangedBy)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestUpdate_CambioDeColumna_AnotaYSella(t *testing.T) {
	board := &models.Board{ID: 1, CreatedBy: 9, TenantID: 42}
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
	}
	s := newHistTaskService(repo, board)

	_, _, err := s.Update(100, 42, 7, "profesional", false, true,
		map[string]interface{}{"status": "en_proceso"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(repo.history) != 1 {
		t.Fatalf("un cambio de columna debe anotar exactamente una fila, hay %d", len(repo.history))
	}
	entry := repo.history[0]
	if entry.FromStatus != "por_hacer" || entry.ToStatus != "en_proceso" {
		t.Fatalf("se esperaba por_hacer → en_proceso, got %q → %q", entry.FromStatus, entry.ToStatus)
	}
	if entry.ChangedBy == nil || *entry.ChangedBy != 7 {
		t.Fatalf("ChangedBy debería ser quien movió la tarjeta (7), got %v", entry.ChangedBy)
	}

	// El sello viaja en el MISMO mapa que el status: si se escribiera aparte,
	// un fallo entre ambas escrituras dejaría la antigüedad mintiendo.
	if _, ok := lastUpdate(repo)["status_changed_at"]; !ok {
		t.Fatal("status_changed_at debe ir en el mismo Updates que el status")
	}
}

func TestUpdate_SinCambioDeColumna_NoTocaLaBitacoraNiElSello(t *testing.T) {
	board := &models.Board{ID: 1, CreatedBy: 9, TenantID: 42}
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
	}
	s := newHistTaskService(repo, board)

	// El formulario de edición reenvía el status actual en CADA guardado, así que
	// este es el caso corriente: cambiar el título no puede reiniciar la antigüedad.
	_, _, err := s.Update(100, 42, 7, "profesional", false, true,
		map[string]interface{}{"title": "Otro título", "status": "en_proceso"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(repo.history) != 0 {
		t.Fatalf("un guardado que no mueve la tarjeta no debe anotar nada, hay %d filas", len(repo.history))
	}
	if _, ok := lastUpdate(repo)["status_changed_at"]; ok {
		t.Fatal("editar el título no debe re-sellar status_changed_at: la tarea sigue en la misma columna")
	}
}

// ---------------------------------------------------------------------------
// ToggleCompletion
// ---------------------------------------------------------------------------

func TestToggleCompletion_CompletarEsUnMovimientoDeColumna(t *testing.T) {
	board := &models.Board{
		ID: 1, CreatedBy: 9, TenantID: 42,
		Phases: []models.Phase{{ID: 1, Name: "Por hacer", Status: "por_hacer", Order: 0}},
	}
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusDone, Completed: true},
	}
	s := newHistTaskService(repo, board)

	if _, err := s.ToggleCompletion(100, 42, 7, "profesional", false, true); err != nil {
		t.Fatal(err)
	}

	if len(repo.history) != 1 {
		t.Fatalf("completar debe anotar el movimiento a finalizado, hay %d filas", len(repo.history))
	}
	entry := repo.history[0]
	if entry.FromStatus != "en_proceso" || entry.ToStatus != "finalizado" {
		t.Fatalf("se esperaba en_proceso → finalizado, got %q → %q", entry.FromStatus, entry.ToStatus)
	}
	if _, ok := lastUpdate(repo)["status_changed_at"]; !ok {
		t.Fatal("completar cambia de columna, así que debe re-sellar status_changed_at")
	}
}

func TestToggleCompletion_ReabrirALaMismaColumna_NoAnota(t *testing.T) {
	// La tarea está marcada como completada pero su status ya es la primera fase.
	// Reabrirla no la mueve a ningún lado: no hay movimiento que anotar.
	board := &models.Board{
		ID: 1, CreatedBy: 9, TenantID: 42,
		Phases: []models.Phase{{ID: 1, Name: "Por hacer", Status: "por_hacer", Order: 0}},
	}
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo, Completed: true},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo},
	}
	s := newHistTaskService(repo, board)

	if _, err := s.ToggleCompletion(100, 42, 7, "profesional", false, true); err != nil {
		t.Fatal(err)
	}

	if len(repo.history) != 0 {
		t.Fatalf("reabrir sin cambiar de columna no debe anotar nada, hay %d filas", len(repo.history))
	}
	if _, ok := lastUpdate(repo)["status_changed_at"]; ok {
		t.Fatal("sin movimiento de columna no se re-sella status_changed_at")
	}
}

// ---------------------------------------------------------------------------
// Aislamiento por tenant
// ---------------------------------------------------------------------------

// La bitácora es la base de schedule.task_stale, que consultará por tenant. Una
// fila con el tenant equivocado haría que una regla de la empresa A viera
// movimientos de la B, así que el tenant se hereda SIEMPRE de la tarea y nunca del
// actor ni del tablero.
func TestHistoria_HeredaElTenantDeLaTarea(t *testing.T) {
	board := &models.Board{ID: 1, CreatedBy: 9, TenantID: 42}
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 77, Status: models.TaskStatusTodo},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 77, Status: models.TaskStatusInProcess},
	}
	s := newHistTaskService(repo, board)

	// isSuperadmin=true para que la autorización no dependa del tenant del actor:
	// lo que se comprueba aquí es de dónde sale el tenant de la fila.
	_, _, err := s.Update(100, 0, 7, "superadmin", false, true,
		map[string]interface{}{"status": "en_proceso"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(repo.history) != 1 {
		t.Fatalf("se esperaba una fila de bitácora, hay %d", len(repo.history))
	}
	if got := repo.history[0].TenantID; got != 77 {
		t.Fatalf("la fila debe llevar el tenant de la TAREA (77), no el del tablero (42); got %d", got)
	}
	if repo.history[0].TaskID != 100 {
		t.Fatalf("TaskID incorrecto: %d", repo.history[0].TaskID)
	}
}
