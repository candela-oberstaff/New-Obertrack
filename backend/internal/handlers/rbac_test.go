package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// stubRBACService implementa solo EffectivePermissions; el resto de la
// interfaz embebida queda nil y haría panic si algo inesperado la llamara.
type stubRBACService struct {
	service.RBACService
	perms    map[string]string
	hasRoles bool
}

func (s *stubRBACService) EffectivePermissions(userID, tenantID uint) (map[string]string, bool, error) {
	return s.perms, s.hasRoles, nil
}

func permCtx(method, role string, isSuperadmin bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/", nil)
	c.Set("user_id", uint(42))
	c.Set("role", role)
	c.Set("is_superadmin", isSuperadmin)
	return c, w
}

func TestRequirePermission(t *testing.T) {
	cases := []struct {
		name      string
		role      string
		isSuper   bool
		svc       *stubRBACService
		module    string
		level     string
		wantAllow bool
	}{
		{
			name: "superadmin nunca se restringe", role: "superadmin", isSuper: true,
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"hours": "none"}},
			module: "hours", level: models.PermissionEdit, wantAllow: true,
		},
		{
			name: "cuenta empresa nunca se restringe", role: "empleador",
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"hours": "none"}},
			module: "hours", level: models.PermissionEdit, wantAllow: true,
		},
		{
			name: "sin roles asignados conserva el comportamiento histórico", role: "profesional",
			svc:    &stubRBACService{hasRoles: false},
			module: "hours", level: models.PermissionEdit, wantAllow: true,
		},
		{
			name: "view alcanza para una ruta de lectura", role: "profesional",
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"hours": "view"}},
			module: "hours", level: models.PermissionView, wantAllow: true,
		},
		{
			name: "view NO alcanza para una ruta de escritura", role: "profesional",
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"hours": "view"}},
			module: "hours", level: models.PermissionEdit, wantAllow: false,
		},
		{
			name: "edit alcanza para todo", role: "profesional",
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"hours": "edit"}},
			module: "hours", level: models.PermissionEdit, wantAllow: true,
		},
		{
			name: "sin acceso bloquea incluso la lectura", role: "profesional",
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"hours": "none"}},
			module: "hours", level: models.PermissionView, wantAllow: false,
		},
		{
			name: "módulo no presente en los permisos = sin acceso", role: "profesional",
			svc:    &stubRBACService{hasRoles: true, perms: map[string]string{"tasks": "edit"}},
			module: "hours", level: models.PermissionView, wantAllow: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := permCtx(http.MethodPost, tc.role, tc.isSuper)
			RequirePermission(tc.svc, tc.module, tc.level)(c)

			allowed := !c.IsAborted()
			if allowed != tc.wantAllow {
				t.Fatalf("allowed = %v (status %d), esperaba %v", allowed, w.Code, tc.wantAllow)
			}
			if !tc.wantAllow && w.Code != http.StatusForbidden {
				t.Fatalf("un bloqueo debe responder 403, obtuve %d", w.Code)
			}
		})
	}
}

func TestAudienceForRequest(t *testing.T) {
	cases := []struct {
		role    string
		isSuper bool
		want    string
	}{
		{"superadmin", true, ""},                                        // ve todas las audiencias
		{string(models.UserTypeEmployer), false, models.TutorialAudienceEmployer},
		{string(models.UserTypeProfessional), false, models.TutorialAudienceProfessional},
		{string(models.UserTypeCustomerSuccess), false, ""},             // soporte ve todo
		{string(models.UserTypeITAnalyst), false, ""},
	}
	for _, tc := range cases {
		c, _ := permCtx(http.MethodGet, tc.role, tc.isSuper)
		if got := audienceForRequest(c); got != tc.want {
			t.Errorf("audienceForRequest(role=%s, super=%v) = %q, esperaba %q", tc.role, tc.isSuper, got, tc.want)
		}
	}
}

func TestRequireRBACManager(t *testing.T) {
	cases := []struct {
		role      string
		isSuper   bool
		wantAllow bool
	}{
		{"superadmin", true, true},
		{string(models.UserTypeEmployer), false, true},
		{string(models.UserTypeProfessional), false, false},
		{string(models.UserTypeCustomerSuccess), false, false},
	}
	for _, tc := range cases {
		c, w := permCtx(http.MethodPost, tc.role, tc.isSuper)
		RequireRBACManager()(c)
		allowed := !c.IsAborted()
		if allowed != tc.wantAllow {
			t.Errorf("RequireRBACManager(role=%s) allowed = %v (status %d), esperaba %v", tc.role, allowed, w.Code, tc.wantAllow)
		}
	}
}
