package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// stubRBACRepo implementa solo lo que estos tests necesitan; el resto de la
// interfaz embebida queda nil y haría panic si algo inesperado la llamara.
type stubRBACRepo struct {
	repository.RBACRepository
	roles []models.Role
}

func (s *stubRBACRepo) GetUserRoles(userID, tenantID uint) ([]models.Role, error) {
	return s.roles, nil
}

func (s *stubRBACRepo) ListRoles(tenantID uint) ([]models.Role, error) {
	return s.roles, nil
}

func (s *stubRBACRepo) CreateRole(role *models.Role) error {
	s.roles = append(s.roles, *role)
	return nil
}

func TestNormalizePermissions(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"vacío se normaliza a objeto vacío", "", false},
		{"objeto válido", `{"tasks":"edit","hours":"view"}`, false},
		{"nivel none válido", `{"chat":"none"}`, false},
		{"nivel inválido", `{"tasks":"admin"}`, true},
		{"no es JSON", `tasks=edit`, true},
		{"JSON pero no objeto de strings", `{"tasks":1}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizePermissions(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("normalizePermissions(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}

	normalized, err := normalizePermissions("")
	if err != nil || normalized != "{}" {
		t.Fatalf("permisos vacíos deben normalizar a {}, obtuve %q (err %v)", normalized, err)
	}
}

func TestEffectivePermissionsMergeMasPermisivo(t *testing.T) {
	repo := &stubRBACRepo{roles: []models.Role{
		{ID: 1, Permissions: `{"tasks":"view","hours":"edit"}`},
		{ID: 2, Permissions: `{"tasks":"edit","chat":"view"}`},
		{ID: 3, Permissions: `permisos corruptos`}, // no debe aportar ni romper
	}}
	svc := NewRBACService(repo, nil)

	perms, hasRoles, err := svc.EffectivePermissions(1)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !hasRoles {
		t.Fatal("hasRoles debe ser true cuando el usuario tiene roles")
	}
	if perms["tasks"] != models.PermissionEdit {
		t.Errorf("tasks: gana el más permisivo (edit), obtuve %q", perms["tasks"])
	}
	if perms["hours"] != models.PermissionEdit {
		t.Errorf("hours: esperaba edit, obtuve %q", perms["hours"])
	}
	if perms["chat"] != models.PermissionView {
		t.Errorf("chat: esperaba view, obtuve %q", perms["chat"])
	}
}

func TestEffectivePermissionsSinRoles(t *testing.T) {
	svc := NewRBACService(&stubRBACRepo{}, nil)
	_, hasRoles, err := svc.EffectivePermissions(1)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if hasRoles {
		t.Fatal("sin roles asignados, hasRoles debe ser false (comportamiento histórico)")
	}
}

func TestSeedDefaultRolesEsIdempotente(t *testing.T) {
	repo := &stubRBACRepo{}
	svc := NewRBACService(repo, nil)

	if err := svc.SeedDefaultRoles(7, 1); err != nil {
		t.Fatalf("primer seed falló: %v", err)
	}
	if len(repo.roles) != len(defaultRolePresets) {
		t.Fatalf("esperaba %d presets, obtuve %d", len(defaultRolePresets), len(repo.roles))
	}

	if err := svc.SeedDefaultRoles(7, 1); err != nil {
		t.Fatalf("segundo seed falló: %v", err)
	}
	if len(repo.roles) != len(defaultRolePresets) {
		t.Fatalf("el seed no es idempotente: %d roles tras la segunda corrida", len(repo.roles))
	}

	// Todos los presets deben traer permisos válidos.
	for _, preset := range defaultRolePresets {
		if _, err := normalizePermissions(preset.Permissions); err != nil {
			t.Errorf("preset %q tiene permisos inválidos: %v", preset.Name, err)
		}
	}

	if err := svc.SeedDefaultRoles(0, 1); err == nil {
		t.Error("tenant 0 debe rechazarse")
	}
}
