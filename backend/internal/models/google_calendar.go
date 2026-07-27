package models

import "time"

const (
	// GoogleCalStatusActive: el vínculo funciona y se puede refrescar el token.
	GoogleCalStatusActive = "active"
	// GoogleCalStatusNeedsReauth: Google rechazó el refresh token (el usuario
	// revocó el acceso desde su cuenta, cambió la contraseña, o el token de un
	// proyecto en modo "testing" caducó a los 7 días). El vínculo se conserva
	// para poder mostrar "reconectar" en vez de perder el estado en silencio.
	GoogleCalStatusNeedsReauth = "needs_reauth"
)

// GoogleCalendarAccount es el vínculo PERSONAL de un usuario con una cuenta de
// Google. La pantalla de consentimiento es de tipo "External", así que sirve
// igual un @gmail.com que un dominio propio (@oberstaff.com); no hay nada
// configurado por dominio.
//
// El vínculo es por USUARIO y no por empresa a propósito: el calendario
// pertenece a la persona, así que sobrevive a SwitchCompany y a las altas y
// bajas de empleo. De ahí el uniqueIndex sobre user_id.
type GoogleCalendarAccount struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;uniqueIndex" json:"user_id"`
	// GoogleSub es el 'sub' del id_token: el identificador estable de la cuenta
	// de Google. Se guarda porque el email SÍ puede cambiar, y sin esto una
	// reconexión tras un cambio de email crearía un vínculo duplicado.
	GoogleSub   string `gorm:"size:64;index" json:"-"`
	GoogleEmail string `gorm:"size:255;not null" json:"google_email"`

	// Tokens cifrados con AES-256-GCM (utils.SecretSealer). El `json:"-"` es
	// defensa en profundidad: aunque un handler serializara la fila entera, el
	// secreto nunca sale al frontend.
	RefreshTokenEnc string     `gorm:"type:text;not null" json:"-"`
	AccessTokenEnc  string     `gorm:"type:text" json:"-"`
	AccessExpiresAt *time.Time `json:"-"`

	Scopes string `gorm:"type:text" json:"scopes"`
	// CalendarID es el calendario destino dentro de la cuenta. 'primary' es el
	// principal del usuario; puede elegir otro (p.ej. uno dedicado a trabajo).
	CalendarID   string `gorm:"size:255;not null;default:'primary'" json:"calendar_id"`
	CalendarName string `gorm:"size:255" json:"calendar_name"`

	Status    string `gorm:"size:20;not null;default:'active';index" json:"status"`
	LastError string `gorm:"type:text" json:"last_error,omitempty"`

	ConnectedAt time.Time `json:"connected_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (GoogleCalendarAccount) TableName() string {
	return "google_calendar_accounts"
}
