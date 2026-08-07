package service

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// WebPushService envía notificaciones push del navegador (pestaña cerrada
// incluida). Complementa al correo de respaldo: el push llega al instante al
// navegador donde el usuario aceptó recibirlas.
//
// El par VAPID se genera UNA vez y se persiste en la base: cambiarlo
// invalidaría todas las suscripciones existentes.
type WebPushService struct {
	repo repository.PushRepository

	mu   sync.Mutex
	keys *models.WebPushKeys
}

func NewWebPushService(repo repository.PushRepository) *WebPushService {
	return &WebPushService{repo: repo}
}

// ensureKeys carga (o genera y persiste) el par VAPID. Perezoso y cacheado.
func (s *WebPushService) ensureKeys() *models.WebPushKeys {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys != nil {
		return s.keys
	}
	keys, err := s.repo.GetKeys()
	if err != nil {
		log.Printf("[webpush] no se pudieron leer las claves VAPID: %v", err)
		return nil
	}
	if keys == nil {
		priv, pub, gerr := webpush.GenerateVAPIDKeys()
		if gerr != nil {
			log.Printf("[webpush] no se pudieron generar las claves VAPID: %v", gerr)
			return nil
		}
		keys = &models.WebPushKeys{PublicKey: pub, PrivateKey: priv}
		if serr := s.repo.SaveKeys(keys); serr != nil {
			log.Printf("[webpush] no se pudieron guardar las claves VAPID: %v", serr)
			return nil
		}
		log.Println("[webpush] par VAPID generado y persistido")
	}
	s.keys = keys
	return s.keys
}

// PublicKey expone la clave pública VAPID que el navegador necesita para
// suscribirse. Vacía si el servicio no pudo inicializarse.
func (s *WebPushService) PublicKey() string {
	if k := s.ensureKeys(); k != nil {
		return k.PublicKey
	}
	return ""
}

// Subscribe registra la suscripción de un navegador para el usuario.
func (s *WebPushService) Subscribe(userID uint, endpoint, p256dh, auth string) error {
	return s.repo.UpsertSubscription(&models.PushSubscription{
		UserID:   userID,
		Endpoint: endpoint,
		P256dh:   p256dh,
		Auth:     auth,
	})
}

// Unsubscribe elimina la suscripción de un endpoint para el usuario.
func (s *WebPushService) Unsubscribe(userID uint, endpoint string) error {
	return s.repo.DeleteSubscription(userID, endpoint)
}

// vapidSubject es el contacto que exige el protocolo VAPID (RFC 8292).
func vapidSubject() string {
	if subject := strings.TrimSpace(os.Getenv("VAPID_SUBJECT")); subject != "" {
		return subject
	}
	return "mailto:notificaciones@obertrack.app"
}

type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// SendToUser empuja la notificación a TODOS los navegadores suscritos del
// usuario. Best-effort: los fallos se loguean, y un endpoint muerto (404/410
// del push service: el usuario revocó el permiso o borró el perfil del
// navegador) se elimina para no insistirle.
//
// Pensado para invocarse en una goroutine — hace HTTP hacia los push services.
func (s *WebPushService) SendToUser(userID uint, title, body, link string) {
	keys := s.ensureKeys()
	if keys == nil {
		return
	}
	subs, err := s.repo.ListSubscriptionsByUser(userID)
	if err != nil || len(subs) == 0 {
		return
	}

	url := link
	if url == "" {
		url = "/"
	}
	if base := frontendBaseURL(); base != "" && strings.HasPrefix(url, "/") {
		url = base + url
	}
	payload, _ := json.Marshal(pushPayload{Title: title, Body: body, URL: url})

	for _, sub := range subs {
		resp, serr := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
		}, &webpush.Options{
			Subscriber:      vapidSubject(),
			VAPIDPublicKey:  keys.PublicKey,
			VAPIDPrivateKey: keys.PrivateKey,
			TTL:             int((12 * time.Hour).Seconds()),
		})
		if serr != nil {
			log.Printf("[webpush] envío fallido a usuario %d: %v", userID, serr)
			continue
		}
		if resp != nil {
			if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
				// Suscripción muerta: el navegador la revocó. Se limpia.
				_ = s.repo.DeleteByEndpoint(sub.Endpoint)
			}
			resp.Body.Close()
		}
	}
}
