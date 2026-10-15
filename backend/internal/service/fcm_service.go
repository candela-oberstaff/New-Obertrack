package service

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// FCMSender envía notificaciones a la app móvil por Firebase Cloud Messaging
// (API HTTP v1). Se autentica con la cuenta de servicio del proyecto Firebase:
// firma un JWT con su clave privada y lo cambia por un access token de OAuth,
// que se reutiliza hasta poco antes de vencer.
//
// La cuenta de servicio llega por entorno y nunca vive en el repo:
//   - FIREBASE_CREDENTIALS_FILE: ruta al JSON descargado de Firebase, o
//   - FIREBASE_CREDENTIALS_JSON: el contenido del JSON.
//
// Sin ninguna de las dos, NewFCMSenderFromEnv devuelve nil y el push móvil
// queda apagado: el resto de canales (WebSocket, Web Push, correo) siguen.
type FCMSender struct {
	projectID   string
	clientEmail string
	tokenURL    string
	sendURL     string
	key         *rsa.PrivateKey
	client      *http.Client

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// ErrFCMTokenGone indica que FCM ya no reconoce el token: la app se
// desinstaló o el token se renovó. Quien envía debe borrarlo.
var ErrFCMTokenGone = errors.New("token FCM inexistente")

type fcmServiceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// NewFCMSenderFromEnv arma el emisor con la cuenta de servicio del entorno, o
// devuelve nil si no está configurada o no se puede leer.
func NewFCMSenderFromEnv() *FCMSender {
	raw := []byte(strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_JSON")))
	if len(raw) == 0 {
		path := strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_FILE"))
		if path == "" {
			log.Println("[fcm] sin FIREBASE_CREDENTIALS_FILE ni FIREBASE_CREDENTIALS_JSON: push móvil apagado")
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[fcm] no se pudo leer la cuenta de servicio (%s): %v", path, err)
			return nil
		}
		raw = b
	}
	s, err := NewFCMSender(raw)
	if err != nil {
		log.Printf("[fcm] cuenta de servicio inválida: %v", err)
		return nil
	}
	log.Printf("[fcm] push móvil activo (proyecto %s)", s.projectID)
	return s
}

// NewFCMSender arma el emisor a partir del JSON de la cuenta de servicio.
func NewFCMSender(serviceAccountJSON []byte) (*FCMSender, error) {
	var sa fcmServiceAccount
	if err := json.Unmarshal(serviceAccountJSON, &sa); err != nil {
		return nil, err
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("faltan project_id, client_email o private_key")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("private_key: %w", err)
	}
	tokenURL := sa.TokenURI
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	return &FCMSender{
		projectID:   sa.ProjectID,
		clientEmail: sa.ClientEmail,
		tokenURL:    tokenURL,
		sendURL:     fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", sa.ProjectID),
		key:         key,
		client:      &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// token devuelve un access token vigente, pidiendo uno nuevo si hace falta.
func (s *FCMSender) token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && time.Until(s.expiry) > time.Minute {
		return s.accessToken, nil
	}

	now := time.Now()
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   s.clientEmail,
		"scope": fcmScope,
		"aud":   s.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}).SignedString(s.key)
	if err != nil {
		return "", err
	}

	resp, err := s.client.PostForm(s.tokenURL, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("oauth %d: %s", resp.StatusCode, body)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("oauth sin access_token")
	}
	s.accessToken = out.AccessToken
	s.expiry = now.Add(time.Duration(out.ExpiresIn) * time.Second)
	return s.accessToken, nil
}

// Send manda una notificación a un teléfono. Devuelve ErrFCMTokenGone cuando
// FCM ya no conoce el token.
//
// Va como mensaje "notification" (Android la muestra solo con la app cerrada o
// en segundo plano) con el `data` que la app usa al tocarla. Con la app abierta
// Android no la enseña: ahí el aviso ya entró por el WebSocket.
func (s *FCMSender) Send(deviceToken, title, body string, data map[string]string) error {
	access, err := s.token()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"message": map[string]any{
			"token":        deviceToken,
			"notification": map[string]string{"title": title, "body": body},
			"data":         data,
			"android": map[string]any{
				"priority": "HIGH",
				// Avisos que caducan: no tiene sentido entregar uno de hace días.
				"ttl": "43200s",
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, s.sendURL, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if fcmTokenGone(resp.StatusCode, raw) {
		return ErrFCMTokenGone
	}
	return fmt.Errorf("fcm %d: %s", resp.StatusCode, raw)
}

// fcmTokenGone reconoce las respuestas de FCM que significan "este token no
// sirve": 404 / UNREGISTERED (la app se desinstaló) o un 400 que señala el
// campo message.token (token mal formado). Un 400 por otra causa (payload mal
// armado) NO borra el token.
func fcmTokenGone(status int, body []byte) bool {
	if status == http.StatusNotFound {
		return true
	}
	var e struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				ErrorCode       string `json:"errorCode"`
				FieldViolations []struct {
					Field string `json:"field"`
				} `json:"fieldViolations"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil {
		return false
	}
	if e.Error.Status == "NOT_FOUND" {
		return true
	}
	for _, d := range e.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return true
		}
		for _, v := range d.FieldViolations {
			if v.Field == "message.token" {
				return true
			}
		}
	}
	return false
}
