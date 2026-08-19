package models

import "testing"

// La regla del publico objetivo la comparten el reparto de notificaciones, el
// aviso a pantalla completa y el alcance de las metricas. Si se rompe aqui, las
// tres cosas mienten a la vez, asi que se prueba a fondo.

func company(id uint) *uint { return &id }

func TestTargetIsEmpty(t *testing.T) {
	if !(TutorialTarget{}).IsEmpty() {
		t.Error("un publico sin criterios debe ser vacio")
	}
	notEmpty := []TutorialTarget{
		{CompanyIDs: []uint{1}},
		{Countries: []string{"Venezuela"}},
		{GroupIDs: []uint{3}},
		{ManagersOnly: true},
	}
	for i, target := range notEmpty {
		if target.IsEmpty() {
			t.Errorf("caso %d: el publico tiene criterios y se tomo como vacio", i)
		}
	}
}

func TestTargetMatchesEmpty(t *testing.T) {
	// Sin acotar alcanza a cualquiera: es el comportamiento de toda novedad
	// anterior a esta funcion.
	user := &User{ID: 7, UserType: UserTypeProfessional}
	if !(TutorialTarget{}).Matches(user, false) {
		t.Error("un publico vacio deberia alcanzar a cualquiera")
	}
	if (TutorialTarget{}).Matches(nil, false) {
		t.Error("sin usuario no hay coincidencia posible")
	}
}

func TestTargetMatchesCompany(t *testing.T) {
	target := TutorialTarget{CompanyIDs: []uint{5}}

	// La cuenta de la empresa entra por su propio ID...
	if !target.Matches(&User{ID: 5, UserType: UserTypeEmployer}, false) {
		t.Error("la empresa elegida deberia entrar en su propio publico")
	}
	// ...y sus profesionales por el vinculo.
	if !target.Matches(&User{ID: 9, UserType: UserTypeProfessional, EmpleadorID: company(5)}, false) {
		t.Error("un profesional de la empresa elegida deberia entrar")
	}
	if target.Matches(&User{ID: 10, UserType: UserTypeProfessional, EmpleadorID: company(6)}, false) {
		t.Error("un profesional de otra empresa no deberia entrar")
	}
	if target.Matches(&User{ID: 11, UserType: UserTypeProfessional}, false) {
		t.Error("un profesional sin empresa no deberia entrar")
	}
}

func TestTargetMatchesCountry(t *testing.T) {
	target := TutorialTarget{Countries: []string{"Venezuela", "Colombia"}}

	if !target.Matches(&User{ID: 1, Country: "Colombia"}, false) {
		t.Error("el pais elegido deberia entrar")
	}
	// Mayusculas y espacios no deberian dejar gente fuera de un aviso.
	if !target.Matches(&User{ID: 2, Country: " venezuela "}, false) {
		t.Error("el pais deberia compararse sin distinguir mayusculas ni espacios")
	}
	if target.Matches(&User{ID: 3, Country: "Peru"}, false) {
		t.Error("otro pais no deberia entrar")
	}
	if target.Matches(&User{ID: 4}, false) {
		t.Error("sin pais en la ficha no deberia entrar")
	}
}

func TestTargetMatchesGroupAndManagers(t *testing.T) {
	group := TutorialTarget{GroupIDs: []uint{2}}
	if !group.Matches(&User{ID: 1}, true) {
		t.Error("quien esta en el grupo deberia entrar")
	}
	if group.Matches(&User{ID: 1}, false) {
		t.Error("quien no esta en el grupo no deberia entrar")
	}

	managers := TutorialTarget{ManagersOnly: true}
	if !managers.Matches(&User{ID: 1, IsManager: true}, false) {
		t.Error("un manager deberia entrar")
	}
	// El supervisor tiene equipo a cargo: cuenta como manager para esto.
	if !managers.Matches(&User{ID: 2, IsSupervisor: true}, false) {
		t.Error("un supervisor deberia entrar")
	}
	if managers.Matches(&User{ID: 3}, false) {
		t.Error("quien no tiene equipo a cargo no deberia entrar")
	}
}

func TestTargetCriteriaCombineWithAnd(t *testing.T) {
	// "Profesionales de Acme que ademas esten en Venezuela y tengan equipo".
	target := TutorialTarget{
		CompanyIDs:   []uint{5},
		Countries:    []string{"Venezuela"},
		ManagersOnly: true,
	}
	full := &User{ID: 9, UserType: UserTypeProfessional, EmpleadorID: company(5), Country: "Venezuela", IsManager: true}
	if !target.Matches(full, false) {
		t.Error("quien cumple todos los criterios deberia entrar")
	}

	// Fallar UNO solo deja fuera: los criterios se suman, no se eligen.
	wrongCompany := *full
	wrongCompany.EmpleadorID = company(6)
	wrongCountry := *full
	wrongCountry.Country = "Peru"
	notManager := *full
	notManager.IsManager = false

	for name, user := range map[string]*User{
		"otra empresa": &wrongCompany,
		"otro pais":    &wrongCountry,
		"sin equipo":   &notManager,
	} {
		if target.Matches(user, false) {
			t.Errorf("%s: deberia quedar fuera cuando falla un criterio", name)
		}
	}
}
