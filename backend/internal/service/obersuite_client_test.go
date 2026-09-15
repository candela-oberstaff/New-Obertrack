package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// El cliente hacia Obersuite, contra un servidor que hace de Obersuite. Lo que
// se fija: que el token viaja en la cabecera acordada, que un 401 se explica
// (es configuración), que un 404 se distingue de un fallo, y que sin
// configurar no sale ni una petición.

func obersuiteFalso(t *testing.T, status int, body string) *obersuiteClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &obersuiteClient{baseURL: srv.URL, token: "tok-123", http: &http.Client{Timeout: 2 * time.Second}}
}

func TestClienteObersuite_MandaElTokenEnLaCabeceraAcordada(t *testing.T) {
	var cabecera, ruta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cabecera, ruta = r.Header.Get("X-Service-Token"), r.URL.Path
		_, _ = w.Write([]byte(`{"company_id":36,"subscriptions":[]}`))
	}))
	t.Cleanup(srv.Close)
	c := &obersuiteClient{baseURL: srv.URL + "/", token: "tok-123", http: &http.Client{Timeout: 2 * time.Second}}

	raw, err := c.CompanySubscriptions(36)

	if err != nil || len(raw) == 0 {
		t.Fatalf("%v %s", err, raw)
	}
	if cabecera != "tok-123" {
		t.Errorf("X-Service-Token = %q", cabecera)
	}
	if ruta != "/api/integrations/obertrack/companies/36/subscriptions" {
		t.Errorf("ruta %q (la barra final de la URL base no debe duplicarse)", ruta)
	}
}

func TestClienteObersuite_Un401SeExplicaComoConfiguracion(t *testing.T) {
	c := obersuiteFalso(t, 401, `{"error":"bad token"}`)
	_, err := c.CompanySubscriptions(36)
	if err == nil || !strings.Contains(err.Error(), "OBERSUITE_API_TOKEN") {
		t.Fatalf("un 401 tiene que apuntar a la variable: %v", err)
	}
}

func TestClienteObersuite_Un404SeDistingueDeUnFallo(t *testing.T) {
	c := obersuiteFalso(t, 404, `{"error":"no"}`)
	_, err := c.CompanySubscriptions(36)
	if !IsObersuiteNotFound(err) {
		t.Fatalf("got %v", err)
	}
	c2 := obersuiteFalso(t, 500, `boom`)
	_, err = c2.CompanySubscriptions(36)
	if err == nil || IsObersuiteNotFound(err) {
		t.Fatalf("un 500 no es un 404: %v", err)
	}
}

func TestClienteObersuite_LoQueNoEsJSONSeRechaza(t *testing.T) {
	c := obersuiteFalso(t, 200, `<html>login</html>`)
	if _, err := c.CompanySubscriptions(36); err == nil {
		t.Fatal("una página HTML (p. ej. un proxy devolviendo el login) no puede pasar por datos")
	}
}

func TestClienteObersuite_SinConfigurarNoSaleNiUnaPeticion(t *testing.T) {
	c := &obersuiteClient{http: &http.Client{Timeout: time.Second}}
	if c.Configured() {
		t.Fatal("sin URL ni token no está configurado")
	}
	if _, err := c.CompanySubscriptions(36); err != ErrObersuiteNotConfigured {
		t.Fatalf("got %v", err)
	}
}
