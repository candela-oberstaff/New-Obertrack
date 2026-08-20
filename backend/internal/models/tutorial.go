package models

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Tipo de contenido de una novedad. Nació siendo siempre un video de Drive o
// YouTube; hoy también puede ser una imagen (un anuncio diseñado, un flyer) o
// un texto con formato. El tipo decide qué campo lleva el contenido y cómo se
// pinta la tarjeta, el reproductor y el aviso a pantalla completa.
const (
	TutorialContentVideo = "video"  // GoogleDriveURL: Drive o YouTube
	TutorialContentImage = "imagen" // ImageURL: archivo subido a /uploads
	TutorialContentText  = "texto"  // Body: HTML con formato e imágenes
)

func IsValidTutorialContentType(contentType string) bool {
	return contentType == TutorialContentVideo ||
		contentType == TutorialContentImage ||
		contentType == TutorialContentText
}

// Audiencia de un tutorial: controla qué tipo de usuario puede verlo.
const (
	TutorialAudienceAll          = "all"         // Visible para empresas y profesionales
	TutorialAudienceEmployer     = "empleador"   // Solo usuarios tipo empresa
	TutorialAudienceProfessional = "profesional" // Solo usuarios tipo profesional
	// TutorialAudienceManager son los profesionales con equipo a cargo
	// (managers y supervisores). Es un rol dentro de los profesionales, no un
	// tipo de cuenta aparte: por eso quien es manager ve TAMBIÉN las novedades
	// dirigidas a profesionales.
	TutorialAudienceManager = "manager"
)

func IsValidTutorialAudience(audience string) bool {
	return audience == TutorialAudienceAll ||
		audience == TutorialAudienceEmployer ||
		audience == TutorialAudienceProfessional ||
		audience == TutorialAudienceManager
}

// AudiencesForUser son las audiencias que alcanzan a una persona. Se usa para
// filtrar tanto el listado como el aviso emergente: un manager entra por su
// audiencia de profesional y además por la de manager.
func AudiencesForUser(userType string, hasTeam bool) []string {
	switch userType {
	case string(UserTypeEmployer):
		return []string{TutorialAudienceAll, TutorialAudienceEmployer}
	case string(UserTypeProfessional):
		audiences := []string{TutorialAudienceAll, TutorialAudienceProfessional}
		if hasTeam {
			audiences = append(audiences, TutorialAudienceManager)
		}
		return audiences
	}
	// Sin filtro: superadmin y personal de plataforma ven todas.
	return nil
}

type Tutorial struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`
	// ContentType elige cuál de los tres campos de contenido manda. El default
	// es 'video' porque es lo único que existía: las novedades anteriores
	// siguen leyéndose igual sin tocar una sola fila.
	ContentType string `gorm:"size:20;not null;default:'video';index" json:"content_type"`
	// GoogleDriveURL es el video (Drive o YouTube). Conserva el nombre por
	// compatibilidad: la inducción y el front lo leen así desde el principio.
	GoogleDriveURL string `gorm:"size:1000;not null" json:"google_drive_url"`
	// ImageURL es la imagen de la novedad, subida por /api/uploads.
	ImageURL string `gorm:"size:1000" json:"image_url"`
	// Body es el contenido con formato de las novedades de texto: HTML del
	// editor enriquecido, con sus imágenes ya subidas al servidor.
	Body     string `gorm:"type:text" json:"body"`
	IconName string `gorm:"size:50;not null;default:'PlayCircle'" json:"icon_name"`
	Category string `gorm:"size:80;default:'General';index" json:"category"`
	Audience string `gorm:"size:20;not null;default:'all';index" json:"audience"`
	// TargetSpec es el público objetivo serializado. Va en una sola columna de
	// texto —y no en tres tablas puente— porque nunca se consulta desde SQL:
	// la regla se evalúa en Go (ver TutorialTarget.Matches), que es lo que
	// mantiene el reparto, el aviso emergente y las métricas de acuerdo.
	TargetSpec string `gorm:"type:text" json:"-"`
	// Target es el mismo público ya desempaquetado, que es como viaja al panel.
	Target      TutorialTarget `gorm:"-" json:"target"`
	DurationMin int            `gorm:"default:0" json:"duration_min"`
	OrderIndex  int            `gorm:"default:0;index" json:"order_index"`
	IsActive    bool           `gorm:"default:true;index" json:"is_active"`
	// AnnouncedAt es el momento en que la novedad se anunció al equipo: cuando
	// se repartieron las notificaciones y empezó a emerger al iniciar sesión.
	// NULL significa "nunca anunciada", que es el estado de todo lo publicado
	// antes de esta función: así el aviso emergente no revive el histórico.
	AnnouncedAt *time.Time `gorm:"index" json:"announced_at,omitempty"`
	// AnnounceDays es cuántos días sigue emergiendo el aviso a quien no lo ha
	// visto, contados desde AnnouncedAt. Es por novedad porque no todas pesan
	// igual: un cambio urgente insiste dos días, uno informativo puede esperar
	// dos semanas. 0 apaga el emergente y deja solo la notificación.
	AnnounceDays int `gorm:"not null;default:2" json:"announce_days"`
	// CTALabel y CTAURL son el boton de accion opcional. Sin ellos la novedad
	// solo se lee; con ellos lleva a algun sitio y ese clic se mide, que es la
	// diferencia entre "la vieron" y "hicieron algo".
	CTALabel string `gorm:"size:80" json:"cta_label"`
	CTAURL   string `gorm:"size:500" json:"cta_url"`
	// PublishAt deja la novedad preparada para publicarse sola. Nil = se
	// publica al activarla, como siempre.
	PublishAt *time.Time `gorm:"index" json:"publish_at,omitempty"`
	// ExpiresAt la retira de la seccion sin borrarla.
	ExpiresAt *time.Time `gorm:"index" json:"expires_at,omitempty"`
	// RemindedAt es el ultimo recordatorio a los pendientes. Sostiene el freno
	// contra el doble clic accidental, que mandaria dos avisos a media empresa.
	RemindedAt *time.Time `json:"reminded_at,omitempty"`
	// RequireAck exige confirmar la lectura en vez de bastar con cerrar. Para
	// lo que tiene consecuencias: cambios de pago, politicas, obligaciones.
	RequireAck bool `gorm:"not null;default:false" json:"require_ack"`
	// AnnounceMaxShows es cuántas veces se le puede mostrar el aviso a una
	// misma persona. 0 = sin límite (manda solo el plazo en días). Es el freno
	// para quien nunca cierra el aviso y lo esquiva recargando.
	AnnounceMaxShows int            `gorm:"not null;default:0" json:"announce_max_shows"`
	CreatedBy        uint           `gorm:"not null;index" json:"created_by"`
	Creator          User           `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Tutorial) TableName() string {
	return "tutorials"
}

