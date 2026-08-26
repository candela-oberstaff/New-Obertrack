package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Envío masivo. Antes se mandaba de una en una: quinientos destinatarios eran
// quinientas llamadas seguidas a Brevo, con quien pulsó "enviar" esperando delante de
// la pantalla. Lo que se fija aquí es que agrupe de verdad, que cada persona siga
// recibiendo una copia SEPARADA —nadie ve las direcciones de los demás— y que un lote
// rechazado no se cuente como enviado.

// brevoFalso levanta un servidor que hace de API de Brevo y guarda lo que recibe.
func brevoFalso(t *testing.T, status int) (*BrevoService, *[]BrevoBatchRequest) {
	t.Helper()
	recibidas := []BrevoBatchRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cuerpo, _ := io.ReadAll(r.Body)
		var req BrevoBatchRequest
		if err := json.Unmarshal(cuerpo, &req); err != nil {
			t.Errorf("payload ilegible: %v", err)
		}
		recibidas = append(recibidas, req)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"messageId":"<abc@brevo>"}`))
	}))
	t.Cleanup(srv.Close)

	s := &BrevoService{apiKey: "clave-de-prueba", apiURL: srv.URL, from: BrevoContact{Name: "Obertrack", Email: "noreply@obertrack.com"}}
	return s, &recibidas
}

func destinatarios(n int) []BatchRecipient {
	out := make([]BatchRecipient, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, BatchRecipient{Email: fmt.Sprintf("p%d@example.com", i), Name: fmt.Sprintf("Persona %d", i)})
	}
	return out
}

func TestLote_MandaMuchosEnPocasPeticiones(t *testing.T) {
	s, recibidas := brevoFalso(t, http.StatusCreated)

	enviados, fallos := s.SendBatchKind(EmailKindCampaign, destinatarios(500), "Novedades", "<p>Hola</p>", nil)

	if enviados != 500 || len(fallos) != 0 {
		t.Fatalf("deberían salir los 500: enviados=%d fallos=%v", enviados, fallos)
	}
	// Con contenido idéntico caben 500 por petición: una sola llamada.
	if len(*recibidas) != 1 {
		t.Fatalf("500 iguales deberían ir en UNA petición, got %d", len(*recibidas))
	}
	if len((*recibidas)[0].MessageVersions) != 500 {
		t.Fatalf("faltan versiones: %d", len((*recibidas)[0].MessageVersions))
	}
}

// Lo que no se puede perder al agrupar: cada persona en su propia versión. Meter a
// varios en un mismo `to` les enseñaría las direcciones de los demás.
func TestLote_CadaPersonaRecibeSuCopiaSeparada(t *testing.T) {
	s, recibidas := brevoFalso(t, http.StatusCreated)

	s.SendBatchKind(EmailKindCampaign, destinatarios(3), "Novedades", "<p>Hola</p>", nil)

	for _, v := range (*recibidas)[0].MessageVersions {
		if len(v.To) != 1 {
			t.Fatalf("cada versión lleva UN destinatario, got %d", len(v.To))
		}
	}
}

// Con cuerpos personalizados el lote es más pequeño: cada versión arrastra la
// plantilla entera y una petición con mil sería de decenas de megas.
func TestLote_ConContenidoPropioTroceaMasFino(t *testing.T) {
	s, recibidas := brevoFalso(t, http.StatusCreated)

	lista := destinatarios(120)
	for i := range lista {
		lista[i].HTML = fmt.Sprintf("<p>Hola %s</p>", lista[i].Name)
	}
	enviados, _ := s.SendBatchKind(EmailKindCampaign, lista, "Novedades", "<p>Hola</p>", nil)

	if enviados != 120 {
		t.Fatalf("enviados=%d", enviados)
	}
	if len(*recibidas) != 3 {
		t.Fatalf("120 personalizados en lotes de 50 son 3 peticiones, got %d", len(*recibidas))
	}
	// Y el global viaja igualmente: la API exige que exista para aceptar el de cada
	// versión.
	if (*recibidas)[0].HTMLContent == "" {
		t.Fatal("falta el htmlContent global")
	}
	if !strings.Contains((*recibidas)[0].MessageVersions[0].HTMLContent, "Persona 0") {
		t.Fatal("la versión debería llevar su cuerpo personalizado")
	}
}

// Un lote rechazado no se cuenta como enviado: la API acepta o rechaza la petición
// entera, y dar por buenos a los suyos sería mentir en el informe que ve quien envía.
func TestLote_UnLoteRechazadoCuentaComoFalloDeTodosLosSuyos(t *testing.T) {
	s, _ := brevoFalso(t, http.StatusTooManyRequests)

	enviados, fallos := s.SendBatchKind(EmailKindCampaign, destinatarios(3), "Novedades", "<p>Hola</p>", nil)

	if enviados != 0 {
		t.Fatalf("nada salió: enviados=%d", enviados)
	}
	if len(fallos) != 3 {
		t.Fatalf("los tres tienen que aparecer como fallidos, got %v", fallos)
	}
}

// Y sigue respetando el interruptor: apagar el tipo apaga también el envío masivo.
func TestLote_RespetaElInterruptor(t *testing.T) {
	s, recibidas := brevoFalso(t, http.StatusCreated)
	s.SetKindGate(func(kind string) bool { return false })

	enviados, fallos := s.SendBatchKind(EmailKindCampaign, destinatarios(10), "Novedades", "<p>Hola</p>", nil)

	if enviados != 0 || len(fallos) != 0 {
		t.Fatalf("apagado no envía ni reporta fallos: %d / %v", enviados, fallos)
	}
	if len(*recibidas) != 0 {
		t.Fatal("no debería haber llamado a Brevo")
	}
}
