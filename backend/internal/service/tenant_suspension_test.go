package service

import (
	"errors"
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Suspender una empresa tiene que cortar el acceso en el acto, no en la
// siguiente entrada: el portero del login solo mira a quien vuelve a entrar, así
// que sin subir token_version quien ya tenía sesión seguía trabajando. Estos
// tests fijan ese contrato en adminService.SetTenantStatus.
//
// Mismo patrón que manager_flow_test.go: cada fake embebe la interfaz real y
// sobrescribe SOLO lo que toca el camino bajo prueba.

type fakeTenantUserRepo struct {
	repository.UserRepository

	users map[uint]*models.User

	updates       map[uint]map[string]interface{}
	revokedFor    []uint
	revokeErr     error
	revokedBefore bool // ¿se revocó ANTES de escribir la empresa?
	companyWrote  bool
}

func (f *fakeTenantUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeTenantUserRepo) Update(user *models.User, updates map[string]interface{}) error {
	if f.updates == nil {
		f.updates = map[uint]map[string]interface{}{}
	}
	f.updates[user.ID] = updates
	f.companyWrote = true
	return nil
}

func (f *fakeTenantUserRepo) RevokeSessionsByEmployer(employerID uint) (int64, error) {
	if f.revokeErr != nil {
		return 0, f.revokeErr
	}
	if !f.companyWrote {
		f.revokedBefore = true
	}
	f.revokedFor = append(f.revokedFor, employerID)
	return 3, nil
}

type fakeTenantAdminRepo struct {
	repository.AdminRepository
	events []*models.CompanyEvent
}

func (f *fakeTenantAdminRepo) CreateCompanyEvent(event *models.CompanyEvent) error {
	f.events = append(f.events, event)
	return nil
}

func newTenantSvc(company *models.User) (*adminService, *fakeTenantUserRepo, *fakeTenantAdminRepo) {
	userRepo := &fakeTenantUserRepo{users: map[uint]*models.User{company.ID: company}}
	adminRepo := &fakeTenantAdminRepo{}
	return &adminService{repo: adminRepo, userRepo: userRepo}, userRepo, adminRepo
}

func TestSetTenantStatus_SuspendRevokesLiveSessions(t *testing.T) {
	company := &models.User{ID: 7, UserType: models.UserTypeEmployer, IsActive: true, TokenVersion: 4}
	svc, userRepo, adminRepo := newTenantSvc(company)

	if _, err := svc.SetTenantStatus(7, false, 1); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	updates := userRepo.updates[7]
	if updates["is_active"] != false {
		t.Fatalf("is_active: want false, got %v", updates["is_active"])
	}
	// Sin este bump, el propio empleador seguiría dentro con su token vigente.
	if updates["token_version"] != 5 {
		t.Fatalf("token_version: want 5, got %v", updates["token_version"])
	}
	if len(userRepo.revokedFor) != 1 || userRepo.revokedFor[0] != 7 {
		t.Fatalf("se esperaba revocar las sesiones de la empresa 7, got %v", userRepo.revokedFor)
	}
	if len(adminRepo.events) != 1 || adminRepo.events[0].Type != models.CompanyEventSuspended {
		t.Fatalf("expediente: se esperaba un evento de suspensión, got %v", adminRepo.events)
	}
}

// Si no se pueden revocar las sesiones de la plantilla, la empresa NO se
// suspende: mejor un error que una empresa suspendida con todos dentro.
func TestSetTenantStatus_SuspendAbortsWhenRevokeFails(t *testing.T) {
	company := &models.User{ID: 7, UserType: models.UserTypeEmployer, IsActive: true, TokenVersion: 4}
	svc, userRepo, adminRepo := newTenantSvc(company)
	userRepo.revokeErr = errors.New("db caída")

	if _, err := svc.SetTenantStatus(7, false, 1); err == nil {
		t.Fatal("se esperaba error al fallar la revocación")
	}
	if userRepo.companyWrote {
		t.Fatal("la empresa no debería quedar suspendida si la revocación falló")
	}
	if len(adminRepo.events) != 0 {
		t.Fatalf("no debería registrarse el hito: %v", adminRepo.events)
	}
}

func TestSetTenantStatus_SuspendRevokesBeforeWritingCompany(t *testing.T) {
	company := &models.User{ID: 7, UserType: models.UserTypeEmployer, IsActive: true, TokenVersion: 4}
	svc, userRepo, _ := newTenantSvc(company)

	if _, err := svc.SetTenantStatus(7, false, 1); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if !userRepo.revokedBefore {
		t.Fatal("la revocación debe ir antes de suspender, para no dejar un estado a medias")
	}
}

// Reactivar no toca sesiones: nadie tiene una viva y subir la versión solo
// obligaría a un login extra sin motivo.
func TestSetTenantStatus_ActivateLeavesSessionsAlone(t *testing.T) {
	company := &models.User{ID: 7, UserType: models.UserTypeEmployer, IsActive: false, TokenVersion: 4}
	svc, userRepo, adminRepo := newTenantSvc(company)

	if _, err := svc.SetTenantStatus(7, true, 1); err != nil {
		t.Fatalf("activate: %v", err)
	}

	updates := userRepo.updates[7]
	if updates["is_active"] != true {
		t.Fatalf("is_active: want true, got %v", updates["is_active"])
	}
	if _, bumped := updates["token_version"]; bumped {
		t.Fatal("reactivar no debería subir token_version")
	}
	if len(userRepo.revokedFor) != 0 {
		t.Fatalf("reactivar no debería revocar sesiones, got %v", userRepo.revokedFor)
	}
	if len(adminRepo.events) != 1 || adminRepo.events[0].Type != models.CompanyEventReactivated {
		t.Fatalf("expediente: se esperaba un evento de reactivación, got %v", adminRepo.events)
	}
}

// Un usuario que no es empresa nunca debe pasar por este camino.
func TestSetTenantStatus_RejectsNonEmployer(t *testing.T) {
	pro := &models.User{ID: 7, UserType: models.UserTypeProfessional, IsActive: true}
	svc, userRepo, _ := newTenantSvc(pro)

	if _, err := svc.SetTenantStatus(7, false, 1); err == nil {
		t.Fatal("se esperaba error al suspender a un no-empleador")
	}
	if userRepo.companyWrote || len(userRepo.revokedFor) != 0 {
		t.Fatal("no debería tocarse nada para un usuario que no es empresa")
	}
}
