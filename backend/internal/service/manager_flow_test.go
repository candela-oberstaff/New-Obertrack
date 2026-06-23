package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// These tests pin down the manager lifecycle rules in userService:
// promote/demote, assign-to-manager, toggle-status, delete and reassign-team.
// Following the multitenant_scoping_test.go pattern, each fake repo embeds the
// real repository interface (so it satisfies it) and overrides ONLY the methods
// the path under test invokes; every other method stays nil from the embed.
//
// All tests run as superadmin (isSuperadmin=true) so authorizeAdminAction /
// authorizeUserTenant short-circuit and we isolate the manager logic itself.

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeUserRepo struct {
	repository.UserRepository

	// inyección de respuestas
	getByID          map[uint]*models.User
	getErr           error
	reportsByManager int64 // users.manager_id = X (relación canónica)
	reportsErr       error

	// captura de llamadas
	updates        map[uint]map[string]interface{} // por user.ID
	saved          []*models.User
	reassignOld    uint
	reassignNew    *uint
	reassignCalled bool
	reassignCount  int64
	reassignErr    error
	deletedID      uint
	deleteCalled   bool
}

func (f *fakeUserRepo) GetByID(id uint) (*models.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if u, ok := f.getByID[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeUserRepo) Update(user *models.User, updates map[string]interface{}) error {
	if f.updates == nil {
		f.updates = map[uint]map[string]interface{}{}
	}
	f.updates[user.ID] = updates
	return nil
}

func (f *fakeUserRepo) Save(user *models.User) error {
	f.saved = append(f.saved, user)
	return nil
}

func (f *fakeUserRepo) CountReportsByManager(managerID uint) (int64, error) {
	return f.reportsByManager, f.reportsErr
}

func (f *fakeUserRepo) Delete(id uint) error {
	f.deletedID = id
	f.deleteCalled = true
	return nil
}

func (f *fakeUserRepo) ReassignManager(oldManagerID uint, newManagerID *uint, _ uint) (int64, error) {
	f.reassignOld = oldManagerID
	f.reassignNew = newManagerID
	f.reassignCalled = true
	return f.reassignCount, f.reassignErr
}

type fakeEmploymentRepo struct {
	repository.EmploymentRepository

	// inyección
	countByManager int64
	countErr       error
	activeByKey    map[[2]uint]*models.Employment // (userID, companyID)
	activeErr      error

	// captura
	countCalled    bool
	reassignOld    uint
	reassignNew    *uint
	reassignCalled bool
	reassignCount  int64
	reassignErr    error
	updatedEmps    []map[string]interface{}

	// captura del dual-write (employment_managers)
	primaryManager      [][2]uint // (employmentID, managerID) marcados como principal
	clearedManagers     []uint    // employmentIDs con vínculos limpiados
	linksReassignCalled bool
	linksReassignOld    uint
	linksReassignNew    *uint

	// --- Lecturas via-links (FASE 2) ---
	// isManagerOf[(userID, companyID)] = managerIDs con vínculo vivo.
	isManagerOf      map[[2]uint][]uint
	isManagerOfErr   error
	countViaLinks    int64 // CountActiveByManagerViaLinks
	countViaLinksErr error
	// countInCompanyViaLinks[(managerID, companyID)] = conteo via-links acotado.
	countInCompanyViaLinks    map[[2]uint]int64
	countInCompanyViaLinksErr error
	// managerIDs[(userID, companyID)] = managers a notificar (ListManagerIDs).
	managerIDs    map[[2]uint][]uint
	managerIDsErr error
}

func (f *fakeEmploymentRepo) IsManagerOf(userID, companyID, managerID uint) (bool, error) {
	if f.isManagerOfErr != nil {
		return false, f.isManagerOfErr
	}
	for _, m := range f.isManagerOf[[2]uint{userID, companyID}] {
		if m == managerID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeEmploymentRepo) CountActiveByManagerViaLinks(managerID uint) (int64, error) {
	f.countCalled = true
	return f.countViaLinks, f.countViaLinksErr
}

func (f *fakeEmploymentRepo) CountActiveByManagerInCompanyViaLinks(managerID, companyID uint) (int64, error) {
	if f.countInCompanyViaLinksErr != nil {
		return 0, f.countInCompanyViaLinksErr
	}
	return f.countInCompanyViaLinks[[2]uint{managerID, companyID}], nil
}

func (f *fakeEmploymentRepo) ListManagerIDs(userID, companyID uint) ([]uint, error) {
	if f.managerIDsErr != nil {
		return nil, f.managerIDsErr
	}
	return f.managerIDs[[2]uint{userID, companyID}], nil
}

func (f *fakeEmploymentRepo) CountActiveByManager(managerID uint) (int64, error) {
	f.countCalled = true
	return f.countByManager, f.countErr
}

func (f *fakeEmploymentRepo) GetActive(userID, companyID uint) (*models.Employment, error) {
	if f.activeErr != nil {
		return nil, f.activeErr
	}
	if e, ok := f.activeByKey[[2]uint{userID, companyID}]; ok {
		return e, nil
	}
	return nil, errors.New("no active employment")
}

func (f *fakeEmploymentRepo) Update(employment *models.Employment, updates map[string]interface{}) error {
	f.updatedEmps = append(f.updatedEmps, updates)
	return nil
}

func (f *fakeEmploymentRepo) ReassignManager(oldManagerID uint, newManagerID *uint, _ uint) (int64, error) {
	f.reassignOld = oldManagerID
	f.reassignNew = newManagerID
	f.reassignCalled = true
	return f.reassignCount, f.reassignErr
}

// --- Multi-manager N-a-N (dual-write Fase 1) ---
// El dual-write es aditivo y best-effort: estas implementaciones son no-ops que
// solo registran que el espejo employment_managers fue sincronizado, sin alterar
// las aserciones existentes (que verifican el puntero employments.manager_id).
func (f *fakeEmploymentRepo) SetPrimaryManager(employmentID, managerID uint) error {
	f.primaryManager = append(f.primaryManager, [2]uint{employmentID, managerID})
	return nil
}

func (f *fakeEmploymentRepo) ClearManagers(employmentID uint) error {
	f.clearedManagers = append(f.clearedManagers, employmentID)
	return nil
}

func (f *fakeEmploymentRepo) ReassignManagerLinks(oldManagerID, newManagerID *uint, _ uint) error {
	f.linksReassignCalled = true
	if oldManagerID != nil {
		f.linksReassignOld = *oldManagerID
	}
	f.linksReassignNew = newManagerID
	return nil
}

func boolPtr(b bool) *bool { return &b }
func uintPtr(u uint) *uint { return &u }

// ---------------------------------------------------------------------------
// PromoteToManager
// ---------------------------------------------------------------------------

func TestManagerPromote_ProfessionalSetsIsManager(t *testing.T) {
	pro := &models.User{ID: 7, UserType: models.UserTypeProfessional}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: pro}}
	empRepo := &fakeEmploymentRepo{}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	got, err := s.PromoteToManager(7, 1, 0, "superadmin", false, true, boolPtr(true))
	if err != nil {
		t.Fatalf("promoting a professional should succeed, got error: %v", err)
	}
	if !got.IsManager {
		t.Fatalf("returned user should have IsManager=true, got %+v", got)
	}
	upd, ok := userRepo.updates[7]
	if !ok {
		t.Fatalf("repo.Update should have been called for user 7")
	}
	if v, _ := upd["is_manager"].(bool); !v {
		t.Fatalf("update should set is_manager=true, got %v", upd["is_manager"])
	}
}

