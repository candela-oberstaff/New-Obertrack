package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Fase 5: el escalado. La decisión de producto es que el aviso de pendientes
// siga yendo SOLO al manager directo y que al supervisor le llegue únicamente lo
// que lleva demasiado sin resolver — si no, se le devuelve el ruido de toda su
// estructura, que es justo lo que se quiso evitar.
//
// Lo que se protege aquí: que no se avise de más (ventana, árbol vacío, flag) y
// que el registro se escriba antes de notificar.

// fakeEscalationWHRepo captura los filtros del conteo y simula el registro de
// avisos.
type fakeEscalationWHRepo struct {
	repository.WorkHourRepository

	pending     int64
	canEscalate bool
	canErr      error
	findErr     error

	lastFilters map[string]interface{}
	markedIDs   []uint
}

func (f *fakeEscalationWHRepo) FindAll(filters map[string]interface{}, _, _ int) ([]models.WorkHour, int64, error) {
	f.lastFilters = filters
	if f.findErr != nil {
		return nil, 0, f.findErr
	}
	return nil, f.pending, nil
}

func (f *fakeEscalationWHRepo) CanEscalateTo(_ uint, _ time.Duration) (bool, error) {
	return f.canEscalate, f.canErr
}

func (f *fakeEscalationWHRepo) MarkEscalationSent(userID uint) error {
	f.markedIDs = append(f.markedIDs, userID)
	return nil
}

// fakeSupervisorListRepo entrega la lista de supervisores activos.
type fakeSupervisorListRepo struct {
	repository.UserRepository
	supervisors []models.User
	listErr     error
}

func (f *fakeSupervisorListRepo) ListActiveSupervisors() ([]models.User, error) {
	return f.supervisors, f.listErr
}

func escalationSetup(pending int64, canEscalate bool) (*SupervisorEscalationWatcher, *fakeEscalationWHRepo, *fakeNotifSvc) {
	userRepo := &fakeSupervisorListRepo{supervisors: []models.User{{
		ID: supervisorID, UserType: models.UserTypeProfessional,
		IsManager: true, IsSupervisor: true, IsActive: true,
		EmpleadorID: uintPtr(supCompanyID),
	}}}
	_, empRepo := supervisorSetup()
	whRepo := &fakeEscalationWHRepo{pending: pending, canEscalate: canEscalate}
	notif := &fakeNotifSvc{}
	return NewSupervisorEscalationWatcher(userRepo, empRepo, whRepo, notif), whRepo, notif
}

// Con jornadas atascadas en el árbol, al supervisor le llega UN aviso resumido.
func TestEscalation_AvisaCuandoHayPendientesViejas(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	w, whRepo, notif := escalationSetup(7, true)

	n, err := w.RunOnce()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperaba 1 supervisor avisado, got %d", n)
	}
	if len(notif.notified) != 1 || notif.notified[0] != supervisorID {
		t.Fatalf("el aviso debe ir al supervisor, got %v", notif.notified)
	}
	// El registro se escribe ANTES de notificar: perder un aviso es más barato
	// que duplicarlo.
	if len(whRepo.markedIDs) != 1 {
		t.Fatal("debe quedar constancia del aviso")
	}

	// El conteo mira el ÁRBOL, no el tenant entero, y solo lo pendiente y viejo.
	ids, ok := whRepo.lastFilters["user_ids"].([]uint)
	if !ok || len(ids) != 2 {
		t.Fatalf("debe contarse sobre el árbol del supervisor, got %v", whRepo.lastFilters)
	}
	if whRepo.lastFilters["approved"] != false || whRepo.lastFilters["rejected"] != false {
		t.Fatalf("solo cuentan las pendientes, got %v", whRepo.lastFilters)
	}
	if _, ok := whRepo.lastFilters["end_date"].(time.Time); !ok {
		t.Fatal("debe aplicarse el corte de antigüedad")
	}
}

