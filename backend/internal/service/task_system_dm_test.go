package service

import (
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Estos tests fijan los avisos por DM del bot "Obertrack" que taskService dispara
// al asignar / cambiar la fecha / completar una tarea. Siguen el patrón de
// multitenant_scoping_test.go: cada fake embebe la interfaz real y sobrescribe
// SOLO lo que el path bajo prueba invoca. El emisor de DM se inyecta con
// SetSystemDM y captura (destinatario, contenido) en vez de tocar el chat real.

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type dmTaskRepo struct {
	repository.TaskRepository
	initial  *models.Task // 1ª llamada a GetByID (estado previo); opcional
	final    *models.Task // llamadas siguientes (estado recargado)
	getCalls int
}

func (r *dmTaskRepo) Create(task *models.Task) error {
	task.ID = 100
	return nil
}
func (r *dmTaskRepo) SyncAssignees(_ *models.Task, _ []uint) error         { return nil }
func (r *dmTaskRepo) Update(_ *models.Task, _ map[string]interface{}) error { return nil }
func (r *dmTaskRepo) GetByID(_ uint) (*models.Task, error) {
	r.getCalls++
	// La 1ª lectura (authorizeTaskByID) devuelve el estado previo; las siguientes
	// (finalTask) el recargado. Así el test puede simular un cambio real de fecha.
	if r.initial != nil && r.getCalls == 1 {
		return r.initial, nil
	}
	return r.final, nil
}
func (r *dmTaskRepo) GetByIDAndTenant(_, _ uint) (*models.Task, error) { return r.final, nil }

type dmBoardRepo struct {
	repository.BoardRepository
	board *models.Board
}

func (r *dmBoardRepo) GetByID(_ uint) (*models.Board, error)      { return r.board, nil }
func (r *dmBoardRepo) AddMember(_ *models.Board, _ *models.User) error { return nil }

type dmUserRepo struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (r *dmUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return &models.User{ID: id, Name: "User"}, nil
}

type capturedDM struct {
	to      uint
	content string
}

func newDMTaskServiceWith(repo *dmTaskRepo, board *models.Board) (*taskService, *[]capturedDM) {
	var got []capturedDM
	s := &taskService{
		repo:      repo,
		userRepo:  &dmUserRepo{users: map[uint]*models.User{}},
		boardRepo: &dmBoardRepo{board: board},
		notifSvc:  &fakeNotifSvc{},
	}
	s.SetSystemDM(func(to uint, content string) {
		got = append(got, capturedDM{to: to, content: content})
	})
	return s, &got
}

func newDMTaskService(final *models.Task, board *models.Board) (*taskService, *[]capturedDM) {
	return newDMTaskServiceWith(&dmTaskRepo{final: final}, board)
}

func findDM(dms []capturedDM, to uint) (capturedDM, bool) {
	for _, d := range dms {
		if d.to == to {
			return d, true
		}
	}
	return capturedDM{}, false
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreate_SendsAssignmentDMWithDueDate(t *testing.T) {
	due := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	final := &models.Task{
		ID:      100,
		Title:   "Revisar informe",
		BoardID: 1,
		EndDate: &due,
		Assignees: []models.User{
			{ID: 7, Name: "Ana"},
		},
	}
	board := &models.Board{ID: 1, CreatedBy: 9, Members: []models.User{{ID: 7}}}
	s, dms := newDMTaskService(final, board)

	_, _, err := s.Create(9, true, 0, "Revisar informe", "", "medium", nil, []uint{7}, 1)
	if err != nil {
		t.Fatal(err)
	}

	dm, ok := findDM(*dms, 7)
	if !ok {
		t.Fatal("el asignado 7 debería recibir un DM de asignación")
	}
	if !strings.Contains(dm.content, "Se te asignó la tarea: Revisar informe") {
		t.Fatalf("contenido inesperado: %q", dm.content)
	}
	if !strings.Contains(dm.content, "vence 20/07/2026") {
		t.Fatalf("el DM debería incluir la fecha de vencimiento, got %q", dm.content)
	}
}

func TestCreate_NoDueDate_OmitsDueSuffix(t *testing.T) {
	final := &models.Task{
		ID: 100, Title: "Sin fecha", BoardID: 1,
		Assignees: []models.User{{ID: 7, Name: "Ana"}},
	}
	board := &models.Board{ID: 1, CreatedBy: 9, Members: []models.User{{ID: 7}}}
	s, dms := newDMTaskService(final, board)

	if _, _, err := s.Create(9, true, 0, "Sin fecha", "", "medium", nil, []uint{7}, 1); err != nil {
		t.Fatal(err)
	}

	dm, _ := findDM(*dms, 7)
	if strings.Contains(dm.content, "vence") {
		t.Fatalf("sin fecha límite no debería mencionar vencimiento, got %q", dm.content)
	}
}

// ---------------------------------------------------------------------------
// Update — cambio de fecha
// ---------------------------------------------------------------------------

// Regresión: cambiar la prioridad (u otro campo) NO debe avisar cambio de fecha.
// El frontend reenvía end_date en cada edición, así que detectar el cambio por la
// presencia de la clave daba un falso positivo.
func TestUpdate_NonDeadlineChangeSendsNoDeadlineDM(t *testing.T) {
	due := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	// initial nil ⇒ GetByID devuelve siempre el mismo estado ⇒ fecha sin cambios.
	task := &models.Task{
		ID: 100, Title: "Osvell", BoardID: 1, EndDate: &due,
		Assignees: []models.User{{ID: 7, Name: "Ana"}},
	}
	board := &models.Board{ID: 1, CreatedBy: 9}
	s, dms := newDMTaskServiceWith(&dmTaskRepo{final: task}, board)

	// El empleador 9 solo cambia la prioridad; no reasigna (assignees nil).
	if _, _, err := s.Update(100, 0, 9, "empleador", false, true, map[string]interface{}{"priority": "high"}, nil); err != nil {
		t.Fatal(err)
	}

	if len(*dms) != 0 {
		t.Fatalf("un cambio que no toca la fecha no debe generar DMs, se enviaron %d: %+v", len(*dms), *dms)
	}
}

// Cambiar la fecha de verdad sí avisa por DM a los asignados que ya estaban.
func TestUpdate_RealDeadlineChangeSendsDeadlineDM(t *testing.T) {
	oldDue := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	newDue := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	initial := &models.Task{
		ID: 100, Title: "Osvell", BoardID: 1, EndDate: &oldDue,
		Assignees: []models.User{{ID: 7, Name: "Ana"}},
	}
	final := &models.Task{
		ID: 100, Title: "Osvell", BoardID: 1, EndDate: &newDue,
		Assignees: []models.User{{ID: 7, Name: "Ana"}},
	}
	board := &models.Board{ID: 1, CreatedBy: 9}
	s, dms := newDMTaskServiceWith(&dmTaskRepo{initial: initial, final: final}, board)

	if _, _, err := s.Update(100, 0, 9, "empleador", false, true, map[string]interface{}{"end_date": "2026-10-09"}, nil); err != nil {
		t.Fatal(err)
	}

	dm, ok := findDM(*dms, 7)
	if !ok {
		t.Fatal("cambiar la fecha debería avisar por DM al asignado 7")
	}
	if !strings.Contains(dm.content, "ahora vence 09/10/2026") {
		t.Fatalf("contenido inesperado: %q", dm.content)
	}
}

// ---------------------------------------------------------------------------
// ToggleCompletion
// ---------------------------------------------------------------------------

func TestToggleCompletion_CompletedNotifiesAssigneesButNotUpdater(t *testing.T) {
	// task inicial (antes de completar): completed=false → se completará.
	initial := &models.Task{
		ID: 100, Title: "Cerrar sprint", BoardID: 1, Completed: false,
		Assignees: []models.User{{ID: 7, Name: "Ana"}, {ID: 8, Name: "Beto"}},
	}
	board := &models.Board{ID: 1, CreatedBy: 9}
	s, dms := newDMTaskService(initial, board)

	// Actualiza (completa) el usuario 8: no debe recibirse a sí mismo el DM.
	if _, err := s.ToggleCompletion(100, 0, 8, "empleador", false, true); err != nil {
		t.Fatal(err)
	}

	if _, ok := findDM(*dms, 7); !ok {
		t.Fatal("el asignado 7 debería recibir el DM de completada")
	}
	if _, ok := findDM(*dms, 8); ok {
		t.Fatal("el usuario que completó (8) NO debería recibir un DM")
	}
	if dm, _ := findDM(*dms, 7); !strings.Contains(dm.content, "Se completó la tarea: Cerrar sprint") {
		t.Fatalf("contenido inesperado: %q", dm.content)
	}
}

func TestToggleCompletion_ReopenSendsNoDM(t *testing.T) {
	// completed=true → al alternar se reabre; reabrir no manda DM.
	initial := &models.Task{
		ID: 100, Title: "Cerrar sprint", BoardID: 1, Completed: true,
		Assignees: []models.User{{ID: 7, Name: "Ana"}},
	}
	board := &models.Board{ID: 1, CreatedBy: 9}
	s, dms := newDMTaskService(initial, board)

	if _, err := s.ToggleCompletion(100, 0, 9, "empleador", false, true); err != nil {
		t.Fatal(err)
	}
	if len(*dms) != 0 {
		t.Fatalf("reabrir no debería enviar DMs, se enviaron %d", len(*dms))
	}
}

// ---------------------------------------------------------------------------
// PostSystemDM — guardas
// ---------------------------------------------------------------------------

type dmChannelRepo struct {
	repository.ChannelRepository
	touched bool // true si se intentó abrir/crear el DM
}

func (r *dmChannelRepo) GetChannelByNameAndType(_ string, _ models.ChannelType, _ uint) (*models.Channel, error) {
	r.touched = true
	return nil, nil
}
func (r *dmChannelRepo) CreateDMChannel(_ *models.Channel, _ []uint) error {
	r.touched = true
	return nil
}

type dmBotUserRepo struct {
	repository.UserRepository
	bot   *models.User
	users map[uint]*models.User
}

func (r *dmBotUserRepo) GetByEmail(_ string) (*models.User, error) {
	if r.bot == nil {
		return nil, nil
	}
	return r.bot, nil
}
func (r *dmBotUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

// Un destinatario superadmin no tiene tenant donde vive un DM: se debe saltar
// sin tocar el repositorio de canales.
func TestPostSystemDM_SkipsSuperadminRecipient(t *testing.T) {
	chRepo := &dmChannelRepo{}
	s := &channelService{
		repo: chRepo,
		userRepo: &dmBotUserRepo{
			bot:   &models.User{ID: 50, UserType: models.UserTypeSuperadmin, Email: models.SystemBotEmail},
			users: map[uint]*models.User{7: {ID: 7, UserType: models.UserTypeSuperadmin}},
		},
	}

	s.PostSystemDM(7, "hola")

	if chRepo.touched {
		t.Fatal("un destinatario superadmin (tenant 0) no debería abrir un DM")
	}
}

// Sin usuario bot (migración no corrida), PostSystemDM se salta silenciosamente.
func TestPostSystemDM_SkipsWhenBotMissing(t *testing.T) {
	chRepo := &dmChannelRepo{}
	s := &channelService{
		repo:     chRepo,
		userRepo: &dmBotUserRepo{bot: nil, users: map[uint]*models.User{}},
	}

	s.PostSystemDM(7, "hola")

	if chRepo.touched {
		t.Fatal("sin usuario bot no debería intentar abrir un DM")
	}
}
