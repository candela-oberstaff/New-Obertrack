package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// El grupo de automatizaciones mezcla un comodín y rutas fijas en la misma posición:
// /workflows/:id/runs convive con /workflows/gates. Es justo el caso en el que gin
// puede entrar en pánico al construir el árbol —y un pánico al registrar rutas no se
// ve en ninguna prueba de servicio: aparece al arrancar, en el despliegue.
//
// Esta prueba registra el mismo conjunto contra un router limpio y comprueba que cada
// camino llega a su destino, incluido el que decide si "gates" se lo traga el comodín.
func TestWorkflowRoutes_ElComodinNoSeTragaLasPuertas(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")

	hit := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { c.String(http.StatusOK, name) }
	}

	// Registrar es la mitad de la prueba.
	workflows := api.Group("/workflows")
	{
		workflows.GET("", hit("list"))
		workflows.GET("/recipes", hit("recipes"))
		workflows.POST("/recipes", hit("set-recipe"))
		workflows.GET("/:id/runs", hit("runs"))
		workflows.GET("/gates", hit("gates"))
		workflows.POST("/gates", hit("create-gate"))
		workflows.PUT("/gates/:id", hit("update-gate"))
		workflows.DELETE("/gates/:id", hit("delete-gate"))
	}

	casos := []struct {
		metodo, ruta, espera string
	}{
		{http.MethodGet, "/api/workflows", "list"},
		{http.MethodGet, "/api/workflows/recipes", "recipes"},
		{http.MethodPost, "/api/workflows/recipes", "set-recipe"},
		{http.MethodGet, "/api/workflows/7/runs", "runs"},
		// Las cuatro del constructor. Que "gates" no acabe interpretándose como un
		// id es lo único que hay que fijar aquí.
		{http.MethodGet, "/api/workflows/gates", "gates"},
		{http.MethodPost, "/api/workflows/gates", "create-gate"},
		{http.MethodPut, "/api/workflows/gates/3", "update-gate"},
		{http.MethodDelete, "/api/workflows/gates/3", "delete-gate"},
	}

	for _, c := range casos {
		t.Run(c.metodo+" "+c.ruta, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(c.metodo, c.ruta, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("%s %s devolvió %d", c.metodo, c.ruta, w.Code)
			}
			if w.Body.String() != c.espera {
				t.Fatalf("%s %s llegó a %q y debía llegar a %q", c.metodo, c.ruta, w.Body.String(), c.espera)
			}
		})
	}
}
