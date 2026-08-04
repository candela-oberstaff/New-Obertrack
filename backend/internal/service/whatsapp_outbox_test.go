package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// outboxFakeRepo solo implementa lo que toca el worker. Embebe la interfaz para
// que cualquier otro método que se llamara por error reviente en el test en vez
// de pasar desapercibido.
type outboxFakeRepo struct {
	repository.TicketRepository

	ticket *models.Ticket
	getErr error

	sentID    string
	sentCalls int
	// markSentFailures hace fallar las N primeras llamadas a MarkMessageSent, para
	// ejercitar el caso en que el envío salió pero la fila no se pudo cerrar.
	markSentFailures int
	failedCalls      int
	failedAttempt    int
	failedRetryAt    *time.Time
	rescheduled      int
}

func (f *outboxFakeRepo) GetWithContact(uint) (*models.Ticket, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.ticket, nil
}

func (f *outboxFakeRepo) MarkMessageSent(_ uint, externalID string) error {
	f.sentCalls++
	if f.sentCalls <= f.markSentFailures {
		return errors.New("base no disponible")
	}
	f.sentID = externalID
	return nil
}

func (f *outboxFakeRepo) MarkMessageFailed(_ uint, attempts int, _ string, retryAt *time.Time) error {
	f.failedCalls++
	f.failedAttempt = attempts
	f.failedRetryAt = retryAt
	return nil
}

func (f *outboxFakeRepo) RescheduleMessage(uint, time.Time) error {
	f.rescheduled++
	return nil
}

func newTestTicket() *models.Ticket {
	return &models.Ticket{ID: 1, Contact: &models.Contact{WaID: "112828824473809@lid"}}
}

// newProbeWaha apunta el cliente a un servidor de prueba y desactiva las esperas
// antibaneo, que aquí solo alargarían el test.
func newProbeWaha(url string) *WahaService {
	return &WahaService{
		apiURL:      url,
		session:     "session_1",
		client:      &http.Client{Timeout: 5 * time.Second},
		maxPerMin:   100,
		minInterval: 0,
		humanTyping: false,
	}
}

