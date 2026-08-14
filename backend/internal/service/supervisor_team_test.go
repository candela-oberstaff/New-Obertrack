package service

import (
	"errors"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Fase 4: el supervisor gestiona a los suyos (promover/quitar manager, asignar y
// reasignar) sin ser la empresa. Es la ÚNICA guarda del rol que abre algo
// reservado hasta ahora a la cuenta empleador, así que la mitad de este archivo
// son intentos de escalada que tienen que fallar.
//
// El organigrama es el de supervisor_scope_test.go: 7 (supervisor) → 8 (manager)
// → 9 (nieto). 77 cuelga de otra rama y 88 es de otra empresa.

const otraEmpresaID = uint(30)

// teamSetup extiende supervisorSetup con los forasteros que se usan para probar
// que el árbol es de verdad el límite.
func teamSetup() (*fakeUserRepo, *fakeTreeEmploymentRepo) {
	userRepo, empRepo := supervisorSetup()
	// 77: misma empresa, otra rama del organigrama.
	userRepo.getByID[77] = &models.User{
		ID: 77, UserType: models.UserTypeProfessional, IsManager: true,
		IsActive: true, EmpleadorID: uintPtr(supCompanyID),
	}
	// 88: otra empresa.
	userRepo.getByID[88] = &models.User{
		ID: 88, UserType: models.UserTypeProfessional, IsManager: true,
		IsActive: true, EmpleadorID: uintPtr(otraEmpresaID),
	}
	empRepo.employments[[2]uint{77, supCompanyID}] = &models.Employment{
		ID: 177, UserID: 77, CompanyID: supCompanyID, Status: models.EmploymentActive,
	}
	empRepo.employments[[2]uint{88, otraEmpresaID}] = &models.Employment{
		ID: 188, UserID: 88, CompanyID: otraEmpresaID, Status: models.EmploymentActive,
	}
	return userRepo, empRepo
}

// promoteAsSupervisor ejecuta el camino de promoción con la identidad del
// supervisor (no superadmin, no empleador: un profesional con is_manager).
func promoteAsSupervisor(s *userService, targetID uint, desired bool) (*models.User, error) {
	return s.PromoteToManager(targetID, supervisorID, supCompanyID, "profesional", true, false, &desired, nil)
}

// ---------------------------------------------------------------------------
// Camino feliz: dentro del árbol
// ---------------------------------------------------------------------------

func TestSupervisorTeam_PromuevePorDentroDeSuArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	got, err := promoteAsSupervisor(s, grandChildID, true)
	if err != nil {
		t.Fatalf("el supervisor debe poder promover a alguien de su árbol, got: %v", err)
	}
	if !got.IsManager {
		t.Fatal("el usuario devuelto debe quedar como manager")
	}
	if v, _ := userRepo.updates[grandChildID]["is_manager"].(bool); !v {
		t.Fatalf("debe persistirse is_manager=true, got %v", userRepo.updates[grandChildID])
	}
}

func TestSupervisorTeam_AsignaEntreManagersDeSuArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	// El nieto (9) pasa a colgar del manager 8: los dos están en el árbol.
	if _, err := s.AssignToManager(grandChildID, subManagerID, supervisorID, supCompanyID, 0,
		"profesional", true, false); err != nil {
		t.Fatalf("mover gente entre managers suyos debe permitirse, got: %v", err)
	}
}

// Colgarse a alguien de UNO MISMO es la operación más normal del organigrama.
// No salía de los tests con fakes: se descubrió usando la aplicación, porque
// IsDescendantOf dice (con razón) que nadie es descendiente de sí mismo.
func TestSupervisorTeam_PuedeColgarseAAlguienDeSiMismo(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := s.AssignToManager(grandChildID, supervisorID, supervisorID, supCompanyID, 0,
		"profesional", true, false); err != nil {
		t.Fatalf("un supervisor debe poder subirse a alguien de su árbol, got: %v", err)
	}
}