// Dentro de la ventana no se repite el aviso, aunque las jornadas sigan ahí.
func TestEscalation_RespetaLaVentana(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	w, whRepo, notif := escalationSetup(7, false)

	if n, err := w.RunOnce(); err != nil || n != 0 {
		t.Fatalf("no debe reavisarse dentro de la ventana, got n=%d err=%v", n, err)
	}
	if len(notif.notified) != 0 {
		t.Fatal("no debe emitirse ninguna notificación")
	}
	// Ni siquiera se cuenta: a quien ya se avisó no hace falta calcularle nada.
	if whRepo.lastFilters != nil {
		t.Fatal("no debe consultarse nada para quien ya fue avisado")
	}
}

// Sin nada atascado no se avisa (y no se gasta un registro).
func TestEscalation_SinPendientesNoAvisa(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	w, whRepo, notif := escalationSetup(0, true)

	if n, err := w.RunOnce(); err != nil || n != 0 {
		t.Fatalf("sin pendientes no hay aviso, got n=%d err=%v", n, err)
	}
	if len(notif.notified) != 0 || len(whRepo.markedIDs) != 0 {
		t.Fatal("no debe notificarse ni registrarse nada")
	}
}

// Con el flag apagado el watcher es un no-op: el rol no existe.
func TestEscalation_FlagApagadoEsNoOp(t *testing.T) {
	w, whRepo, notif := escalationSetup(7, true)

	if n, err := w.RunOnce(); err != nil || n != 0 {
		t.Fatalf("sin SUPERVISOR_SCOPE no debe hacer nada, got n=%d err=%v", n, err)
	}
	if len(notif.notified) != 0 || whRepo.lastFilters != nil {
		t.Fatal("no debe tocar nada con el flag apagado")
	}
}

// Un supervisor sin árbol no genera avisos.
func TestEscalation_ArbolVacioNoAvisa(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo := &fakeSupervisorListRepo{supervisors: []models.User{{
		ID: supervisorID, IsManager: true, IsSupervisor: true, IsActive: true,
		EmpleadorID: uintPtr(supCompanyID),
	}}}
	empRepo := &fakeTreeEmploymentRepo{descendants: map[[2]uint][]uint{}}
	whRepo := &fakeEscalationWHRepo{pending: 7, canEscalate: true}
	notif := &fakeNotifSvc{}
	w := NewSupervisorEscalationWatcher(userRepo, empRepo, whRepo, notif)

	if n, err := w.RunOnce(); err != nil || n != 0 {
		t.Fatalf("sin árbol no hay a quién escalarle, got n=%d err=%v", n, err)
	}
	if len(notif.notified) != 0 {
		t.Fatal("no debe notificarse")
	}
}

// Un fallo con un supervisor no puede tumbar la corrida de los demás.
func TestEscalation_UnFalloNoTumbaLaCorrida(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo := &fakeSupervisorListRepo{supervisors: []models.User{
		{ID: supervisorID, IsManager: true, IsSupervisor: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID)},
		{ID: 999, IsManager: true, IsSupervisor: true, IsActive: true, EmpleadorID: uintPtr(supCompanyID)},
	}}
	_, empRepo := supervisorSetup()
	whRepo := &fakeEscalationWHRepo{pending: 3, canEscalate: true, findErr: errors.New("db caída")}
	notif := &fakeNotifSvc{}
	w := NewSupervisorEscalationWatcher(userRepo, empRepo, whRepo, notif)

	if _, err := w.RunOnce(); err != nil {
		t.Fatalf("los fallos por supervisor se registran y se sigue, got %v", err)
	}
	if len(notif.notified) != 0 {
		t.Fatal("si no se pudo contar, no se avisa")
	}
}

// Un supervisor sin empresa activa se omite en vez de romper la corrida.
func TestEscalation_SinEmpresaSeOmite(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo := &fakeSupervisorListRepo{supervisors: []models.User{{
		ID: supervisorID, IsManager: true, IsSupervisor: true, IsActive: true,
	}}}
	_, empRepo := supervisorSetup()
	whRepo := &fakeEscalationWHRepo{pending: 7, canEscalate: true}
	notif := &fakeNotifSvc{}
	w := NewSupervisorEscalationWatcher(userRepo, empRepo, whRepo, notif)

	if n, err := w.RunOnce(); err != nil || n != 0 {
		t.Fatalf("sin empresa no hay árbol que mirar, got n=%d err=%v", n, err)
	}
}
