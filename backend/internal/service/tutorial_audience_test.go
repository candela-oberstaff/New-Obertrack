package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Estas pruebas cubren el CAMINO COMPLETO del público objetivo dentro del
// servicio: que el criterio llegue desde el formulario hasta la lista de gente
// a la que se le reparte. La regla en sí (quién entra) se prueba en
// models/tutorial_target_test.go; lo que se comprueba aquí es que el servicio
// la aplique y no la ignore por el camino.

type fakeAudienceUserRepo struct {
	repository.UserRepository
	users []models.User
}

func (f *fakeAudienceUserRepo) ListActiveByTypes(types []models.UserType) ([]models.User, error) {
	wanted := map[models.UserType]bool{}
	for _, t := range types {
		wanted[t] = true
	}
	var out []models.User
	for _, u := range f.users {
		if wanted[u.UserType] {
			out = append(out, u)
		}
	}
	return out, nil
}

type fakeAudienceTutorialRepo struct {
	repository.TutorialRepository
	groupMembers map[uint]bool
}

func (f *fakeAudienceTutorialRepo) UsersInGroups(groupIDs []uint) (map[uint]bool, error) {
	return f.groupMembers, nil
}

func audienceFixture() TutorialService {
	users := []models.User{
		{ID: 1, Name: "Acme", UserType: models.UserTypeEmployer, Country: "Venezuela"},
		{ID: 2, Name: "Beta", UserType: models.UserTypeEmployer, Country: "Chile"},
		{ID: 3, Name: "Ana", UserType: models.UserTypeProfessional, EmpleadorID: uintPtr(1), Country: "Venezuela", IsManager: true},
		{ID: 4, Name: "Luis", UserType: models.UserTypeProfessional, EmpleadorID: uintPtr(1), Country: "Venezuela"},
		{ID: 5, Name: "Sara", UserType: models.UserTypeProfessional, EmpleadorID: uintPtr(2), Country: "Chile", IsSupervisor: true},
		{ID: 6, Name: "Pedro", UserType: models.UserTypeProfessional, EmpleadorID: uintPtr(2), Country: "Chile"},
		// El superadmin recibe el aviso pero nunca cuenta en el alcance.
		{ID: 9, Name: "Root", UserType: models.UserTypeSuperadmin},
	}
	return NewTutorialService(
		&fakeAudienceTutorialRepo{groupMembers: map[uint]bool{4: true, 6: true}},
		&fakeAudienceUserRepo{users: users},
		nil,
	)
}

func TestPreviewAudienceWithoutTarget(t *testing.T) {
	svc := audienceFixture()

	preview, err := svc.PreviewAudience(models.TutorialAudienceAll, models.TutorialTarget{})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	// 2 empresas + 4 profesionales. El superadmin queda fuera del conteo.
	if preview.Reach != 6 {
		t.Errorf("alcance sin acotar = %d, esperaba 6", preview.Reach)
	}
	if len(preview.ByAudience) != 2 {
		t.Fatalf("esperaba desglose de dos tipos de cuenta, hubo %d", len(preview.ByAudience))
	}
}

func TestPreviewAudienceManagersOnly(t *testing.T) {
	svc := audienceFixture()

	preview, err := svc.PreviewAudience(models.TutorialAudienceAll, models.TutorialTarget{ManagersOnly: true})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	// Ana (manager) y Sara (supervisora). Las cuentas de empresa NO llevan la
	// marca de equipo a cargo, así que quedan fuera: es el comportamiento que
	// hay que recordar al usar este criterio con la audiencia "Todos".
	if preview.Reach != 2 {
		t.Errorf("alcance con solo equipo a cargo = %d, esperaba 2", preview.Reach)
	}
	for _, row := range preview.ByAudience {
		if row.UserType == string(models.UserTypeEmployer) {
			t.Errorf("las empresas no deberían entrar en 'solo con equipo a cargo': %d", row.Reach)
		}
	}
}

func TestPreviewAudienceCombinesCriteria(t *testing.T) {
	svc := audienceFixture()

	// Empresa 1 + solo con equipo a cargo: solo Ana.
	preview, err := svc.PreviewAudience(models.TutorialAudienceAll, models.TutorialTarget{
		CompanyIDs:   []uint{1},
		ManagersOnly: true,
	})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	if preview.Reach != 1 {
		t.Errorf("alcance de empresa 1 con equipo a cargo = %d, esperaba 1", preview.Reach)
	}

	// El mismo criterio sobre una empresa sin managers deja el alcance en cero,
	// que es justo lo que el contador tiene que avisar antes de publicar.
	preview, err = svc.PreviewAudience(models.TutorialAudienceEmployer, models.TutorialTarget{ManagersOnly: true})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	if preview.Reach != 0 {
		t.Errorf("alcance de solo empresas con equipo a cargo = %d, esperaba 0", preview.Reach)
	}
}