// AfterFind desempaqueta el público objetivo al leer. Un JSON corrupto no
// puede tumbar la lectura de la novedad: se degrada a "sin acotar", que es el
// comportamiento de toda novedad que no usa esta función.
func (t *Tutorial) AfterFind(tx *gorm.DB) error {
	t.Target = TutorialTarget{}
	if strings.TrimSpace(t.TargetSpec) == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(t.TargetSpec), &t.Target)
	return nil
}

// TutorialTarget acota a QUIÉN va dirigida una novedad, por encima del tipo de
// cuenta (Audience). Los criterios se combinan con Y: "profesionales de Acme
// que además estén en Venezuela". Todos vacíos = toda la audiencia.
type TutorialTarget struct {
	// CompanyIDs son las empresas elegidas. Alcanza a la cuenta de la empresa
	// y a sus profesionales.
	CompanyIDs []uint `json:"company_ids"`
	// Countries filtra por el país de la ficha.
	Countries []string `json:"countries"`
	// GroupIDs son grupos de audiencia (los mismos de Correos).
	GroupIDs []uint `json:"group_ids"`
	// ManagersOnly deja fuera a quien no tiene equipo a cargo.
	ManagersOnly bool `json:"managers_only"`
}

// IsEmpty indica que la novedad va a toda su audiencia, sin acotar.
func (t TutorialTarget) IsEmpty() bool {
	return len(t.CompanyIDs) == 0 && len(t.Countries) == 0 && len(t.GroupIDs) == 0 && !t.ManagersOnly
}

