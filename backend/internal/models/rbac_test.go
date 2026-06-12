package models

import "testing"

func TestIsValidPermissionLevel(t *testing.T) {
	for _, level := range []string{PermissionNone, PermissionView, PermissionEdit} {
		if !IsValidPermissionLevel(level) {
			t.Errorf("nivel válido rechazado: %q", level)
		}
	}
	for _, level := range []string{"", "admin", "VIEW", "editar"} {
		if IsValidPermissionLevel(level) {
			t.Errorf("nivel inválido aceptado: %q", level)
		}
	}
}

func TestIsValidTutorialAudience(t *testing.T) {
	for _, audience := range []string{TutorialAudienceAll, TutorialAudienceEmployer, TutorialAudienceProfessional} {
		if !IsValidTutorialAudience(audience) {
			t.Errorf("audiencia válida rechazada: %q", audience)
		}
	}
	for _, audience := range []string{"", "superadmin", "empresa", "ALL"} {
		if IsValidTutorialAudience(audience) {
			t.Errorf("audiencia inválida aceptada: %q", audience)
		}
	}
}

func TestTenantForUser(t *testing.T) {
	empleadorID := uint(8)

	if got := TenantForUser(nil); got != 0 {
		t.Errorf("usuario nil debe ser tenant 0, obtuve %d", got)
	}

	empresa := &User{ID: 8, UserType: UserTypeEmployer}
	if got := TenantForUser(empresa); got != 8 {
		t.Errorf("la cuenta empresa es su propio tenant: esperaba 8, obtuve %d", got)
	}

	profesional := &User{ID: 3, UserType: UserTypeProfessional, EmpleadorID: &empleadorID}
	if got := TenantForUser(profesional); got != 8 {
		t.Errorf("el profesional hereda el tenant de su empleador: esperaba 8, obtuve %d", got)
	}

	csVinculado := &User{ID: 6, UserType: UserTypeCustomerSuccess, EmpleadorID: &empleadorID}
	if got := TenantForUser(csVinculado); got != 8 {
		t.Errorf("el CS vinculado hereda el tenant de su empresa: esperaba 8, obtuve %d", got)
	}

	superadmin := &User{ID: 1, UserType: UserTypeSuperadmin}
	if got := TenantForUser(superadmin); got != 0 {
		t.Errorf("el superadmin no tiene tenant: esperaba 0, obtuve %d", got)
	}

	itAnalyst := &User{ID: 9, UserType: UserTypeITAnalyst}
	if got := TenantForUser(itAnalyst); got != 0 {
		t.Errorf("el analista de IT no tiene tenant: esperaba 0, obtuve %d", got)
	}
}
