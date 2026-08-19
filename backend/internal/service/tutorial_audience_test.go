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