func TestManagerPromote_EmployerRejected(t *testing.T) {
	emp := &models.User{ID: 9, UserType: models.UserTypeEmployer}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{9: emp}}
	empRepo := &fakeEmploymentRepo{}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.PromoteToManager(9, 1, 0, "superadmin", false, true, boolPtr(true))
	if err == nil {
		t.Fatalf("promoting an employer to manager must fail")
	}
	if !strings.Contains(err.Error(), "Manager inválido") {
		t.Fatalf("expected 'Manager inválido' error, got: %v", err)
	}
	if _, ok := userRepo.updates[9]; ok {
		t.Fatalf("repo.Update must NOT be called when promotion is rejected")
	}
}

func TestManagerPromote_DemoteWithTeamRejected(t *testing.T) {
	mgr := &models.User{ID: 5, UserType: models.UserTypeProfessional, IsManager: true, Name: "Ana"}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{5: mgr}}
	empRepo := &fakeEmploymentRepo{countByManager: 3} // tiene equipo
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.PromoteToManager(5, 1, 0, "superadmin", false, true, boolPtr(false))
	if err == nil {
		t.Fatalf("demoting a manager with an active team must fail")
	}
	if !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("expected 'a su cargo' error, got: %v", err)
	}
	if !empRepo.countCalled {
		t.Fatalf("CountActiveByManager should have been consulted")
	}
}

