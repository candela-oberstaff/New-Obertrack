package service

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Estos tests fijan la regla que reemplazó a la vieja defensa "un manager no
// puede tener manager": ahora la cadena de mando admite tantos niveles como
// haga falta y lo único que se rechaza es cerrar un círculo.
//
// Mismo patrón que manager_flow_test.go: fakes que solo implementan los métodos
// que el camino bajo prueba toca.

const cycleCompanyID = uint(5)

// chainRepos arma una empresa donde chain[sub] = manager de sub. Devuelve los
// repos con los employments y usuarios necesarios para recorrer la cadena.
func chainRepos(chain map[uint]uint, ids ...uint) (*fakeUserRepo, *fakeEmploymentRepo) {
	users := map[uint]*models.User{}
	active := map[[2]uint]*models.Employment{}
	empID := uint(100)
	for _, id := range ids {
		u := &models.User{
			ID:          id,
			UserType:    models.UserTypeProfessional,
			IsManager:   true,
			IsActive:    true,
			EmpleadorID: uintPtr(cycleCompanyID),
		}
		emp := &models.Employment{ID: empID, UserID: id, CompanyID: cycleCompanyID, Status: models.EmploymentActive}
		if mgr, ok := chain[id]; ok {
			u.ManagerID = uintPtr(mgr)
			emp.ManagerID = uintPtr(mgr)
		}
		users[id] = u
		active[[2]uint{id, cycleCompanyID}] = emp
		empID++
	}
	return &fakeUserRepo{getByID: users}, &fakeEmploymentRepo{activeByKey: active}
}

// Organigrama de tres niveles: 3 reporta a 2 y 2 reporta a 1. Colgar a alguien
// nuevo (4) de 3 es válido aunque 3 ya sea manager con manager propio.
func TestManagerCycle_MultiLevelChainAllowed(t *testing.T) {
	userRepo, empRepo := chainRepos(map[uint]uint{3: 2, 2: 1}, 1, 2, 3, 4)

	if err := ensureNoManagerCycle(userRepo, empRepo, 3, 4, cycleCompanyID); err != nil {
		t.Fatalf("una cadena de varios niveles debe permitirse, got: %v", err)
	}
}

// Ciclo directo: 2 ya reporta a 1, así que poner a 1 bajo 2 los deja siendo
// manager uno del otro.
func TestManagerCycle_DirectCycleRejected(t *testing.T) {
	userRepo, empRepo := chainRepos(map[uint]uint{2: 1}, 1, 2)

	err := ensureNoManagerCycle(userRepo, empRepo, 2, 1, cycleCompanyID)
	if err == nil {
		t.Fatal("asignar a 1 bajo su propio subordinado 2 debe rechazarse")
	}
	if !strings.Contains(err.Error(), "ciclo") {
		t.Fatalf("expected mensaje de ciclo, got: %v", err)
	}
}

// Ciclo indirecto: 3 → 2 → 1. Colgar a 1 de 3 cierra el triángulo.
func TestManagerCycle_IndirectCycleRejected(t *testing.T) {
	userRepo, empRepo := chainRepos(map[uint]uint{3: 2, 2: 1}, 1, 2, 3)

	if err := ensureNoManagerCycle(userRepo, empRepo, 3, 1, cycleCompanyID); err == nil {
		t.Fatal("el ciclo indirecto 1→3→2→1 debe rechazarse")
	}
}

func TestManagerCycle_SelfRejected(t *testing.T) {
	userRepo, empRepo := chainRepos(nil, 1)

	if err := ensureNoManagerCycle(userRepo, empRepo, 1, 1, cycleCompanyID); err == nil {
		t.Fatal("nadie puede ser su propio manager")
	}
}

// Alta de un profesional que todavía no existe: no puede estar en ninguna
// cadena, así que el guard no debe estorbar.
func TestManagerCycle_NewSubordinateSkipsCheck(t *testing.T) {
	userRepo, empRepo := chainRepos(map[uint]uint{2: 1}, 1, 2)

	if err := ensureNoManagerCycle(userRepo, empRepo, 2, 0, cycleCompanyID); err != nil {
		t.Fatalf("con subordinado 0 (alta) no hay ciclo posible, got: %v", err)
	}
}

// El guard compartido sigue validando lo de antes además del ciclo.
func TestEnsureValidManager_StillChecksRoleAndStatus(t *testing.T) {
	userRepo, empRepo := chainRepos(nil, 1, 2)

	notManager := &models.User{ID: 9, UserType: models.UserTypeProfessional, IsManager: false, IsActive: true}
	if err := ensureValidManager(userRepo, empRepo, notManager, 2, cycleCompanyID); err == nil {
		t.Fatal("un usuario que no es manager no puede recibir reportes")
	}

	inactive := &models.User{ID: 9, UserType: models.UserTypeProfessional, IsManager: true, IsActive: false}
	if err := ensureValidManager(userRepo, empRepo, inactive, 2, cycleCompanyID); err == nil {
		t.Fatal("un manager inactivo no puede recibir reportes")
	}
}

// ---------------------------------------------------------------------------
// Camino masivo (el que usa el import)
// ---------------------------------------------------------------------------

// Antes, BulkAssignManager omitía en silencio cualquier fila cuyo profesional ya
// fuera manager. Ahora esa asignación debe concretarse.
func TestBulkAssignManager_ManagerUnderManagerNowAllowed(t *testing.T) {
	userRepo, empRepo := chainRepos(nil, 1, 2)
	// 2 es manager de su propio equipo y ahora pasa a reportarle a 1.
	s := &adminService{userRepo: userRepo, employmentRepo: empRepo}

	mgrID := uint(1)
	assigned, skipped, err := s.BulkAssignManager([]uint{2}, &mgrID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assigned != 1 || skipped != 0 {
		t.Fatalf("un manager debe poder colgarse de otro; assigned=%d skipped=%d", assigned, skipped)
	}
	if got := userRepo.updates[2]["manager_id"]; got == nil {
		t.Fatal("debe persistirse users.manager_id del subordinado")
	}
}

// ...pero el ciclo se sigue frenando, ahora en el guard compartido.
func TestBulkAssignManager_CycleSkipped(t *testing.T) {
	userRepo, empRepo := chainRepos(map[uint]uint{2: 1}, 1, 2)
	s := &adminService{userRepo: userRepo, employmentRepo: empRepo}

	mgrID := uint(2)
	assigned, skipped, err := s.BulkAssignManager([]uint{1}, &mgrID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assigned != 0 || skipped != 1 {
		t.Fatalf("el ciclo debe omitirse; assigned=%d skipped=%d", assigned, skipped)
	}
	if len(userRepo.updates) != 0 {
		t.Fatal("una asignación que forma ciclo NO debe persistirse")
	}
}
