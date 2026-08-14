package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
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

// El panel admin: superadmin y customer success gestionan con el mismo alcance.
// Las cuatro funciones fuera de CS (Papelera, Auditoría, Configuración y
// Novedades) NO se controlan aquí sino en su propia ruta, para que abrir este
// panel no las arrastre.
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
		{"customer success gestiona (POST)", http.MethodPost, cs, false, true},
		{"customer success gestiona (DELETE)", http.MethodDelete, cs, false, true},
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

// Las cuatro funciones que Customer Success NO recibe. Es el recorte del rol, y
// lo que más fácil se rompe sin querer: Papelera y Configuración cuelgan del
// grupo admin —que ahora SÍ abre a CS— y Novedades la ve todo el mundo, así que
// las tres dependen de una guarda propia que hay que no perder de vista.
func TestCustomerSuccessQuedaFueraDeLasCuatroFunciones(t *testing.T) {
	cs := string(models.UserTypeCustomerSuccess)

	t.Run("Papelera y Configuración exigen superadmin", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			c, w := accessCtx(method, cs, false, false)
			middleware.RequireSuperadmin()(c)
			if !c.IsAborted() {
				t.Errorf("%s: customer success no debería pasar RequireSuperadmin (status %d)", method, w.Code)
			}
		}
	})

	t.Run("Auditoría exige soporte técnico de plataforma", func(t *testing.T) {
		c, w := accessCtx(http.MethodGet, cs, false, false)
		middleware.RequirePlatformTech()(c)
		if !c.IsAborted() {
			t.Errorf("customer success no debería acceder a auditoría (status %d)", w.Code)
		}
	})

	t.Run("Novedades bloquea a customer success y deja pasar al resto", func(t *testing.T) {
		c, w := accessCtx(http.MethodGet, cs, false, false)
		middleware.BlockCustomerSuccess()(c)
		if !c.IsAborted() {
			t.Errorf("customer success no debería ver Novedades (status %d)", w.Code)
		}
		for _, role := range []string{string(models.UserTypeProfessional), string(models.UserTypeEmployer)} {
			c, _ := accessCtx(http.MethodGet, role, false, false)
			middleware.BlockCustomerSuccess()(c)
			if c.IsAborted() {
				t.Errorf("%s sí debería ver Novedades", role)
			}
		}
	})
}

// Escritura en el expediente de empresa: es la excepción a requireAdminPanel.
// Customer success SÍ puede anotar (notas y contactos) porque es el área que
// atiende a las empresas; el resto de roles sigue fuera.
func TestRequireSupportWrite(t *testing.T) {
	cs := string(models.UserTypeCustomerSuccess)
	cases := []struct {
		name      string
		method    string
		role      string
		isSuper   bool
		wantAllow bool
	}{
		{"superadmin escribe", http.MethodPost, "superadmin", true, true},
		{"superadmin borra", http.MethodDelete, "superadmin", true, true},
		{"customer success escribe", http.MethodPost, cs, false, true},
		{"customer success edita", http.MethodPut, cs, false, true},
		{"customer success borra", http.MethodDelete, cs, false, true},
		{"profesional no", http.MethodPost, string(models.UserTypeProfessional), false, false},
		{"empresa no escribe en su propio expediente de plataforma", http.MethodPost, string(models.UserTypeEmployer), false, false},
		{"analista de IT no", http.MethodPost, string(models.UserTypeITAnalyst), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := accessCtx(tc.method, tc.role, tc.isSuper, false)
			requireSupportWrite()(c)
			allowed := !c.IsAborted()
			if allowed != tc.wantAllow {
				t.Fatalf("allowed = %v (status %d), esperaba %v", allowed, w.Code, tc.wantAllow)
			}
		})
	}
}

// Gestión de CUENTAS (activar/desactivar): solo el dueño de la empresa
// (empleador) o superadmin. Un manager NO puede, aunque el flag esté activo:
// defensa en profundidad sobre el servicio. Las acciones sobre el equipo
// (promover/asignar/reasignar manager) se separaron a requireManageTeam, que sí
// admite supervisores.
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

// Acciones sobre el equipo (promover/asignar/reasignar manager): empleador,
// superadmin y —solo con el flag encendido— supervisor. Esta guarda decide quién
// puede intentarlo; que el objetivo esté en su árbol lo comprueba el servicio.
func TestRequireManageTeam(t *testing.T) {
	prof := string(models.UserTypeProfessional)
	cases := []struct {
		name         string
		role         string
		isSuper      bool
		isManager    bool
		isSupervisor bool
		flagOn       bool
		wantAllow    bool
	}{
		{"superadmin", "superadmin", true, false, false, false, true},
		{"empresa (empleador)", string(models.UserTypeEmployer), false, false, false, false, true},
		{"supervisor con el flag encendido", prof, false, true, true, true, true},
		{"supervisor con el flag APAGADO no pasa", prof, false, true, true, false, false},
		{"manager normal no pasa", prof, false, true, false, true, false},
		{"profesional no pasa", prof, false, false, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service.SetSupervisorScope(tc.flagOn)
			defer service.SetSupervisorScope(false)

			c, w := accessCtx(http.MethodPost, tc.role, tc.isSuper, tc.isManager)
			c.Set("is_supervisor", tc.isSupervisor)
			requireManageTeam()(c)
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