func TestPreviewAudienceGroupsAndCountry(t *testing.T) {
	svc := audienceFixture()

	// Grupo con Luis (4) y Pedro (6).
	preview, err := svc.PreviewAudience(models.TutorialAudienceAll, models.TutorialTarget{GroupIDs: []uint{7}})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	if preview.Reach != 2 {
		t.Errorf("alcance por grupo = %d, esperaba 2", preview.Reach)
	}

	// Grupo + país recorta a uno solo: los criterios se suman.
	preview, err = svc.PreviewAudience(models.TutorialAudienceAll, models.TutorialTarget{
		GroupIDs:  []uint{7},
		Countries: []string{"Chile"},
	})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	if preview.Reach != 1 {
		t.Errorf("alcance por grupo y país = %d, esperaba 1", preview.Reach)
	}
}

func TestPreviewAudienceManagers(t *testing.T) {
	svc := audienceFixture()

	// Ana (manager) y Sara (supervisora). El resto de profesionales y las dos
	// empresas quedan fuera: el rol vive dentro de los profesionales.
	preview, err := svc.PreviewAudience(models.TutorialAudienceManager, models.TutorialTarget{})
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	if preview.Reach != 2 {
		t.Errorf("alcance de managers = %d, esperaba 2", preview.Reach)
	}
	for _, row := range preview.ByAudience {
		if row.UserType == string(models.UserTypeEmployer) {
			t.Error("las empresas no entran en la audiencia de managers")
		}
	}
}

func TestNormalizeAnnounceShows(t *testing.T) {
	cases := map[int]int{
		// El 0 es una elección válida: sin tope de veces.
		0:                    0,
		1:                    1,
		5:                    5,
		maxAnnounceShows:     maxAnnounceShows,
		maxAnnounceShows + 1: maxAnnounceShows,
		-3:                   0,
	}
	for in, want := range cases {
		if got := normalizeAnnounceShows(in); got != want {
			t.Errorf("normalizeAnnounceShows(%d) = %d, esperaba %d", in, got, want)
		}
	}
}

// --- Público por roles y por personas ---

func reachOf(t *testing.T, svc TutorialService, target models.TutorialTarget) int64 {
	t.Helper()
	preview, err := svc.PreviewAudience(models.TutorialAudienceAll, target)
	if err != nil {
		t.Fatalf("previsualización falló: %v", err)
	}
	return preview.Reach
}

// Managers y supervisores van separados de los profesionales: cada rol llega
// solo a los suyos y los tres juntos cubren a todos los profesionales.
func TestPreviewAudienceByRoles(t *testing.T) {
	svc := audienceFixture()
	cases := []struct {
		roles []string
		want  int64
	}{
		{[]string{models.TargetRoleManager}, 1},      // Ana
		{[]string{models.TargetRoleSupervisor}, 1},   // Sara
		{[]string{models.TargetRoleProfessional}, 2}, // Luis y Pedro
		{[]string{models.TargetRoleProfessional, models.TargetRoleManager, models.TargetRoleSupervisor}, 4},
		{[]string{models.TargetRoleEmployer}, 2},
		// Nombrar a los superadmins los hace contar.
		{[]string{models.TargetRoleSuperadmin}, 1},
	}
	for _, tc := range cases {
		if got := reachOf(t, svc, models.TutorialTarget{Roles: tc.roles}); got != tc.want {
			t.Errorf("roles %v: alcance %d, esperaba %d", tc.roles, got, tc.want)
		}
	}
}

// "Profesionales de una empresa": rol y empresa se combinan con Y.
func TestPreviewAudienceRolesAndCompany(t *testing.T) {
	svc := audienceFixture()
	target := models.TutorialTarget{Roles: []string{models.TargetRoleProfessional, models.TargetRoleManager}, CompanyIDs: []uint{1}}
	if got := reachOf(t, svc, target); got != 2 {
		t.Errorf("profesionales y managers de Acme = %d, esperaba 2 (Ana y Luis)", got)
	}
}