func TestManagerPromote_DemoteWithoutTeamOK(t *testing.T) {
	mgr := &models.User{ID: 5, UserType: models.UserTypeProfessional, IsManager: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{5: mgr}}
	empRepo := &fakeEmploymentRepo{countByManager: 0} // sin equipo
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	got, err := s.PromoteToManager(5, 1, 0, "superadmin", false, true, boolPtr(false))
	if err != nil {
		t.Fatalf("demoting a manager with no team should succeed, got: %v", err)
	}
	if got.IsManager {
		t.Fatalf("returned user should have IsManager=false after demote, got %+v", got)
	}
	if v, _ := userRepo.updates[5]["is_manager"].(bool); v {
		t.Fatalf("update should set is_manager=false")
	}
}

// FAIL-CLOSED: si el conteo del equipo falla, NO se puede degradar.
func TestManagerPromote_DemoteCountErrorFailsClosed(t *testing.T) {
	mgr := &models.User{ID: 5, UserType: models.UserTypeProfessional, IsManager: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{5: mgr}}
	empRepo := &fakeEmploymentRepo{countErr: errors.New("db down")}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.PromoteToManager(5, 1, 0, "superadmin", false, true, boolPtr(false))
	if err == nil {
		t.Fatalf("when CountActiveByManager errors, demote MUST fail closed")
	}
	if _, ok := userRepo.updates[5]; ok {
		t.Fatalf("repo.Update must NOT run when the team count failed (fail-closed)")
	}
}

// ---------------------------------------------------------------------------
// AssignToManager
// ---------------------------------------------------------------------------

func TestManagerAssign_SelfRejected(t *testing.T) {
	pro := &models.User{ID: 4, UserType: models.UserTypeProfessional}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: pro}}
	empRepo := &fakeEmploymentRepo{}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.AssignToManager(4, 4, 1, 0, "superadmin", false, true)
	if err == nil {
		t.Fatalf("a professional cannot be their own manager")
	}
	if len(userRepo.saved) != 0 {
		t.Fatalf("nothing should be saved when self-assign is rejected")
	}
}

func TestManagerAssign_TargetNotManagerRejected(t *testing.T) {
	pro := &models.User{ID: 4, UserType: models.UserTypeProfessional}
	notMgr := &models.User{ID: 8, UserType: models.UserTypeProfessional, IsManager: false, IsActive: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: pro, 8: notMgr}}
	empRepo := &fakeEmploymentRepo{}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.AssignToManager(4, 8, 1, 0, "superadmin", false, true)
	if err == nil {
		t.Fatalf("assigning to a non-manager must fail")
	}
	if !strings.Contains(err.Error(), "Manager inválido") {
		t.Fatalf("expected 'Manager inválido' error, got: %v", err)
	}
}

func TestManagerAssign_ValidManagerAssignsAndSyncsEmployment(t *testing.T) {
	companyID := uint(20)
	pro := &models.User{ID: 4, UserType: models.UserTypeProfessional, EmpleadorID: &companyID}
	mgr := &models.User{ID: 8, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: &companyID}
	emp := &models.Employment{UserID: 4, CompanyID: companyID}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{4: pro, 8: mgr}}
	empRepo := &fakeEmploymentRepo{
		activeByKey: map[[2]uint]*models.Employment{
			{8, companyID}: {UserID: 8, CompanyID: companyID}, // ensureValidManager: manager pertenece a la empresa
			{4, companyID}: emp,                               // sync: employment del profesional
		},
	}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	got, err := s.AssignToManager(4, 8, 1, 0, "superadmin", false, true)
	if err != nil {
		t.Fatalf("assigning a valid manager should succeed, got: %v", err)
	}
	if got.ManagerID == nil || *got.ManagerID != 8 {
		t.Fatalf("professional.ManagerID should be 8, got %v", got.ManagerID)
	}
	if len(userRepo.saved) != 1 {
		t.Fatalf("professional should be saved exactly once, got %d", len(userRepo.saved))
	}
	if len(empRepo.updatedEmps) != 1 {
		t.Fatalf("employment should be synced once, got %d updates", len(empRepo.updatedEmps))
	}
	mid, ok := empRepo.updatedEmps[0]["manager_id"].(*uint)
	if !ok || mid == nil || *mid != 8 {
		t.Fatalf("employment sync should set manager_id=8, got %v", empRepo.updatedEmps[0]["manager_id"])
	}
}