func TestOutboxMarksSentWithWahaID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"true_112828824473809@lid_ABC"}`))
	}))
	defer srv.Close()

	repo := &outboxFakeRepo{ticket: newTestTicket()}
	o := NewWhatsAppOutbox(repo, newProbeWaha(srv.URL))
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.sentCalls != 1 {
		t.Fatalf("MarkMessageSent llamado %d veces, esperado 1", repo.sentCalls)
	}
	// El ID es lo que evita que el import de historial duplique este mensaje.
	if repo.sentID != "true_112828824473809@lid_ABC" {
		t.Errorf("external_id guardado = %q", repo.sentID)
	}
	if repo.failedCalls != 0 {
		t.Errorf("no debería marcarse como fallido")
	}
}

func TestOutboxRetriesTransientFailureWithBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	repo := &outboxFakeRepo{ticket: newTestTicket()}
	o := NewWhatsAppOutbox(repo, newProbeWaha(srv.URL))
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.failedCalls != 1 || repo.failedAttempt != 1 {
		t.Fatalf("failedCalls=%d attempts=%d, esperado 1 y 1", repo.failedCalls, repo.failedAttempt)
	}
	if repo.failedRetryAt == nil {
		t.Error("un fallo transitorio debe reprogramarse, no agotarse al primer intento")
	}
}

func TestOutboxGivesUpAfterMaxAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	repo := &outboxFakeRepo{ticket: newTestTicket()}
	o := NewWhatsAppOutbox(repo, newProbeWaha(srv.URL))
	o.processMessage(&models.TicketMessage{
		ID: 7, TicketID: 1, Content: "hola",
		DeliveryAttempts: models.DeliveryMaxAttempts - 1,
	})

	if repo.failedRetryAt != nil {
		t.Errorf("agotados los intentos no debe quedar reintento pendiente, got %v", repo.failedRetryAt)
	}
	if repo.failedAttempt != models.DeliveryMaxAttempts {
		t.Errorf("attempts = %d, esperado %d", repo.failedAttempt, models.DeliveryMaxAttempts)
	}
}

func TestOutboxRateLimitPostponesWithoutSpendingAnAttempt(t *testing.T) {
	// El servidor haría falta solo si se llegara a enviar: el límite corta antes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("no debería llegarse a llamar a WAHA estando limitado")
	}))
	defer srv.Close()

	waha := newProbeWaha(srv.URL)
	waha.maxPerMin = 1
	if _, err := waha.reserveSlot(); err != nil { // consume el único turno del minuto
		t.Fatalf("primera reserva: %v", err)
	}

	repo := &outboxFakeRepo{ticket: newTestTicket()}
	o := NewWhatsAppOutbox(repo, waha)
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	// Estar limitado no es un fallo de entrega: si gastara intentos, una ráfaga
	// legítima agotaría los reintentos sin haber tocado WAHA.
	if repo.rescheduled != 1 {
		t.Errorf("RescheduleMessage llamado %d veces, esperado 1", repo.rescheduled)
	}
	if repo.failedCalls != 0 {
		t.Errorf("el límite antibaneo no debe contar como intento fallido")
	}
}

func TestOutboxFailsFastWithoutContact(t *testing.T) {
	repo := &outboxFakeRepo{ticket: &models.Ticket{ID: 1}} // sin Contact
	o := NewWhatsAppOutbox(repo, newProbeWaha("http://127.0.0.1:1"))
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.failedCalls != 1 {
		t.Fatalf("failedCalls=%d, esperado 1", repo.failedCalls)
	}
	// Sin destinatario esperar no arregla nada: se agota sin gastar la ventana.
	if repo.failedRetryAt != nil {
		t.Error("un ticket sin contacto no debe reintentarse")
	}
}

func TestOutboxTicketInexistenteSeAgotaSinReintento(t *testing.T) {
	repo := &outboxFakeRepo{getErr: gorm.ErrRecordNotFound}
	o := NewWhatsAppOutbox(repo, newProbeWaha("http://127.0.0.1:1"))
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.failedCalls != 1 || repo.failedRetryAt != nil {
		t.Errorf("failedCalls=%d retryAt=%v; un ticket que no existe no reaparece", repo.failedCalls, repo.failedRetryAt)
	}
}

func TestOutboxFalloDeBaseSePosponeSinDarPorPerdidoElMensaje(t *testing.T) {
	// Un pool agotado o un failover no dicen nada del mensaje. Tratarlo como
	// "el ticket no tiene contacto" lo marcaría como no entregado para siempre.
	repo := &outboxFakeRepo{getErr: errors.New("connection refused")}
	o := NewWhatsAppOutbox(repo, newProbeWaha("http://127.0.0.1:1"))
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.rescheduled != 1 {
		t.Errorf("RescheduleMessage llamado %d veces, esperado 1", repo.rescheduled)
	}
	if repo.failedCalls != 0 {
		t.Errorf("un fallo de base no debe gastar un intento de entrega")
	}
}

// Un timeout no dice si el mensaje salió. Si WAHA sí lo entregó, reenviarlo se lo
// dejaría duplicado al contacto en el teléfono: hay que cerrarlo como enviado.
func TestOutboxNoReenviaUnMensajeQueElTimeoutOcultoQueSiSeEntrego(t *testing.T) {
	sends := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			// El chat confirma que el mensaje está entregado, con su ID de WAHA.
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `[{"id":"true_ABC","fromMe":true,"body":"hola","timestamp":%d}]`, time.Now().Unix())
			return
		}
		sends++
		time.Sleep(300 * time.Millisecond) // vence el plazo del cliente
	}))
	defer srv.Close()

	waha := newProbeWaha(srv.URL)
	waha.client = &http.Client{Timeout: 100 * time.Millisecond}

	repo := &outboxFakeRepo{ticket: newTestTicket()}
	o := NewWhatsAppOutbox(repo, waha)
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if sends != 1 {
		t.Errorf("sendText llamado %d veces; un envío incierto no se repite a ciegas", sends)
	}
	if repo.sentCalls != 1 || repo.sentID != "true_ABC" {
		t.Errorf("sentCalls=%d id=%q; debería cerrarse con el ID que devolvió el chat", repo.sentCalls, repo.sentID)
	}
	if repo.failedCalls != 0 {
		t.Errorf("un mensaje que sí llegó no puede quedar como fallido")
	}
}

// Si el chat no confirma la entrega, el mensaje vuelve a la cola: perderlo sería
// peor que arriesgar un duplicado que, además, la reconciliación ya descartó.
func TestOutboxEnvioInciertoNoConfirmadoSeReintenta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	waha := newProbeWaha(srv.URL)
	waha.client = &http.Client{Timeout: 100 * time.Millisecond}

	repo := &outboxFakeRepo{ticket: newTestTicket()}
	o := NewWhatsAppOutbox(repo, waha)
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.sentCalls != 0 {
		t.Errorf("sin confirmación no se puede dar por enviado")
	}
	if repo.failedCalls != 1 || repo.failedRetryAt == nil {
		t.Errorf("failedCalls=%d retryAt=%v; debería quedar reprogramado", repo.failedCalls, repo.failedRetryAt)
	}
}

// El envío salió pero no se pudo cerrar la fila: se insiste antes de rendirse,
// porque dejarlo en 'pending' hace que la vuelta siguiente lo entregue otra vez.
func TestOutboxInsisteEnMarcarComoEnviado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"true_ABC"}`))
	}))
	defer srv.Close()

	repo := &outboxFakeRepo{ticket: newTestTicket(), markSentFailures: 2}
	o := NewWhatsAppOutbox(repo, newProbeWaha(srv.URL))
	o.processMessage(&models.TicketMessage{ID: 7, TicketID: 1, Content: "hola"})

	if repo.sentCalls != 3 {
		t.Errorf("MarkMessageSent llamado %d veces, esperado 3 (dos fallos + el bueno)", repo.sentCalls)
	}
	if repo.sentID != "true_ABC" {
		t.Errorf("external_id guardado = %q", repo.sentID)
	}
}

// Los reintentos también pasan por el gate antibaneo: si no, un hipo de WAHA
// produce justo la ráfaga que el gate existe para evitar.
func TestReintentoRespetaElLimiteAntibaneo(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	waha := newProbeWaha(srv.URL)
	waha.maxPerMin = 1 // solo alcanza para el primer intento

	if _, err := waha.SendMessage("session_1", "123@c.us", "hola"); err == nil {
		t.Fatal("esperado el error del 500")
	}
	if calls != 1 {
		t.Errorf("WAHA recibió %d envíos; el reintento debería haberse quedado sin turno", calls)
	}
}

func TestDeliveryRetryDelaySaturates(t *testing.T) {
	// Un contador fuera de rango no debe desbordar la tabla de backoff.
	if d := models.DeliveryRetryDelay(0); d != 15*time.Second {
		t.Errorf("delay(0) = %v, esperado el primer escalón", d)
	}
	if d, saturated := models.DeliveryRetryDelay(99), models.DeliveryRetryDelay(models.DeliveryMaxAttempts); d != saturated {
		t.Errorf("delay(99) = %v, debería saturar en %v", d, saturated)
	}
	if models.DeliveryRetryDelay(1) >= models.DeliveryRetryDelay(3) {
		t.Error("el backoff debe escalar, no ser plano")
	}
}
