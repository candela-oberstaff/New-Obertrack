package service

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// El supervisor es un manager con alcance ampliado, así que los dos flags nunca
// se separan. Estos tests fijan esa invariante en los tres caminos que la puede
// romper: el alta, la edición (que es también la del empleador vía
// UpdateUserScoped) y el promote/demote.
//
// Reutilizan los fakes de manager_flow_test.go, que solo implementan los
// métodos que toca el camino bajo prueba.

const supervisorCompanyID = uint(5)

// fakeUserRepo (manager_flow_test.go) no implementa Create porque ningún camino
// de allí da de alta un usuario; el del supervisor sí, así que se registra aquí
// reusando la misma captura que Save.
func (f *fakeUserRepo) Create(user *models.User) error {
	f.saved = append(f.saved, user)
	return nil
}

func profesionalConFlags(id uint, isManager, isSupervisor bool) *models.User {
	return &models.User{
		ID:           id,
		UserType:     models.UserTypeProfessional,
		IsManager:    isManager,
		IsSupervisor: isSupervisor,
		IsActive:     true,
		EmpleadorID:  uintPtr(supervisorCompanyID),
	}
}

// Marcar supervisor implica manager: el alcance ampliado se apoya en todo lo
// que ya cuelga de is_manager, así que no puede quedar uno sin el otro.
func TestUpdateUser_SupervisorImplicaManager(t *testing.T) {
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: profesionalConFlags(7, false, false)}}
	s := &adminService{userRepo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	if _, err := s.UpdateUser(7, map[string]interface{}{"is_supervisor": true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upd := userRepo.updates[7]
	if v, _ := upd["is_manager"].(bool); !v {
		t.Fatalf("marcar supervisor debe implicar is_manager=true, got %v", upd)
	}
	if v, _ := upd["is_supervisor"].(bool); !v {
		t.Fatalf("is_supervisor debe persistirse, got %v", upd)
	}
}

// Al revés: quitar el rol de manager se lleva la supervisión, porque un
// supervisor que ya no es manager no podría aprobar nada de su árbol.
func TestUpdateUser_QuitarManagerQuitaSupervisor(t *testing.T) {
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: profesionalConFlags(7, true, true)}}
	// Sin equipo a cargo: si no, el guard de "reasigna su equipo primero" corta
	// antes y no llegaríamos a comprobar la invariante.
	s := &adminService{userRepo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	if _, err := s.UpdateUser(7, map[string]interface{}{"is_manager": false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upd := userRepo.updates[7]
	v, ok := upd["is_supervisor"].(bool)
	if !ok || v {
		t.Fatalf("quitar manager debe limpiar is_supervisor, got %v", upd)
	}
}

// Una petición contradictoria se resuelve a favor del rol más alto en vez de
// dejar un estado imposible (supervisor que no es manager).
func TestUpdateUser_SupervisorGanaAlManagerFalse(t *testing.T) {
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{7: profesionalConFlags(7, false, false)}}
	s := &adminService{userRepo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	if _, err := s.UpdateUser(7, map[string]interface{}{"is_supervisor": true, "is_manager": false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upd := userRepo.updates[7]
	if v, _ := upd["is_manager"].(bool); !v {
		t.Fatalf("supervisor debe ganar y dejar is_manager=true, got %v", upd)
	}
	if v, _ := upd["is_supervisor"].(bool); !v {
		t.Fatalf("is_supervisor debe quedar en true, got %v", upd)
	}
}

// El supervisor hereda la restricción de tipo del manager: una cuenta empresa
// no puede serlo. Como marcar supervisor implica manager, el guard que ya
// existía para is_manager cubre el caso.
func TestUpdateUser_SupervisorSoloProfesionalOCustomerSuccess(t *testing.T) {
	empleador := &models.User{ID: 9, UserType: models.UserTypeEmployer, IsActive: true, CompanyName: "ACME"}
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{9: empleador}}
	s := &adminService{userRepo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	_, err := s.UpdateUser(9, map[string]interface{}{"is_supervisor": true})
	if err == nil {
		t.Fatal("una cuenta empresa no puede ser supervisor")
	}
	if !strings.Contains(err.Error(), "solo profesionales o customer success") {
		t.Fatalf("expected mensaje de restricción de tipo, got: %v", err)
	}
	if len(userRepo.updates) != 0 {
		t.Fatal("un supervisor inválido NO debe persistirse")
	}
}

// Mismo criterio en el alta, donde el flag llega en el payload de creación.
func TestCreateUser_SupervisorSoloProfesionalOCustomerSuccess(t *testing.T) {
	userRepo := &fakeUserRepo{}
	s := &adminService{userRepo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	_, err := s.CreateUser(map[string]interface{}{
		"name":          "ACME",
		"email":         "acme@example.com",
		"password":      "secret",
		"user_type":     string(models.UserTypeEmployer),
		"company_name":  "ACME",
		"is_supervisor": true,
	})
	if err == nil {
		t.Fatal("no se puede crear una cuenta empresa marcada como supervisor")
	}
	if !strings.Contains(err.Error(), "Supervisor inválido") {
		t.Fatalf("expected error de supervisor inválido, got: %v", err)
	}
}

// El alta de un profesional supervisor deja los dos flags puestos.
func TestCreateUser_SupervisorImplicaManager(t *testing.T) {
	userRepo := &fakeUserRepo{}
	s := &adminService{userRepo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	user, err := s.CreateUser(map[string]interface{}{
		"name":          "Ana",
		"email":         "ana@example.com",
		"password":      "secret",
		"user_type":     string(models.UserTypeProfessional),
		"is_supervisor": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !user.IsSupervisor || !user.IsManager {
		t.Fatalf("el alta de un supervisor debe dejar los dos flags, got manager=%v supervisor=%v",
			user.IsManager, user.IsSupervisor)
	}
}

// El organigrama asciende a supervisor a quien recibe un manager, y lo hace por
// este camino: is_supervisor viaja solo, sin mandar is_manager.
func TestPromoteToManager_MarcarSupervisorImplicaManager(t *testing.T) {
	target := profesionalConFlags(6, false, false)
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: target}}
	s := &userService{repo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	sup := true
	got, err := s.PromoteToManager(6, 1, supervisorCompanyID, "superadmin", false, true, nil, &sup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upd := userRepo.updates[6]
	if v, _ := upd["is_manager"].(bool); !v {
		t.Fatalf("marcar supervisor debe implicar is_manager=true, got %v", upd)
	}
	if v, _ := upd["is_supervisor"].(bool); !v {
		t.Fatalf("debe persistirse is_supervisor=true, got %v", upd)
	}
	if !got.IsManager || !got.IsSupervisor {
		t.Fatalf("el usuario devuelto debe traer los dos flags, got manager=%v supervisor=%v",
			got.IsManager, got.IsSupervisor)
	}
}

// Promover a manager sin decir nada del nivel de supervisor no se lo quita a
// quien ya lo era: los flags son independientes salvo por la invariante.
func TestPromoteToManager_NoPisaElSupervisorExistente(t *testing.T) {
	target := profesionalConFlags(6, true, true)
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: target}}
	s := &userService{repo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	mgr := true
	got, err := s.PromoteToManager(6, 1, supervisorCompanyID, "superadmin", false, true, &mgr, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, touched := userRepo.updates[6]["is_supervisor"]; touched {
		t.Fatalf("no debe tocarse is_supervisor si no se pidió, got %v", userRepo.updates[6])
	}
	if !got.IsSupervisor {
		t.Fatal("el usuario debe seguir siendo supervisor")
	}
}

// El camino corto de promover/degradar (PUT /users/:id/promote) también respeta
// la invariante al quitar el rol.
func TestPromoteToManager_DemoteLimpiaSupervisor(t *testing.T) {
	mgr := profesionalConFlags(6, true, true)
	userRepo := &fakeUserRepo{getByID: map[uint]*models.User{6: mgr}}
	s := &userService{repo: userRepo, employmentRepo: &fakeEmploymentRepo{}}

	desired := false
	got, err := s.PromoteToManager(6, 1, supervisorCompanyID, "superadmin", false, true, &desired, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v, ok := userRepo.updates[6]["is_supervisor"].(bool); !ok || v {
		t.Fatalf("degradar a manager debe limpiar is_supervisor, got %v", userRepo.updates[6])
	}
	if got.IsSupervisor {
		t.Fatal("el usuario devuelto no debe seguir marcado como supervisor")
	}
}