// ---------------------------------------------------------------------------
// ToggleStatus
// ---------------------------------------------------------------------------

func TestManagerToggleStatus_DeactivateManagerWithTeamRejected(t *testing.T) {
	mgr := &models.User{ID: 6, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, Name: "Ana"}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: mgr}}
	empRepo := &fakeEmploymentRepo{countByManager: 2}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.ToggleStatus(6, 1, 0, "superadmin", false, true)
	if err == nil {
		t.Fatalf("deactivating a manager that still has a team must fail")
	}
	if !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("expected 'a su cargo' error, got: %v", err)
	}
	if len(userRepo.saved) != 0 {
		t.Fatalf("user must not be saved when deactivation is rejected")
	}
}

// FAIL-CLOSED para ToggleStatus: si el conteo falla, no se desactiva.
func TestManagerToggleStatus_DeactivateCountErrorFailsClosed(t *testing.T) {
	mgr := &models.User{ID: 6, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: mgr}}
	empRepo := &fakeEmploymentRepo{countErr: errors.New("db down")}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.ToggleStatus(6, 1, 0, "superadmin", false, true)
	if err == nil {
		t.Fatalf("when the team count errors, deactivation MUST fail closed")
	}
	if len(userRepo.saved) != 0 {
		t.Fatalf("user must not be saved when team count failed (fail-closed)")
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestManagerDelete_ManagerWithTeamRejected(t *testing.T) {
	mgr := &models.User{ID: 6, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, Name: "Ana"}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: mgr}}
	empRepo := &fakeEmploymentRepo{countByManager: 1}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	err := s.Delete(6, 1, 0, "superadmin", false, true)
	if err == nil {
		t.Fatalf("deleting a manager that still has a team must fail")
	}
	if !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("expected 'a su cargo' error, got: %v", err)
	}
	if userRepo.deleteCalled {
		t.Fatalf("repo.Delete must NOT be called when the manager still has a team")
	}
}

// ---------------------------------------------------------------------------
// ReassignTeam
// ---------------------------------------------------------------------------

func TestManagerReassignTeam_ToValidManager(t *testing.T) {
	oldMgr := &models.User{ID: 6, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(5)}
	newMgr := &models.User{ID: 9, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(5)}
	userRepo := &fakeUserRepo{
		getByID:       map[uint]*models.User{6: oldMgr, 9: newMgr},
		reassignCount: 3,
	}
	empRepo := &fakeEmploymentRepo{
		reassignCount: 3,
		activeByKey:   map[[2]uint]*models.Employment{{9, 5}: {}},
	}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	n, err := s.ReassignTeam(6, uintPtr(9), 1, 0, "superadmin", false, true)
	if err != nil {
		t.Fatalf("reassigning to a valid manager should succeed, got: %v", err)
	}
	if n != 3 {
		t.Fatalf("should return the employment reassign count (3), got %d", n)
	}
	if !empRepo.reassignCalled || empRepo.reassignOld != 6 || empRepo.reassignNew == nil || *empRepo.reassignNew != 9 {
		t.Fatalf("employmentRepo.ReassignManager(6,9) expected, got called=%v old=%d new=%v",
			empRepo.reassignCalled, empRepo.reassignOld, empRepo.reassignNew)
	}
	if !userRepo.reassignCalled || userRepo.reassignOld != 6 || userRepo.reassignNew == nil || *userRepo.reassignNew != 9 {
		t.Fatalf("userRepo.ReassignManager(6,9) expected, got called=%v old=%d new=%v",
			userRepo.reassignCalled, userRepo.reassignOld, userRepo.reassignNew)
	}
}

