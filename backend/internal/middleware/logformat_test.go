package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRedactedLogFormatterHidesCRMToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: RedactedLogFormatter, Output: &buf}))
	r.GET("/api/auth/crm", func(c *gin.Context) { c.Status(http.StatusSeeOther) })
	r.GET("/api/tasks", func(c *gin.Context) { c.Status(http.StatusOK) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/auth/crm?token=eyJ.SECRETO.firma", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/tasks?board_id=7", nil))

	out := buf.String()
	if strings.Contains(out, "SECRETO") {
		t.Fatalf("el token del CRM llegó al log:\n%s", out)
	}
	if !strings.Contains(out, `"/api/auth/crm?[redacted]"`) {
		t.Errorf("la línea del canje debe seguir apareciendo, sin el query:\n%s", out)
	}
	if !strings.Contains(out, `"/api/tasks?board_id=7"`) {
		t.Errorf("el resto de rutas conserva su query:\n%s", out)
	}
}
