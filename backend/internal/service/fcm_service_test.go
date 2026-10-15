package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeFCMServer hace de servidor de OAuth y de FCM a la vez.
func fakeFCMServer(t *testing.T, sendStatus int, sendBody string) (*httptest.Server, *int32, *map[string]any) {
	t.Helper()
	var tokenCalls int32
	sent := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			atomic.AddInt32(&tokenCalls, 1)
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || r.Form.Get("assertion") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-123","expires_in":3600}`))
		case "/send":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&sent)
			w.WriteHeader(sendStatus)
			_, _ = w.Write([]byte(sendBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &tokenCalls, &sent
}

func newTestSender(t *testing.T, srv *httptest.Server) *FCMSender {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	sa, _ := json.Marshal(map[string]string{
		"project_id":   "obertrack-test",
		"client_email": "push@obertrack-test.iam.gserviceaccount.com",
		"private_key":  string(pemKey),
		"token_uri":    srv.URL + "/token",
	})
	s, err := NewFCMSender(sa)
	if err != nil {
		t.Fatal(err)
	}
	s.sendURL = srv.URL + "/send"
	return s
}

func TestFCMSendArmaElMensajeYReusaElToken(t *testing.T) {
	srv, tokenCalls, sent := fakeFCMServer(t, http.StatusOK, `{"name":"projects/x/messages/1"}`)
	s := newTestSender(t, srv)

	data := map[string]string{"notification_id": "7", "type": "task_assigned"}
	for i := 0; i < 2; i++ {
		if err := s.Send("device-abc", "Nueva tarea", "Te asignaron algo", data); err != nil {
			t.Fatalf("Send() = %v", err)
		}
	}
	// El access token se pide una vez y se reutiliza.
	if n := atomic.LoadInt32(tokenCalls); n != 1 {
		t.Fatalf("se pidiÃ³ el token %d veces, esperaba 1", n)
	}
	msg, _ := (*sent)["message"].(map[string]any)
	if msg["token"] != "device-abc" {
		t.Fatalf("token del mensaje = %v", msg["token"])
	}
	notif, _ := msg["notification"].(map[string]any)
	if notif["title"] != "Nueva tarea" || notif["body"] != "Te asignaron algo" {
		t.Fatalf("notification = %v", notif)
	}
	got, _ := msg["data"].(map[string]any)
	if got["notification_id"] != "7" || got["type"] != "task_assigned" {
		t.Fatalf("data = %v", got)
	}
}

func TestFCMTokenInexistenteSeReconoce(t *testing.T) {
	body := `{"error":{"code":404,"status":"NOT_FOUND","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`
	srv, _, _ := fakeFCMServer(t, http.StatusNotFound, body)
	s := newTestSender(t, srv)
	if err := s.Send("muerto", "t", "b", nil); !errors.Is(err, ErrFCMTokenGone) {
		t.Fatalf("Send() = %v, esperaba ErrFCMTokenGone", err)
	}
}

func TestFCMTokenMalFormadoSeReconoce(t *testing.T) {
	// Respuesta real de FCM a un token que no tiene formato de token.
	body := `{"error":{"code":400,"status":"INVALID_ARGUMENT","details":[` +
		`{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"},` +
		`{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.token"}]}]}}`
	srv, _, _ := fakeFCMServer(t, http.StatusBadRequest, body)
	s := newTestSender(t, srv)
	if err := s.Send("basura", "t", "b", nil); !errors.Is(err, ErrFCMTokenGone) {
		t.Fatalf("Send() = %v, esperaba ErrFCMTokenGone", err)
	}
}

func TestFCMOtroErrorNoBorraElToken(t *testing.T) {
	// Un 400 por un payload mal armado no significa que el telÃ©fono no exista.
	body := `{"error":{"code":400,"status":"INVALID_ARGUMENT","details":[{"errorCode":"INVALID_ARGUMENT"}]}}`
	srv, _, _ := fakeFCMServer(t, http.StatusBadRequest, body)
	s := newTestSender(t, srv)
	err := s.Send("vivo", "t", "b", nil)
	if err == nil || errors.Is(err, ErrFCMTokenGone) {
		t.Fatalf("Send() = %v, esperaba un error que NO sea ErrFCMTokenGone", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("el error deberÃ­a decir el cÃ³digo: %v", err)
	}
}

func TestFCMCuentaDeServicioInvalida(t *testing.T) {
	if _, err := NewFCMSender([]byte(`{"project_id":"x"}`)); err == nil {
		t.Fatal("esperaba error sin client_email ni private_key")
	}
	if _, err := NewFCMSender([]byte(`{"project_id":"x","client_email":"a","private_key":"no es pem"}`)); err == nil {
		t.Fatal("esperaba error con una clave que no es PEM")
	}
}

func TestFCMSinConfiguracionQuedaApagado(t *testing.T) {
	t.Setenv("FIREBASE_CREDENTIALS_JSON", "")
	t.Setenv("FIREBASE_CREDENTIALS_FILE", "")
	if NewFCMSenderFromEnv() != nil {
		t.Fatal("sin variables el push mÃ³vil debe quedar apagado (nil)")
	}
}
