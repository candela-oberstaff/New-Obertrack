package routes

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/handlers"
)

// Gin entra en pánico si un segmento estático choca con un wildcard hermano.
// /meetings/upcoming convive con /meetings/:id, así que conviene comprobarlo
// aquí y no descubrirlo con el contenedor cayéndose al arrancar.
func TestMeetingRoutesRegisterWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	h := &handlers.MeetingHandler{}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("gin entró en pánico registrando las rutas de sesiones: %v", rec)
		}
	}()

	m := api.Group("/meetings")
	m.GET("", h.List)
	m.GET("/upcoming", h.Upcoming)
	m.GET("/:id", h.Get)
	m.GET("/:id/presence", h.Presence)
	m.POST("", h.Create)
	m.PUT("/:id", h.Update)
	m.DELETE("/:id", h.Cancel)
}