// Matches decide si una persona entra en el público objetivo. inTargetGroup lo
// precalcula quien llama (es una consulta aparte) y solo se mira cuando el
// público acota por grupos.
//
// Esta función es la ÚNICA definición de la regla: la usan el reparto de
// notificaciones, el aviso a pantalla completa y el alcance de las métricas.
// Si alguna vez se reescribe en SQL, los tres se van a desincronizar.
func (t TutorialTarget) Matches(user *User, inTargetGroup bool) bool {
	if user == nil {
		return false
	}
	if len(t.CompanyIDs) > 0 {
		matched := false
		for _, id := range t.CompanyIDs {
			// La empresa es una cuenta más: entra ella y entran los suyos.
			if user.ID == id || (user.EmpleadorID != nil && *user.EmpleadorID == id) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(t.Countries) > 0 {
		matched := false
		for _, country := range t.Countries {
			if strings.EqualFold(strings.TrimSpace(user.Country), strings.TrimSpace(country)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(t.GroupIDs) > 0 && !inTargetGroup {
		return false
	}
	if t.ManagersOnly && !user.IsManager && !user.IsSupervisor {
		return false
	}
	return true
}

// De dónde salió una vista. Sirve para saber si la gente se entera por el
// aviso que le salta al entrar o porque va a buscar la novedad a la sección:
// son dos formas muy distintas de que algo "llegue".
const (
	TutorialViewFromAnnouncement = "anuncio" // Aviso a pantalla completa
	TutorialViewFromSection      = "seccion" // Tarjeta de /novedades
)

type TutorialView struct {
	TutorialID uint `gorm:"primaryKey;autoIncrement:false" json:"tutorial_id"`
	UserID     uint `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	// Source es el primer origen: si alguien cerró el aviso y después abrió la
	// tarjeta, la vista sigue contando como del aviso, que fue lo que funcionó.
	Source string `gorm:"size:20;not null;default:'seccion'" json:"source"`
	// AcknowledgedAt solo se llena cuando la novedad exige confirmar la
	// lectura. Es evidencia: "la vio" y "dijo haberla leido" no son lo mismo.
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ViewedAt       time.Time  `json:"viewed_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TutorialClick registra que alguien pulso el boton de accion de una novedad.
// Una fila por persona: interesa a cuanta gente movio, no cuantas veces pulso.
type TutorialClick struct {
	TutorialID uint      `gorm:"primaryKey;autoIncrement:false" json:"tutorial_id"`
	UserID     uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	ClickedAt  time.Time `json:"clicked_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (TutorialClick) TableName() string {
	return "tutorial_clicks"
}

// TutorialShow cuenta cuántas veces se le ha mostrado el aviso a una persona.
// Es distinto de la vista: la vista se registra al cerrarlo, y esto se registra
// cada vez que aparece, lo haya cerrado o no.
type TutorialShow struct {
	TutorialID  uint      `gorm:"primaryKey;autoIncrement:false" json:"tutorial_id"`
	UserID      uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	ShownCount  int       `gorm:"not null;default:0" json:"shown_count"`
	LastShownAt time.Time `json:"last_shown_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (TutorialShow) TableName() string {
	return "tutorial_shows"
}

func (TutorialView) TableName() string {
	return "tutorial_views"
}

// ── Métricas de una novedad ──────────────────────────────────────────────────
// No son tablas: se calculan al vuelo y viajan tal cual al panel.

// TutorialAudienceStat es el resultado para un tipo de cuenta de la audiencia.
type TutorialAudienceStat struct {
	UserType string `json:"user_type"`
	Reach    int64  `json:"reach"`
	Views    int64  `json:"views"`
}

// TutorialViewer es una persona que ya vio la novedad.
type TutorialViewer struct {
	UserID       uint      `json:"user_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	UserType     string    `json:"user_type"`
	Source       string    `json:"source"`
	ViewedAt     time.Time `json:"viewed_at"`
	Acknowledged bool      `json:"acknowledged"`
	Clicked      bool      `json:"clicked"`
}

// TutorialMetrics es la foto de cómo le fue a una novedad.
type TutorialMetrics struct {
	TutorialID uint   `json:"tutorial_id"`
	Audience   string `json:"audience"`
	// Reach son las cuentas ACTIVAS a las que iba dirigida. Es el denominador:
	// sin él, "12 vistas" no dice si fue un éxito o un fracaso.
	Reach int64 `json:"reach"`
	Views int64 `json:"views"`
	// Pending es a cuántos les falta verla.
	Pending int64 `json:"pending"`
	// ViewRate es el porcentaje visto, redondeado a un decimal.
	ViewRate float64 `json:"view_rate"`
	// Desglose por origen: cuántas vistas ganó el aviso emergente y cuántas la
	// visita a la sección.
	FromAnnouncement int64 `json:"from_announcement"`
	FromSection      int64 `json:"from_section"`

	// Clicks son las personas que pulsaron el boton de accion, y ClickRate su
	// porcentaje SOBRE QUIENES LA VIERON: medirlo contra el alcance castigaria
	// a la novedad por gente que ni siquiera la abrio.
	Clicks    int64   `json:"clicks"`
	ClickRate float64 `json:"click_rate"`
	// Acknowledged son quienes confirmaron la lectura (solo si se exige).
	Acknowledged int64 `json:"acknowledged"`
	RequireAck   bool  `json:"require_ack"`
	// Pendientes de ver, para el recordatorio.
	RemindedAt *time.Time `json:"reminded_at,omitempty"`

	ByAudience    []TutorialAudienceStat `json:"by_audience"`
	RecentViewers []TutorialViewer       `json:"recent_viewers"`

	AnnouncedAt *time.Time `json:"announced_at,omitempty"`
	// AnnounceOpen indica si el aviso emergente todavía está activo.
	AnnounceOpen bool `json:"announce_open"`
}

// TutorialAudiencePreview responde "a cuanta gente le llegaria esto" antes de
// publicar. Sin esa cifra, acotar el publico es dispararle a ciegas.
type TutorialAudiencePreview struct {
	Reach      int64                  `json:"reach"`
	ByAudience []TutorialAudienceStat `json:"by_audience"`
}

// TutorialAudienceOption es una opcion elegible del publico objetivo (una
// empresa o un grupo), con cuanta gente hay detras.
type TutorialAudienceOption struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// TutorialAudienceOptions es lo que puede elegirse al acotar el publico. Se
// calcula de los datos vivos, no de una lista mantenida a mano.
type TutorialAudienceOptions struct {
	Companies []TutorialAudienceOption `json:"companies"`
	Countries []string                 `json:"countries"`
	Groups    []TutorialAudienceOption `json:"groups"`
}