// Y lo mismo al reasignar un equipo entero hacia sí mismo.
func TestSupervisorTeam_PuedeRecibirUnEquipoEntero(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	yo := supervisorID
	if _, err := s.ReassignTeam(subManagerID, &yo, supervisorID, supCompanyID,
		"profesional", true, false); err != nil {
		t.Fatalf("debe poder quedarse con el equipo de uno de sus managers, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Escalada: todo esto tiene que fallar
// ---------------------------------------------------------------------------

// Fuera del árbol no se toca a nadie, aunque sea de la misma empresa.
func TestSupervisorTeam_NoPromueveFueraDeSuArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := promoteAsSupervisor(s, 77, true); err == nil {
		t.Fatal("otra rama del organigrama no es gestionable por este supervisor")
	}
	if len(userRepo.updates) != 0 {
		t.Fatal("una acción denegada NO debe persistir nada")
	}
}

// Ni a sí mismo: IsDescendantOf devuelve false cuando raíz y objetivo coinciden,
// así que un supervisor no puede auto-gestionarse por esta vía.
func TestSupervisorTeam_NoSePromueveASiMismo(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := promoteAsSupervisor(s, supervisorID, true); err == nil {
		t.Fatal("un supervisor no puede gestionarse a sí mismo")
	}
}

// Ni a nadie de otra empresa, aunque por un error de datos apareciera colgando
// de él: el corte de tenant va antes que el del árbol.
func TestSupervisorTeam_NoCruzaDeEmpresa(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	empRepo.descendants[[2]uint{supervisorID, supCompanyID}] = []uint{subManagerID, grandChildID, 88}
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := promoteAsSupervisor(s, 88, true); err == nil {
		t.Fatal("nadie de otra empresa es gestionable, ni estando en el árbol")
	}
}

// El destino de una asignación se autoriza igual que el origen: si no, el
// supervisor podría entregarle su subordinado (y sus horas) a un tercero.
func TestSupervisorTeam_NoAsignaHaciaFueraDeSuArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := s.AssignToManager(grandChildID, 77, supervisorID, supCompanyID, 0,
		"profesional", true, false); err == nil {
		t.Fatal("el manager destino tiene que estar en el árbol del supervisor")
	}
}

// Lo mismo al reasignar un equipo entero.
func TestSupervisorTeam_NoReasignaHaciaFueraDeSuArbol(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	ajeno := uint(77)
	if _, err := s.ReassignTeam(subManagerID, &ajeno, supervisorID, supCompanyID,
		"profesional", true, false); err == nil {
		t.Fatal("no se puede descargar el equipo en un manager de fuera del árbol")
	}
}

// Un manager normal no gana nada: la vía del supervisor exige el flag del
// usuario, no solo ser manager.
func TestSupervisorTeam_ManagerNormalSigueSinPoder(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	desired := true
	if _, err := s.PromoteToManager(grandChildID, subManagerID, supCompanyID,
		"profesional", true, false, &desired, nil); err == nil {
		t.Fatal("un manager que no es supervisor no gestiona a nadie")
	}
}

// Con el flag apagado, el rol no existe a efectos de permisos.
func TestSupervisorTeam_FlagApagadoCierraLaPuerta(t *testing.T) {
	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := promoteAsSupervisor(s, grandChildID, true); err == nil {
		t.Fatal("sin SUPERVISOR_SCOPE el supervisor no gestiona a nadie")
	}
}

// Si no se puede resolver el árbol, se deniega: un fallo de base nunca puede
// acabar concediendo permisos.
func TestSupervisorTeam_ErrorDeArbolDeniega(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	empRepo.descErr = errors.New("db caída")
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	if _, err := promoteAsSupervisor(s, grandChildID, true); err == nil {
		t.Fatal("ante un fallo al resolver el árbol hay que denegar")
	}
}

// El empleador no pierde nada con el cambio: la vía de siempre sigue primero.
func TestSupervisorTeam_EmpleadorSigueGestionando(t *testing.T) {
	SetSupervisorScope(true)
	defer SetSupervisorScope(false)

	userRepo, empRepo := teamSetup()
	s := &userService{repo: userRepo, employmentRepo: empRepo}

	desired := true
	if _, err := s.PromoteToManager(77, 999, supCompanyID,
		string(models.UserTypeEmployer), false, false, &desired, nil); err != nil {
		t.Fatalf("el empleador gestiona todo su tenant como siempre, got: %v", err)
	}
}