func TestManagerReassignTeam_NewManagerNotManagerRejected(t *testing.T) {
	oldMgr := &models.User{ID: 6, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(5)}
	notMgr := &models.User{ID: 9, UserType: models.UserTypeProfessional, IsManager: false, IsActive: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: oldMgr, 9: notMgr}}
	empRepo := &fakeEmploymentRepo{}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.ReassignTeam(6, uintPtr(9), 1, 0, "superadmin", false, true)
	if err == nil {
		t.Fatalf("reassigning to a non-manager must fail")
	}
	if !strings.Contains(err.Error(), "Manager inválido") {
		t.Fatalf("expected 'Manager inválido' error, got: %v", err)
	}
	if empRepo.reassignCalled || userRepo.reassignCalled {
		t.Fatalf("no reassign should happen when the new manager is invalid")
	}
}

func TestManagerReassignTeam_NilUnassigns(t *testing.T) {
	oldMgr := &models.User{ID: 6, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true, EmpleadorID: uintPtr(5)}
	userRepo := &fakeUserRepo{
		getByID:       map[uint]*models.User{6: oldMgr},
		reassignCount: 2,
	}
	empRepo := &fakeEmploymentRepo{reassignCount: 2}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	n, err := s.ReassignTeam(6, nil, 1, 0, "superadmin", false, true)
	if err != nil {
		t.Fatalf("unassigning (nil new manager) should succeed, got: %v", err)
	}
	if n != 2 {
		t.Fatalf("should return reassign count (2), got %d", n)
	}
	if !empRepo.reassignCalled || empRepo.reassignNew != nil {
		t.Fatalf("employmentRepo.ReassignManager(6,nil) expected, got called=%v new=%v",
			empRepo.reassignCalled, empRepo.reassignNew)
	}
	if !userRepo.reassignCalled || userRepo.reassignNew != nil {
		t.Fatalf("userRepo.ReassignManager(6,nil) expected, got called=%v new=%v",
			userRepo.reassignCalled, userRepo.reassignNew)
	}
}

// ---------------------------------------------------------------------------
// Camino owner/superadmin (PUT /admin/users -> adminService.UpdateUser)
// ---------------------------------------------------------------------------

// Un owner/superadmin que intenta quitar el rol de manager a alguien que aún
// tiene equipo asignado debe ser BLOQUEADO (no se degrada hasta reasignar).
func TestManagerAdminUpdateUser_DemoteWithTeamRejected(t *testing.T) {
	mgr := &models.User{ID: 7, Name: "Andrés", UserType: models.UserTypeProfessional, IsManager: true, IsActive: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: mgr}}
	empRepo := &fakeEmploymentRepo{countByManager: 2}
	s := &adminService{userRepo: userRepo, employmentRepo: empRepo}

	_, err := s.UpdateUser(7, map[string]interface{}{"is_manager": false})
	if err == nil {
		t.Fatal("quitar el rol de manager con equipo asignado debe bloquearse")
	}
	if !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("expected 'a su cargo' (orphan) error, got: %v", err)
	}
	if len(userRepo.saved) != 0 || len(userRepo.updates) != 0 {
		t.Fatal("el usuario NO debe persistirse cuando la degradación se rechaza")
	}
}

// FAIL-CLOSED en el camino admin: si el conteo de equipo falla, no se degrada.
func TestManagerAdminUpdateUser_DemoteCountErrorFailsClosed(t *testing.T) {
	mgr := &models.User{ID: 7, UserType: models.UserTypeProfessional, IsManager: true, IsActive: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: mgr}}
	empRepo := &fakeEmploymentRepo{countErr: errors.New("db down")}
	s := &adminService{userRepo: userRepo, employmentRepo: empRepo}

	_, err := s.UpdateUser(7, map[string]interface{}{"is_manager": false})
	if err == nil {
		t.Fatal("ante error de conteo, la degradación DEBE fallar cerrada")
	}
	if len(userRepo.saved) != 0 || len(userRepo.updates) != 0 {
		t.Fatal("el usuario NO debe persistirse cuando el conteo falla (fail-closed)")
	}
}

