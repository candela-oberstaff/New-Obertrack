package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
)

func accessCtx(method, role string, isSuperadmin, isManager bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/", nil)
	c.Set("user_id", uint(42))
	c.Set("role", role)
	c.Set("is_superadmin", isSuperadmin)
	c.Set("is_manager", isManager)
	return c, w
}

// El panel admin: superadmin gestiona todo; customer success solo consulta (GET).
func TestRequireAdminPanel(t *testing.T) {
	cs := string(models.UserTypeCustomerSuccess)
	cases := []struct {
		name      string
		method    string
		role      string
		isSuper   bool
		wantAllow bool
	}{
		{"superadmin GET", http.MethodGet, "superadmin", true, true},
		{"superadmin DELETE", http.MethodDelete, "superadmin", true, true},
		{"customer success consulta (GET)", http.MethodGet, cs, false, true},
		{"customer success NO puede mutar (POST)", http.MethodPost, cs, false, false},
		{"customer success NO puede mutar (DELETE)", http.MethodDelete, cs, false, false},
		{"profesional sin acceso ni a consulta", http.MethodGet, string(models.UserTypeProfessional), false, false},
		{"empresa sin acceso al panel de plataforma", http.MethodGet, string(models.UserTypeEmployer), false, false},
		{"analista de IT sin acceso", http.MethodGet, string(models.UserTypeITAnalyst), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := accessCtx(tc.method, tc.role, tc.isSuper, false)
			requireAdminPanel()(c)
			allowed := !c.IsAborted()
			if allowed != tc.wantAllow {
				t.Fatalf("allowed = %v (status %d), esperaba %v", allowed, w.Code, tc.wantAllow)
			}
		})
	}
}

// Gestión de usuarios (promover/asignar/reasignar manager, activar/desactivar):
// solo el dueño de la empresa (empleador) o superadmin. Un manager (is_manager)
// NO puede, aunque el flag esté activo: defensa en profundidad sobre el servicio.
func TestRequireManageUsers(t *testing.T) {
	cases := []struct {
		name      string
		role      string
		isSuper   bool
		isManager bool
		wantAllow bool
	}{
		{"superadmin", "superadmin", true, false, true},
		{"empresa (empleador)", string(models.UserTypeEmployer), false, false, true},
		{"manager (flag) NO puede gestionar usuarios", string(models.UserTypeProfessional), false, true, false},
		{"profesional sin permiso", string(models.UserTypeProfessional), false, false, false},
		{"customer success sin permiso", string(models.UserTypeCustomerSuccess), false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := accessCtx(http.MethodPost, tc.role, tc.isSuper, tc.isManager)
			requireManageUsers()(c)
			allowed := !c.IsAborted()
			if allowed != tc.wantAllow {
				t.Fatalf("allowed = %v (status %d), esperaba %v", allowed, w.Code, tc.wantAllow)
			}
		})
	}
}

// Bandeja de soporte (tickets/tools): superadmin y cualquier customer success.
func TestRequireSupportInboxAccess(t *testing.T) {
	cases := []struct {
		role      string
		isSuper   bool
		wantAllow bool
	}{
		{"superadmin", true, true},
		{string(models.UserTypeCustomerSuccess), false, true},
		{string(models.UserTypeProfessional), false, false},
		{string(models.UserTypeITAnalyst), false, false},
	}
	for _, tc := range cases {
		c, w := accessCtx(http.MethodGet, tc.role, tc.isSuper, false)
		requireSupportInboxAccess()(c)
		allowed := !c.IsAborted()
		if allowed != tc.wantAllow {
			t.Errorf("requireSupportInboxAccess(role=%s) allowed = %v (status %d), esperaba %v", tc.role, allowed, w.Code, tc.wantAllow)
		}
	}
}
