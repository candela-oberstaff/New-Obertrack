package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// Los procesos de reclutamiento leídos de Obersuite. Lo que se fija: que un
// espejo caído o sin configurar NUNCA tumba la ficha (200 con available:false),
// que un 404 de Obersuite es lista vacía y no error, y que el JSON suyo viaja
// tal cual sin reinterpretar.

type fakeObersuite struct {
	configured bool
	body       string
	err        error
	attBody    string
	pedidoPor  string
}

func (f *fakeObersuite) Configured() bool { return f.configured }
func (f *fakeObersuite) CompanySubscriptions(_ uint) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(f.body), nil
}
func (f *fakeObersuite) SubscriptionAttachment(sid, _ string) (io.ReadCloser, string, string, error) {
	f.pedidoPor = "partes:" + sid
	if f.err != nil {
		return nil, "", "", f.err
	}
	return io.NopCloser(strings.NewReader(f.attBody)), "application/pdf", `attachment; filename="x.pdf"`, nil
}

func (f *fakeObersuite) AttachmentByURL(raw string) (io.ReadCloser, string, string, error) {
	f.pedidoPor = "url:" + raw
	if !strings.HasPrefix(raw, "https://obersuite.oberstaff.com/api/integrations/obertrack/") {
		return nil, "", "", service.ErrNotAnObersuiteURL
	}
	if f.err != nil {
		return nil, "", "", f.err
	}
	return io.NopCloser(strings.NewReader(f.attBody)), "application/pdf", `attachment; filename="x.pdf"`, nil
}

func getSubs(t *testing.T, client service.ObersuiteClient) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "36"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/tenants/36/subscriptions", nil)
	NewTenantSubscriptionsHandler(client).List(c)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func TestProcesos_ElJSONDeObersuiteViajaTalCual(t *testing.T) {
	w, out := getSubs(t, &fakeObersuite{configured: true, body: `{"company_id":36,"subscriptions":[{"external_id":"s1","stage":{"index":1}}],"stages":[],"counts":{"active":1}}`})
	if w.Code != 200 || out["available"] != true {
		t.Fatalf("code=%d out=%v", w.Code, out)
	}
	data := out["data"].(map[string]any)
	if data["company_id"].(float64) != 36 || len(data["subscriptions"].([]any)) != 1 {
		t.Fatalf("el JSON de Obersuite no llegó entero: %v", data)
	}
}

func TestProcesos_SinConfigurarLaFichaAbreIgual(t *testing.T) {
	w, out := getSubs(t, &fakeObersuite{configured: false})
	if w.Code != 200 || out["available"] != false || out["reason"] != "not_configured" {
		t.Fatalf("code=%d out=%v", w.Code, out)
	}
}

func TestProcesos_ObersuiteCaidoNoEs500Nuestro(t *testing.T) {
	w, out := getSubs(t, &fakeObersuite{configured: true, err: errors.New("obersuite no responde: timeout")})
	if w.Code != 200 || out["available"] != false || out["reason"] != "unreachable" {
		t.Fatalf("un espejo caído no tumba la ficha: code=%d out=%v", w.Code, out)
	}
}

func TestProcesos_Un404DeObersuiteEsListaVacia(t *testing.T) {
	w, out := getSubs(t, &fakeObersuite{configured: true, err: service.ObersuiteNotFoundForTests()})
	if w.Code != 200 || out["available"] != true || len(out["subscriptions"].([]any)) != 0 {
		t.Fatalf("code=%d out=%v", w.Code, out)
	}
}

func TestProcesos_ElAdjuntoSeReenviaConSuNombreYTipo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "36"}, {Key: "sid", Value: "s1"}, {Key: "aid", Value: "a1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	NewTenantSubscriptionsHandler(&fakeObersuite{configured: true, attBody: "%PDF-1.4 hola"}).Attachment(c)

	if w.Code != 200 || w.Body.String() != "%PDF-1.4 hola" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/pdf" || !strings.Contains(w.Header().Get("Content-Disposition"), "x.pdf") {
		t.Fatalf("cabeceras: %v", w.Header())
	}
}

// El adjunto se pide por la URL que publica Obersuite cuando viene: sus ids de
// ruta no son el external_id de la suscripción, y construirla nosotros daba 400.
func TestProcesos_ElAdjuntoSePidePorLaURLDeObersuite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeObersuite{configured: true, attBody: "%PDF"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "36"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/x?url=https://obersuite.oberstaff.com/api/integrations/obertrack/subscriptions/b26/attachments/72a", nil)
	NewTenantSubscriptionsHandler(fake).Attachment(c)

	if w.Code != 200 || !strings.HasPrefix(fake.pedidoPor, "url:") {
		t.Fatalf("code=%d pedidoPor=%q", w.Code, fake.pedidoPor)
	}
}

// Sin `url` (un adjunto viejo) se cae al camino por partes, que recorta el
// prefijo del external_id en el cliente.
func TestProcesos_SinURLCaeAlCaminoPorPartes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeObersuite{configured: true, attBody: "%PDF"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "36"}, {Key: "sid", Value: "obersuite-subscription-b26"}, {Key: "aid", Value: "72a"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	NewTenantSubscriptionsHandler(fake).Attachment(c)

	if w.Code != 200 || fake.pedidoPor != "partes:obersuite-subscription-b26" {
		t.Fatalf("code=%d pedidoPor=%q", w.Code, fake.pedidoPor)
	}
}

// Un enlace que no es de Obersuite es un dato malo de quien llama: 400, no el
// 502 de "Obersuite falló", que mandaría a mirar el sistema equivocado.
func TestProcesos_UnEnlaceAjenoEs400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "36"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/x?url=https://otro-dominio.com/algo", nil)
	NewTenantSubscriptionsHandler(&fakeObersuite{configured: true}).Attachment(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d, se esperaba 400", w.Code)
	}
}
