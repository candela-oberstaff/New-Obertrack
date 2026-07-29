package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	// MeetingStatusScheduled: la sesión está viva y su evento existe en Google.
	MeetingStatusScheduled = "scheduled"
	// MeetingStatusCancelled: se canceló. La fila se conserva como bitácora (quién
	// convocó a quién y cuándo) en vez de borrarse.
	MeetingStatusCancelled = "cancelled"
)

// MeetingSession es una reunión con sala de Google Meet convocada desde
// Obertrack. El evento vive en el calendario del ORGANIZADOR: se crea con su
// vínculo personal de Google (models.GoogleCalendarAccount), así que quien no
// tenga cuenta conectada no puede convocar.
//
// A diferencia de las tareas —que son de día completo y esquivan la zona
// horaria— una sesión tiene hora, así que StartAt/EndAt se guardan en UTC y
// TimeZone conserva la zona en la que se convocó.
type MeetingSession struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	TenantID    uint   `gorm:"index;not null" json:"tenant_id"`
	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`

	StartAt time.Time `gorm:"index;not null" json:"start_at"`
	EndAt   time.Time `gorm:"not null" json:"end_at"`
	// TimeZone es el identificador IANA ("America/Bogota") con el que se convocó.
	// Se guarda —y no solo el instante UTC— porque una serie recurrente debe
	// repetirse a la misma hora LOCAL: con un offset fijo, el cambio de horario
	// de verano desplazaría toda la serie.
	TimeZone string `gorm:"size:64;not null;default:'UTC'" json:"time_zone"`

	OrganizerID uint `gorm:"index;not null" json:"organizer_id"`
	Organizer   User `gorm:"foreignKey:OrganizerID" json:"organizer,omitempty"`

	// GoogleEventID y CalendarID identifican el evento en el calendario del
	// organizador; sin ellos no se podría editar ni cancelar la reunión.
	GoogleEventID string `gorm:"size:255;index" json:"google_event_id,omitempty"`
	CalendarID    string `gorm:"size:255;not null;default:'primary'" json:"calendar_id"`
	// MeetURL es el enlace de la videollamada. Puede quedar vacío si Google
	// todavía estaba creando la sala; se resuelve al consultar la sesión.
	MeetURL  string `gorm:"size:512" json:"meet_url"`
	HTMLLink string `gorm:"size:512" json:"html_link,omitempty"`

	Status string `gorm:"size:20;not null;default:'scheduled';index" json:"status"`

	// TaskID enlaza la sesión con una tarea (opcional): reuniones de seguimiento
	// de un trabajo concreto.
	TaskID *uint `gorm:"index" json:"task_id,omitempty"`

	// RecurrenceRule es la RRULE de la serie (vacío = sesión única). Una serie es
	// UN evento recurrente en Google con UN único enlace de Meet.
	RecurrenceRule string `gorm:"size:255" json:"recurrence_rule,omitempty"`
	// SeriesEndsAt es el fin de la ÚLTIMA ocurrencia; NULL = la serie no termina.
	// Existe para poder preguntar en SQL qué series siguen vivas: sin esta
	// columna habría que filtrar por end_at, que solo describe la PRIMERA
	// ocurrencia, y una serie diaria desaparecería de "próximas" en cuanto
	// pasara su primer día.
	SeriesEndsAt *time.Time `gorm:"index" json:"series_ends_at,omitempty"`

	// NextStartAt y NextEndAt son la próxima ocurrencia, calculada al vuelo
	// desde la RRULE. No se persisten (`gorm:"-"`) porque cambian con el reloj:
	// guardarlas obligaría a un job que las fuera corrigiendo.
	NextStartAt *time.Time `gorm:"-" json:"next_start_at,omitempty"`
	NextEndAt   *time.Time `gorm:"-" json:"next_end_at,omitempty"`

	Attendees []MeetingAttendee `gorm:"foreignKey:SessionID" json:"attendees,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MeetingSession) TableName() string { return "meeting_sessions" }

// MeetingAttendee es un invitado. UserID nulo = invitado EXTERNO (un cliente, un
// candidato) del que solo se conoce el correo. El correo se guarda siempre,
// interno o no, porque es lo que viaja a Google; el UserID es lo que permite
// avisar además por campanita y DM dentro de Obertrack.
type MeetingAttendee struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SessionID uint   `gorm:"not null;uniqueIndex:idx_session_email" json:"session_id"`
	UserID    *uint  `gorm:"index" json:"user_id,omitempty"`
	Email     string `gorm:"size:255;not null;uniqueIndex:idx_session_email" json:"email"`
	// Name es una copia para poder mostrar la lista sin resolver usuarios (y para
	// que un invitado externo tenga algo legible).
	Name      string    `gorm:"size:255" json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (MeetingAttendee) TableName() string { return "meeting_attendees" }

// IsExternal indica si el invitado no tiene cuenta en Obertrack.
func (a MeetingAttendee) IsExternal() bool { return a.UserID == nil }