// REGRESIÓN (bug reportado): el equipo está enlazado por users.manager_id pero
// employments aún NO se sincronizó (p.ej. subordinados que nunca iniciaron
// sesión tras el seed). El guard DEBE bloquear igual contando la relación
// canónica users.manager_id, no solo employments.
func TestManagerDemote_EmploymentsEmptyButUsersHaveReports(t *testing.T) {
	// Camino superadmin (admin_service.UpdateUser) — el de la pantalla de detalle.
	mgr := &models.User{ID: 7, Name: "Andrés", UserType: models.UserTypeProfessional, IsManager: true, IsActive: true}
	aUserRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: mgr}, reportsByManager: 3}
	aEmpRepo := &fakeEmploymentRepo{countByManager: 0} // employments sin sincronizar
	admin := &adminService{userRepo: aUserRepo, employmentRepo: aEmpRepo}
	if _, err := admin.UpdateUser(7, map[string]interface{}{"is_manager": false}); err == nil || !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("admin: degradar con reportes en users.manager_id debe bloquearse, got: %v", err)
	}
	if len(aUserRepo.updates) != 0 {
		t.Fatal("admin: no debe persistir la degradación")
	}

	// Camino service PromoteToManager (mismo invariante).
	mgr2 := &models.User{ID: 7, Name: "Andrés", UserType: models.UserTypeProfessional, IsManager: true, IsActive: true}
	uUserRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: mgr2}, reportsByManager: 3}
	uEmpRepo := &fakeEmploymentRepo{countByManager: 0}
	s := &userService{repo: uUserRepo, employmentRepo: uEmpRepo}
	demote := false
	if _, err := s.PromoteToManager(7, 1, 0, "superadmin", false, true, &demote); err == nil || !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("service: degradar con reportes en users.manager_id debe bloquearse, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FASE 2 — Lecturas multi-manager via-links (flag MULTI_MANAGER_READS ON)
// ---------------------------------------------------------------------------
//
// Estos tests fijan el flag con SetMultiManagerReads(true) y lo restauran con
// defer. Demuestran que, con el flag ON, un manager NO-principal (sin puntero
// employments.manager_id pero CON vínculo en employment_managers) obtiene los
// permisos via-links: aprueba jornadas y bloquea su propia degradación.

// fakeApproveWHRepo devuelve las jornadas inyectadas y registra la aprobación.
type fakeApproveWHRepo struct {
	repository.WorkHourRepository
	byID            []models.WorkHour
	approvedIDs     []uint
	approveMultiHit bool
}

func (f *fakeApproveWHRepo) FindManyByIDs(ids []uint) ([]models.WorkHour, error) {
	return f.byID, nil
}
func (f *fakeApproveWHRepo) FindManyByIDsAndTenant(ids []uint, _ uint) ([]models.WorkHour, error) {
	return f.byID, nil
}
func (f *fakeApproveWHRepo) ApproveMultiple(ids []uint, _ uint, _ time.Time) error {
	f.approveMultiHit = true
	f.approvedIDs = ids
	return nil
}
func (f *fakeApproveWHRepo) ApproveMultipleAndTenant(ids []uint, _ uint, _ time.Time, _ uint) error {
	f.approveMultiHit = true
	f.approvedIDs = ids
	return nil
}

// fakeNotifSvc cuenta las notificaciones creadas (no se asierta el contenido).
type fakeNotifSvc struct {
	NotificationService
	notified []uint
}

func (f *fakeNotifSvc) CreateNotification(userID uint, _, _, _ string, _ map[string]interface{}) error {
	f.notified = append(f.notified, userID)
	return nil
}

// Con el flag ON, un segundo manager (vínculo en el fake, NO el principal) puede
// aprobar las jornadas de su subordinado: IsManagerOf=true.
func TestPhase2_SecondaryManagerCanApprove(t *testing.T) {
	SetMultiManagerReads(true)
	defer SetMultiManagerReads(false)

	const employeeID, tenantID, secondaryMgr = uint(4), uint(20), uint(99)
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{UserID: employeeID, TenantID: tenantID},
	}}
	// El empleo tiene a 99 como manager via-links aunque no sea el principal.
	empRepo := &fakeEmploymentRepo{
		isManagerOf: map[[2]uint][]uint{{employeeID, tenantID}: {secondaryMgr}},
	}
	notif := &fakeNotifSvc{}
	s := &workHourService{repo: whRepo, employmentRepo: empRepo, notifSvc: notif, userRepo: &fakeUserRepo{}}

	err := s.Approve([]uint{1}, secondaryMgr, "profesional", false, true, tenantID)
	if err != nil {
		t.Fatalf("un manager secundario (via-links) debe poder aprobar, got: %v", err)
	}
	if !whRepo.approveMultiHit {
		t.Fatal("la aprobación debió persistirse")
	}
}

