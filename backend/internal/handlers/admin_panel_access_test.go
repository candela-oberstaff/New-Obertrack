package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
)

// isAdminPanelUser tiene que decir lo mismo que requireAdminPanel (el guard del
// grupo). Cuando no coincidían, la pantalla abría y el dato llegaba 403: es lo
// que le pasaba al Mapa con customer success.
func TestIsAdminPanelUser(t *testing.T) {
	casos := []struct {
		nombre  string
		rol     string
		isSuper bool
		quiere  bool
	}{
		{"superadmin", "superadmin", true, true},
		{"customer success", string(models.UserTypeCustomerSuccess), false, true},
		{"profesional", string(models.UserTypeProfessional), false, false},
		{"empresa", string(models.UserTypeEmployer), false, false},
		{"analista de IT", string(models.UserTypeITAnalyst), false, false},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Set("user_id", uint(7))
			c.Set("role", tc.rol)
			c.Set("is_superadmin", tc.isSuper)

			if got := isAdminPanelUser(c); got != tc.quiere {
				t.Fatalf("isAdminPanelUser = %v, se esperaba %v", got, tc.quiere)
			}
		})
	}
}
