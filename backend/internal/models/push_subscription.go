package models

import "time"

// PushSubscription es una suscripción Web Push de UN navegador de UN usuario.
// El endpoint identifica al navegador (lo emite su push service: FCM, Mozilla,
// etc.); p256dh y auth son las claves con las que se cifra cada payload
// (RFC 8291). Un usuario puede tener varias filas (PC, portátil, teléfono).
type PushSubscription struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Endpoint  string    `gorm:"size:500;uniqueIndex" json:"endpoint"`
	P256dh    string    `gorm:"size:255;not null" json:"-"`
	Auth      string    `gorm:"size:255;not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

func (PushSubscription) TableName() string {
	return "push_subscriptions"
}

// WebPushKeys es el par VAPID del servidor (una sola fila). Se genera solo en
// el primer arranque y se persiste: si cambiara, TODAS las suscripciones de
// los navegadores quedarían inválidas, por eso vive en la base y no en una
// variable de entorno que alguien puede olvidar al mover el despliegue.
type WebPushKeys struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PublicKey  string    `gorm:"size:255;not null" json:"public_key"`
	PrivateKey string    `gorm:"size:255;not null" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

func (WebPushKeys) TableName() string {
	return "web_push_keys"
}

// DeviceToken es el token de Firebase Cloud Messaging de UN teléfono (app
// móvil) de UN usuario. Como el endpoint del Web Push, el token identifica al
// aparato: si otra persona inicia sesión en el mismo teléfono, la fila pasa a
// ser suya.
type DeviceToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Token     string    `gorm:"size:512;uniqueIndex" json:"-"`
	Platform  string    `gorm:"size:16" json:"platform"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DeviceToken) TableName() string {
	return "device_tokens"
}
