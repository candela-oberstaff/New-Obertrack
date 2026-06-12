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

// Transferencias y reporte de rechazos: solo superadmin y CS Manager.
func TestRequireSupportManager(t *testing.T) {
	cs := string(models.UserTypeCustomerSuccess)
	cases := []struct {
		name      string
		role      string
		isSuper   bool
		isManager bool
		wantAllow bool
	}{
		{"superadmin", "superadmin", true, false, true},
		{"CS manager", cs, false, true, true},
		{"CS analista (sin flag manager)", cs, false, false, false},
		{"profesional manager NO es soporte", string(models.UserTypeProfessional), false, true, false},
		{"empresa NO gestiona soporte", string(models.UserTypeEmployer), false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := accessCtx(http.MethodPost, tc.role, tc.isSuper, tc.isManager)
			requireSupportManager()(c)
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
