package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// El módulo de testimonios reparte sus rutas entre dos grupos que se parecen
// mucho: la página pública cuelga de /testimonial/:token (singular, sin sesión)
// y el panel de /testimonials (plural, con sesión). Si alguien unificara los
// prefijos, el comodín del token se tragaría las rutas del panel —o gin
// entraría en pánico al arrancar, que es peor porque no se ve hasta el
// despliegue—.
//
// Esta prueba fija esa convención registrando el mismo conjunto de rutas contra
// un router limpio y comprobando que cada camino llega a su destino.
func TestTestimonialRoutes_SingularYPluralNoColisionan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")

	hit := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { c.String(http.StatusOK, name) }
	}

	// Registrar es la mitad de la prueba: un conflicto de comodines hace que
	// gin entre en pánico justo aquí.
	public := api.Group("/testimonial")
	{
		public.GET("/:token", hit("landing"))
		public.POST("/:token/submit", hit("submit"))
	}
	internal := api.Group("/testimonials")
	{
		internal.GET("/templates", hit("templates"))
		internal.GET("", hit("list"))
		internal.GET("/:id", hit("get"))
		internal.GET("/:id/signature", hit("signature"))
		internal.GET("/:id/consent.pdf", hit("consent"))
	}

	cases := []struct {
		method, path, want string
	}{
		{http.MethodGet, "/api/testimonial/abc123", "landing"},
		{http.MethodPost, "/api/testimonial/abc123/submit", "submit"},
		{http.MethodGet, "/api/testimonials", "list"},
		// La ruta estática tiene que ganarle al comodín: si resolviera a "get",
		// el panel pediría el testimonio con id "templates".
		{http.MethodGet, "/api/testimonials/templates", "templates"},
		{http.MethodGet, "/api/testimonials/42", "get"},
		{http.MethodGet, "/api/testimonials/42/signature", "signature"},
		{http.MethodGet, "/api/testimonials/42/consent.pdf", "consent"},
	}

	for _, tc := range cases {
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
