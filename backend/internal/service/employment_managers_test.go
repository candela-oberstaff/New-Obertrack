package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Fakes dedicados para los endpoints del CONJUNTO de managers por empleo (Fase 3).
// Embeben la interfaz real y solo implementan lo que ejercita employmentService.

type emAddCall struct {
	emp, mgr uint
	primary  bool
}
type emCall struct{ emp, mgr uint }

type fakeEMRepo struct {
	repository.EmploymentRepository
	byUser map[uint][]models.Employment       // ListByUser
	links  map[uint][]models.EmploymentManager // ListEmploymentManagers (estado simulado)
	active map[[2]uint]bool                    // (managerID, companyID) -> tiene empleo activo

	added   []emAddCall
	removed []emCall
	setPrim []emCall
	empUpd  []map[string]interface{}
}

func (f *fakeEMRepo) ListByUser(userID uint) ([]models.Employment, error) { return f.byUser[userID], nil }
func (f *fakeEMRepo) ListEmploymentManagers(empID uint) ([]models.EmploymentManager, error) {
	return f.links[empID], nil
}
func (f *fakeEMRepo) GetActive(userID, companyID uint) (*models.Employment, error) {
	if f.active[[2]uint{userID, companyID}] {
		return &models.Employment{UserID: userID, CompanyID: companyID, Status: models.EmploymentActive}, nil
	}
	return nil, errors.New("no active employment")
}
func (f *fakeEMRepo) AddManager(emp, mgr uint, primary bool) error {
	f.added = append(f.added, emAddCall{emp, mgr, primary})
	return nil
}
func (f *fakeEMRepo) RemoveManager(emp, mgr uint) error {
	f.removed = append(f.removed, emCall{emp, mgr})
	return nil
}
func (f *fakeEMRepo) SetPrimaryManager(emp, mgr uint) error {
	f.setPrim = append(f.setPrim, emCall{emp, mgr})
	return nil
}
func (f *fakeEMRepo) Update(_ *models.Employment, updates map[string]interface{}) error {
	f.empUpd = append(f.empUpd, updates)
	return nil
}

type fakeEMUserRepo struct {
	repository.UserRepository
	users   map[uint]*models.User
	userUpd []map[string]interface{}
}

func (f *fakeEMUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeEMUserRepo) Update(_ *models.User, updates map[string]interface{}) error {
	f.userUpd = append(f.userUpd, updates)
	return nil
}

func newEMSvc(emp *fakeEMRepo, usr *fakeEMUserRepo) *employmentService {
	return &employmentService{repo: emp, userRepo: usr}
}

// --- AddEmploymentManager ---

func TestPhase3_AddManager_SelfRejected(t *testing.T) {
	emp := &fakeEMRepo{byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5}}}}
	s := newEMSvc(emp, &fakeEMUserRepo{users: map[uint]*models.User{}})
	err := s.AddEmploymentManager(10, 1, 10)
	if err == nil || !strings.Contains(err.Error(), "Manager inválido") {
		t.Fatalf("un profesional no puede ser su propio manager, got: %v", err)
	}
	if len(emp.added) != 0 {
		t.Fatal("no debe agregar nada al rechazar")
	}
}

func TestPhase3_AddManager_NotManagerRejected(t *testing.T) {
	emp := &fakeEMRepo{byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5}}}}
	usr := &fakeEMUserRepo{users: map[uint]*models.User{20: {ID: 20, IsManager: false, IsActive: true}}}
	err := newEMSvc(emp, usr).AddEmploymentManager(10, 1, 20)
	if err == nil || !strings.Contains(err.Error(), "Manager inválido") {
		t.Fatalf("agregar a un no-manager debe rechazarse, got: %v", err)
	}
	if len(emp.added) != 0 {
		t.Fatal("no debe agregar")
	}
}

func TestPhase3_AddManager_ValidAddsNonPrimary(t *testing.T) {
	emp := &fakeEMRepo{
		byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5, ManagerID: uintPtr(20)}}},
		active: map[[2]uint]bool{{21, 5}: true}, // el manager 21 tiene empleo activo en la empresa 5
	}
	usr := &fakeEMUserRepo{users: map[uint]*models.User{21: {ID: 21, IsManager: true, IsActive: true}}}
	if err := newEMSvc(emp, usr).AddEmploymentManager(10, 1, 21); err != nil {
		t.Fatalf("agregar un manager válido debe pasar: %v", err)
	}
	if len(emp.added) != 1 || emp.added[0].mgr != 21 || emp.added[0].primary {
		t.Fatalf("se esperaba AddManager(1,21,false), got %+v", emp.added)
	}
	if len(emp.empUpd) != 0 {
		t.Fatal("agregar un manager ADICIONAL no debe tocar el espejo employments.manager_id")
	}
}

