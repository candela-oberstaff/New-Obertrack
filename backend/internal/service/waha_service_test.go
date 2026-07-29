package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
)

func TestIsIndividualChat(t *testing.T) {
	cases := map[string]bool{
		"17873491050@c.us":        true,
		"112828824473809@lid":     true, // WhatsApp's current 1:1 identifier
		"120363217324508940@g.us": false,
		"status@broadcast":        false,
		"":                        false,
	}
	for chatID, want := range cases {
		if got := IsIndividualChat(chatID); got != want {
			t.Errorf("IsIndividualChat(%q) = %v, want %v", chatID, got, want)
		}
	}
}

func TestMediaPlaceholder(t *testing.T) {
	cases := []struct {
		name     string
		msgType  string
		mimetype string
		hasMedia bool
		want     string
	}{
		{"explicit type", "image", "", true, "📷 Imagen recibida"},
		{"voice note", "ptt", "", true, "🎤 Nota de voz recibida"},
		// The WEBJS engine returns type:null on history messages, so the mimetype
		// (and failing that, hasMedia) has to carry the classification.
		{"type missing, mimetype present", "", "application/pdf", true, "📄 Documento recibido"},
		{"type and mimetype missing", "", "", true, "📎 Archivo adjunto recibido"},
		{"plain empty message is not media", "chat", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MediaPlaceholder(tc.msgType, tc.mimetype, tc.hasMedia); got != tc.want {
				t.Errorf("MediaPlaceholder(%q, %q, %v) = %q, want %q",
					tc.msgType, tc.mimetype, tc.hasMedia, got, tc.want)
			}
		})
	}
}

func TestWahaSendResponseMessageID(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"string id", `{"id":"true_1234@c.us_ABC"}`, "true_1234@c.us_ABC"},
		{"object id", `{"id":{"_serialized":"true_1234@c.us_ABC"}}`, "true_1234@c.us_ABC"},
		{"absent id", `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp wahaSendResponse
			if err := json.Unmarshal([]byte(tc.body), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := resp.messageID(); got != tc.want {
				t.Errorf("messageID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsRetryableSendErr(t *testing.T) {
	if !isRetryableSendErr(errors.New("connection reset")) {
		t.Error("transport errors should be retried")
	}
	if !isRetryableSendErr(&wahaHTTPError{status: http.StatusBadGateway}) {
		t.Error("5xx should be retried")
	}
	if !isRetryableSendErr(&wahaHTTPError{status: http.StatusTooManyRequests}) {
		t.Error("429 should be retried")
	}
	// 422 is WAHA's "session does not exist" — retrying cannot help.
	if isRetryableSendErr(&wahaHTTPError{status: http.StatusUnprocessableEntity}) {
		t.Error("422 should not be retried")
	}
}

func TestReserveSlotSpacesSendsAndCapsPerMinute(t *testing.T) {
	s := &WahaService{maxPerMin: 3, minInterval: 100 * time.Millisecond}

	first, err := s.reserveSlot()
	if err != nil {
		t.Fatalf("first reservation: %v", err)
	}
	second, err := s.reserveSlot()
	if err != nil {
		t.Fatalf("second reservation: %v", err)
	}
	// Slots are handed out spaced apart, without the caller having slept yet —
	// that is what lets concurrent senders wait in parallel instead of behind a
	// mutex held during a sleep.
	if gap := second.Sub(first); gap < s.minInterval {
		t.Errorf("consecutive slots spaced %v apart, want >= %v", gap, s.minInterval)
	}

	if _, err := s.reserveSlot(); err != nil {
		t.Fatalf("third reservation: %v", err)
	}
	if _, err := s.reserveSlot(); !errors.Is(err, apperrors.ErrRateLimited) {
		t.Errorf("4th reservation over a cap of 3: got %v, want ErrRateLimited", err)
	}
}

func TestReserveSlotDoesNotBlock(t *testing.T) {
	s := &WahaService{maxPerMin: 50, minInterval: 2 * time.Second}

	start := time.Now()
	for i := 0; i < 5; i++ {
		if _, err := s.reserveSlot(); err != nil {
			t.Fatalf("reservation %d: %v", i, err)
		}
	}
	// 5 reservations at 2s spacing schedule 8s of future waiting, but reserving
	// them must itself be instant.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("reserving 5 slots took %v; reserveSlot must not sleep under the lock", elapsed)
	}
}

func TestRealPhoneResolvesLidContact(t *testing.T) {
	// Shape returned by GET /api/contacts?contactId=<lid>@lid: `id` carries the
	// real phone JID while `number` carries the LID.
	var c WahaContactResponse
	body := `{"id":"17873491050@c.us","number":"112828824473809","name":"Edgardo Vázquez"}`
	if err := json.Unmarshal([]byte(body), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := c.RealPhone(); got != "17873491050" {
		t.Errorf("RealPhone() = %q, want the phone from `id`, not the LID", got)
	}
	if got := c.BestName(); got != "Edgardo Vázquez" {
		t.Errorf("BestName() = %q", got)
	}
}