// Por personas llega a la lista y a los grupos, sin filtros de perfil.
func TestPreviewAudienceByPeople(t *testing.T) {
	svc := audienceFixture()
	target := models.TutorialTarget{Mode: models.TargetModePeople, UserIDs: []uint{3, 2, 3}, Roles: []string{models.TargetRoleSupervisor}}
	if got := reachOf(t, svc, target); got != 2 {
		t.Errorf("dos personas elegidas = %d, esperaba 2 (los roles no aplican)", got)
	}
	withGroup := models.TutorialTarget{Mode: models.TargetModePeople, UserIDs: []uint{3}, GroupIDs: []uint{1}}
	if got := reachOf(t, svc, withGroup); got != 3 {
		t.Errorf("persona + grupo = %d, esperaba 3 (Ana, Luis y Pedro)", got)
	}
	if got := reachOf(t, svc, models.TutorialTarget{Mode: models.TargetModePeople}); got != 0 {
		t.Errorf("sin nadie elegido = %d, esperaba 0", got)
	}
}

func TestNormalizeTarget(t *testing.T) {
	target, audience, err := normalizeTarget(models.TutorialTarget{Roles: []string{"empresa", "nada", "empresa"}, ManagersOnly: true, UserIDs: []uint{4}})
	if err != nil || audience != models.TutorialAudienceEmployer {
		t.Fatalf("solo empresas debe dar audiencia empleador: %q %v", audience, err)
	}
	if len(target.Roles) != 1 || target.ManagersOnly || target.UserIDs != nil {
		t.Fatalf("público mal limpiado: %+v", target)
	}
	if _, audience, _ := normalizeTarget(models.TutorialTarget{Roles: []string{"manager", "supervisor"}}); audience != models.TutorialAudienceProfessional {
		t.Errorf("roles de profesional deben dar audiencia profesional, dio %q", audience)
	}
	if _, audience, _ := normalizeTarget(models.TutorialTarget{Roles: []string{"empresa", "manager"}}); audience != models.TutorialAudienceAll {
		t.Errorf("una mezcla debe dar audiencia all, dio %q", audience)
	}
	if _, audience, _ := normalizeTarget(models.TutorialTarget{CompanyIDs: []uint{1}}); audience != "" {
		t.Errorf("sin roles no se decide la audiencia, dio %q", audience)
	}
	if _, _, err := normalizeTarget(models.TutorialTarget{Mode: models.TargetModePeople}); err == nil {
		t.Error("por personas sin nadie debe fallar al guardar")
	}
}

type fakeListTutorialRepo struct {
	fakeAudienceTutorialRepo
	tutorials []models.Tutorial
}

func (f *fakeListTutorialRepo) FindAll(onlyActive bool, audiences []string) ([]models.Tutorial, error) {
	return f.tutorials, nil
}

func (f *fakeAudienceUserRepo) GetByID(id uint) (*models.User, error) {
	for i := range f.users {
		if f.users[i].ID == id {
			return &f.users[i], nil
		}
	}
	return nil, nil
}

// El listado aplica el público: una novedad para dos personas no aparece en
// la lista de los demás, y una por rol solo en la de ese rol.
func TestGetAllFiltersByTarget(t *testing.T) {
	users := []models.User{
		{ID: 3, UserType: models.UserTypeProfessional, IsManager: true},
		{ID: 4, UserType: models.UserTypeProfessional},
	}
	repo := &fakeListTutorialRepo{tutorials: []models.Tutorial{
		{ID: 1, Title: "Para todos"},
		{ID: 2, Title: "Para Ana", Target: models.TutorialTarget{Mode: models.TargetModePeople, UserIDs: []uint{3}}},
		{ID: 3, Title: "Para managers", Target: models.TutorialTarget{Roles: []string{models.TargetRoleManager}}},
	}}
	svc := NewTutorialService(repo, &fakeAudienceUserRepo{users: users}, nil)

	titles := func(viewer uint) []string {
		list, err := svc.GetAll(true, []string{models.TutorialAudienceAll}, viewer)
		if err != nil {
			t.Fatalf("listado: %v", err)
		}
		out := []string{}
		for _, tu := range list {
			out = append(out, tu.Title)
		}
		return out
	}
	if got := titles(3); len(got) != 3 {
		t.Errorf("Ana (manager elegida) debe ver las tres: %v", got)
	}
	if got := titles(4); len(got) != 1 || got[0] != "Para todos" {
		t.Errorf("Luis solo debe ver la general: %v", got)
	}
	// Sin espectador (superadmin) no se filtra.
	if got := titles(0); len(got) != 3 {
		t.Errorf("sin espectador se listan todas: %v", got)
	}
}
