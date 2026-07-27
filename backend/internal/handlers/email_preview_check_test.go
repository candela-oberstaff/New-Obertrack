package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEmailPreviewRenders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewEmailPreviewHandler()

	// Index lista las tres plantillas.
	r := gin.New()
	r.GET("/api/dev/email-preview", h.Index)
	r.GET("/api/dev/email-preview/:slug", h.Show)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dev/email-preview", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "password-setup") {
		t.Fatalf("index falló: code=%d", w.Code)
	}

	// Cada slug renderiza su HTML con el titular esperado.
	cases := map[string]string{
		"induction-invite": "Completa tu inducción",
		"password-setup":   "¡Aprobaste tu inducción!",
		"password-reset":   "Recuperar Contraseña",
	}
	for slug, marker := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dev/email-preview/"+slug, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), marker) {
			t.Errorf("%s: code=%d, falta %q", slug, w.Code, marker)
		}
	}
}