// Con el flag OFF, ese mismo manager secundario (sin puntero principal) NO puede
// aprobar: comportamiento bit-a-bit actual.
func TestPhase2_SecondaryManagerRejectedWhenFlagOff(t *testing.T) {
	// flag OFF por defecto; explícito para claridad.
	SetMultiManagerReads(false)

	const employeeID, tenantID, secondaryMgr = uint(4), uint(20), uint(99)
	whRepo := &fakeApproveWHRepo{byID: []models.WorkHour{
		{UserID: employeeID, TenantID: tenantID},
	}}
	// activeByKey sin manager_id principal => GetActive devuelve emp con ManagerID nil.
	empRepo := &fakeEmploymentRepo{
		activeByKey: map[[2]uint]*models.Employment{
			{employeeID, tenantID}: {UserID: employeeID, CompanyID: tenantID},
		},
		isManagerOf: map[[2]uint][]uint{{employeeID, tenantID}: {secondaryMgr}},
	}
	s := &workHourService{repo: whRepo, employmentRepo: empRepo, notifSvc: &fakeNotifSvc{}, userRepo: &fakeUserRepo{}}

	err := s.Approve([]uint{1}, secondaryMgr, "profesional", false, true, tenantID)
	if err == nil {
		t.Fatal("con el flag OFF, el manager secundario NO debe poder aprobar")
	}
	if whRepo.approveMultiHit {
		t.Fatal("no debió persistir aprobación cuando el permiso se deniega")
	}
}

// Con el flag ON, CountActiveByManagerViaLinks>0 bloquea degradar a un manager
// aunque el puntero (CountActiveByManager) esté en cero: tiene equipo via-links.
func TestPhase2_ViaLinksTeamBlocksDemote(t *testing.T) {
	SetMultiManagerReads(true)
	defer SetMultiManagerReads(false)

	mgr := &models.User{ID: 5, UserType: models.UserTypeProfessional, IsManager: true}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{5: mgr}}
	// Puntero en cero, pero la tabla N-a-N reporta equipo vivo.
	empRepo := &fakeEmploymentRepo{countByManager: 0, countViaLinks: 2}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	_, err := s.PromoteToManager(5, 1, 0, "superadmin", false, true, boolPtr(false))
	if err == nil {
		t.Fatal("degradar un manager con equipo via-links debe bloquearse")
	}
	if !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("expected 'a su cargo' error, got: %v", err)
	}
	if !empRepo.countCalled {
		t.Fatal("CountActiveByManagerViaLinks debió consultarse")
	}
	if _, ok := userRepo.updates[5]; ok {
		t.Fatal("no debe persistir la degradación cuando hay equipo via-links")
	}
}

// Con el flag ON, EndEmployment usa CountActiveByManagerInCompanyViaLinks: si hay
// equipo via-links en la empresa, no se puede finalizar (fail-closed sobre la tabla).
func TestPhase2_EndEmploymentBlockedByViaLinks(t *testing.T) {
	SetMultiManagerReads(true)
	defer SetMultiManagerReads(false)

	const empID, userID, companyID = uint(50), uint(7), uint(20)
	target := models.Employment{UserID: userID, CompanyID: companyID, Status: models.EmploymentActive}
	target.ID = empID
	empRepo := &fakeEndEmploymentRepo{
		byUser: map[uint][]models.Employment{userID: {target}},
		countInCompanyViaLinks: map[[2]uint]int64{
			{userID, companyID}: 1, // tiene equipo via-links en la empresa
		},
	}
	s := &employmentService{repo: empRepo}

	err := s.EndEmployment(userID, empID, "fin")
	if err == nil {
		t.Fatal("finalizar empleo de un manager con equipo via-links debe bloquearse")
	}
	if !strings.Contains(err.Error(), "a su cargo") {
		t.Fatalf("expected 'a su cargo' error, got: %v", err)
	}
}

// fakeEndEmploymentRepo es un repo mínimo para EndEmployment con conteo via-links.
type fakeEndEmploymentRepo struct {
	repository.EmploymentRepository
	byUser                 map[uint][]models.Employment
	countInCompanyViaLinks map[[2]uint]int64
}

func (f *fakeEndEmploymentRepo) ListByUser(userID uint) ([]models.Employment, error) {
	return f.byUser[userID], nil
}

func (f *fakeEndEmploymentRepo) CountActiveByManagerInCompanyViaLinks(managerID, companyID uint) (int64, error) {
	return f.countInCompanyViaLinks[[2]uint{managerID, companyID}], nil
}
