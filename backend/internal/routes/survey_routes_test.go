package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// La encuesta reparte sus rutas entre el panel (/surveys/:id, con sesión) y la
// página que se responde sin sesión. Esa segunda NO puede colgar de
// /surveys/:token: dos comodines distintos en el mismo tramo hacen que gin
// entre en pánico al arrancar, y eso no se ve hasta el despliegue. Por eso vive
// en /survey-link/:token, igual que induction respecto de inductions.
//
// Esta prueba fija la convención: registra el mismo conjunto contra un router
// limpio —registrar ya es media prueba— y comprueba que cada camino llega a su
// destino, incluido el clic del correo, que sí cuelga de /surveys/:id porque
// "quick-response" es un hijo estático.
func TestRutasDeEncuesta_ElEnlacePublicoNoChocaConElPanel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")

	hit := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { c.String(http.StatusOK, name) }
	}

	surveys := api.Group("/surveys")
	{
		surveys.GET("", hit("list"))
		surveys.GET("/:id", hit("get"))
		surveys.POST("/:id/responses", hit("submit"))
		surveys.POST("/:id/send", hit("send"))
		surveys.GET("/:id/quick-response", hit("quick"))
	}
	surveyLink := api.Group("/survey-link")
	{
		surveyLink.GET("/:token", hit("landing"))
		surveyLink.POST("/:token/responses", hit("public-submit"))
	}

	casos := []struct {
		method, path, want string
	}{
		{http.MethodGet, "/api/surveys", "list"},
		{http.MethodGet, "/api/surveys/42", "get"},
		{http.MethodPost, "/api/surveys/42/responses", "submit"},
		{http.MethodPost, "/api/surveys/42/send", "send"},
		{http.MethodGet, "/api/surveys/42/quick-response", "quick"},
		// El token lleva puntos; tiene que llegar entero a la landing.
		{http.MethodGet, "/api/survey-link/3.7.1790000000.abc123", "landing"},
		{http.MethodPost, "/api/survey-link/3.7.1790000000.abc123/responses", "public-submit"},
	}

	for _, tc := range casos {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s %s: código %d, se esperaba 200", tc.method, tc.path, w.Code)
			continue
		}
		if got := w.Body.String(); got != tc.want {
			t.Errorf("%s %s resolvió a %q, se esperaba %q", tc.method, tc.path, got, tc.want)
		}
	}
}