// --- SetPrimaryEmploymentManager ---

func TestPhase3_SetPrimary_NotAssignedRejected(t *testing.T) {
	emp := &fakeEMRepo{
		byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5, ManagerID: uintPtr(20)}}},
		links:  map[uint][]models.EmploymentManager{1: {{ManagerID: 20, IsPrimary: true}}},
	}
	err := newEMSvc(emp, &fakeEMUserRepo{users: map[uint]*models.User{}}).SetPrimaryEmploymentManager(10, 1, 99)
	if err == nil || !strings.Contains(err.Error(), "no está asignado") {
		t.Fatalf("marcar principal a un manager no asignado debe rechazarse, got: %v", err)
	}
}

func TestPhase3_SetPrimary_ValidMirrors(t *testing.T) {
	emp := &fakeEMRepo{
		byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5, ManagerID: uintPtr(20)}}},
		links:  map[uint][]models.EmploymentManager{1: {{ManagerID: 20, IsPrimary: true}, {ManagerID: 21}}},
	}
	usr := &fakeEMUserRepo{users: map[uint]*models.User{10: {ID: 10, EmpleadorID: uintPtr(5)}}} // empresa activa = 5
	if err := newEMSvc(emp, usr).SetPrimaryEmploymentManager(10, 1, 21); err != nil {
		t.Fatalf("%v", err)
	}
	if len(emp.setPrim) != 1 || emp.setPrim[0].mgr != 21 {
		t.Fatalf("se esperaba SetPrimaryManager(1,21), got %+v", emp.setPrim)
	}
	if len(emp.empUpd) != 1 {
		t.Fatalf("debe espejar employments.manager_id, got %+v", emp.empUpd)
	}
	if len(usr.userUpd) != 1 {
		t.Fatal("debe espejar users.manager_id porque es la empresa activa")
	}
}

// --- RemoveEmploymentManager ---

func TestPhase3_RemovePrimary_AutoPromotes(t *testing.T) {
	emp := &fakeEMRepo{
		byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5, ManagerID: uintPtr(20)}}},
		links:  map[uint][]models.EmploymentManager{1: {{ManagerID: 21}}}, // restante tras quitar al principal 20
	}
	usr := &fakeEMUserRepo{users: map[uint]*models.User{10: {ID: 10, EmpleadorID: uintPtr(5)}}}
	if err := newEMSvc(emp, usr).RemoveEmploymentManager(10, 1, 20); err != nil {
		t.Fatalf("%v", err)
	}
	if len(emp.removed) != 1 || emp.removed[0].mgr != 20 {
		t.Fatalf("se esperaba RemoveManager(1,20), got %+v", emp.removed)
	}
	if len(emp.setPrim) != 1 || emp.setPrim[0].mgr != 21 {
		t.Fatalf("debe auto-promover a 21 como principal, got %+v", emp.setPrim)
	}
	if len(emp.empUpd) != 1 {
		t.Fatal("debe espejar el nuevo principal en employments.manager_id")
	}
}

func TestPhase3_RemoveNonPrimary_LeavesMirror(t *testing.T) {
	emp := &fakeEMRepo{
		byUser: map[uint][]models.Employment{10: {{ID: 1, UserID: 10, CompanyID: 5, ManagerID: uintPtr(20)}}},
	}
	usr := &fakeEMUserRepo{users: map[uint]*models.User{}}
	if err := newEMSvc(emp, usr).RemoveEmploymentManager(10, 1, 21); err != nil { // 21 != principal (20)
		t.Fatalf("%v", err)
	}
	if len(emp.removed) != 1 {
		t.Fatal("debe quitar el vínculo")
	}
	if len(emp.empUpd) != 0 || len(emp.setPrim) != 0 {
		t.Fatal("quitar un manager NO-principal no debe tocar el espejo ni re-promover")
	}
}
