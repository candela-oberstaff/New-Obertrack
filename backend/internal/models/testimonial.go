package models

import (
	"time"

	"gorm.io/gorm"
)

// Audiencia de un testimonio: quién lo escribe. Cada una tiene su plantilla de
// preguntas y su texto de consentimiento (ver service.TestimonialTemplates).
const (
	// TestimonialFromProfessional: lo escribe un profesional sobre su
	// experiencia trabajando a través de Oberstaff.
	TestimonialFromProfessional = "professional"
	// TestimonialFromCompany: lo escribe la cuenta empresa sobre el servicio.
	TestimonialFromCompany = "company"
)

// Ciclo de vida de un testimonio. Es lineal salvo por el reenvío, que devuelve
// un 'pending' a 'pending' con un token nuevo.
const (
	TestimonialPending   = "pending"   // Enviado, esperando respuesta
	TestimonialSubmitted = "submitted" // Respondido y firmado, esperando revisión
	TestimonialApproved  = "approved"  // Aprobado: se puede usar
	TestimonialRejected  = "rejected"  // Descartado por el equipo
	// TestimonialChangesRequested: se devolvió con un motivo para que su autor
	// corrija (una firma ilegible, un nombre a medias, una errata). El enlace
	// vuelve a abrirse con lo que ya escribió precargado.
	//
	// Es un estado aparte y no un "rechazado que se puede reabrir" porque son
	// dos desenlaces distintos: descartar cierra el asunto, devolver lo deja
	// abierto esperando a la persona. Mezclarlos haría que la bandeja no
	// distinguiera lo que ya no va a pasar de lo que está en curso.
	TestimonialChangesRequested = "changes_requested"
)

// Cómo se produjo la firma. Forma parte de la evidencia y NO es un detalle
// cosmético: las tres valen como firma electrónica simple, pero no prueban lo
// mismo. Un trazo es un acto manual irrepetible; una imagen cargada pudo salir
// de cualquier sitio; un nombre tipografiado es solo texto con una fuente
// bonita, y lo que lo sostiene es el resto de la evidencia (el enlace enviado a
// su correo, la IP, la marca de tiempo).
//
// Por eso la constancia dice con cuál se firmó: quien la lea tiene que poder
// juzgarlo, no suponerlo.
const (
	SignatureDrawn    = "drawn"    // Trazada a mano en el recuadro
	SignatureUploaded = "uploaded" // Imagen de una firma real, cargada
	SignatureTyped    = "typed"    // Nombre escrito con una tipografía
)

// Testimonial es la solicitud de testimonio Y el testimonio en sí: una sola
// fila que recorre todo el ciclo (se pide → se responde y firma → se aprueba).
//
// Se modeló en una sola tabla y no en dos porque la relación es 1 a 1 y el
// panel siempre necesita ver ambas mitades juntas: qué se pidió, a quién, y qué
// contestó. Cada solicitud vive independiente de las demás, así que a la misma
// persona se le puede volver a pedir más adelante sin pisar la anterior.
//
// Los campos marcados como CONGELADOS se copian al emitir la solicitud y nunca
// se releen de su origen. Es deliberado: un testimonio se publica con el cargo
// y la empresa que la persona tenía CUANDO lo escribió, y el consentimiento
// vale por el texto exacto que se le mostró, no por el que esté vigente hoy.
type Testimonial struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Token es la única credencial de la página pública: quien responde puede
	// no tener sesión (una cuenta empresa que delega en su gerente, un
	// profesional que ya terminó su empleo). Por eso es aleatorio, único y con
	// vencimiento. Nunca se serializa.
	Token string `gorm:"size:64;not null;uniqueIndex" json:"-"`

	Audience string `gorm:"size:20;not null;index" json:"audience"`
	Status   string `gorm:"size:20;not null;default:'pending';index" json:"status"`

	// UserID es a quién se le pidió. Apunta siempre a users: un profesional, o
	// la cuenta empleador cuando la audiencia es 'company'.
	UserID uint `gorm:"not null;index" json:"user_id"`
	// RequestedBy es quién lo pidió (superadmin o customer success).
	RequestedBy uint `gorm:"not null;index" json:"requested_by"`

	// --- Identidad CONGELADA de quien firma ---
	RecipientName    string `gorm:"size:255;not null" json:"recipient_name"`
	RecipientEmail   string `gorm:"size:255;not null" json:"recipient_email"`
	RecipientRole    string `gorm:"size:255" json:"recipient_role"`    // Cargo al momento de pedirlo
	RecipientCompany string `gorm:"size:255" json:"recipient_company"` // Empresa al momento de pedirlo

	// --- Contenido CONGELADO de la solicitud ---
	// Prompts son las preguntas guía, como JSON de strings. Orientan la
	// redacción; no son obligatorias.
	Prompts string `gorm:"type:text" json:"prompts"`
	// IntroMessage es la nota personal que el equipo escribe al pedirlo.
	IntroMessage string `gorm:"type:text" json:"intro_message"`
	// ConsentText es el texto EXACTO de autorización que se le mostró, y
	// ConsentVersion la versión de esa redacción. Congelarlos es lo que hace
	// defendible el permiso: si mañana cambia el texto, este testimonio sigue
	// respaldado por el que su autor realmente leyó.
	ConsentText    string `gorm:"type:text" json:"consent_text"`
	ConsentVersion string `gorm:"size:20" json:"consent_version"`

	ExpiresAt  time.Time  `json:"expires_at"`
	RemindedAt *time.Time `json:"reminded_at,omitempty"`

	// --- Respuesta ---
	Rating int    `gorm:"not null;default:0" json:"rating"` // 1 a 5; 0 = sin calificar
	Quote  string `gorm:"type:text" json:"quote"`           // El testimonio
	// Answers son las respuestas a las preguntas guía, como JSON de
	// [{prompt, answer}]. Sirven de material para editar la cita.
	Answers     string     `gorm:"type:text" json:"answers"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`

	// --- Alcance del permiso otorgado ---
	// Qué se autoriza a mostrar junto a la cita. El nombre siempre se puede
	// omitir a pedido, pero estos campos dejan por escrito qué dijo que sí.
	AllowPublicName bool `gorm:"not null;default:false" json:"allow_public_name"`
	AllowRole       bool `gorm:"not null;default:false" json:"allow_role"`
	AllowPhoto      bool `gorm:"not null;default:false" json:"allow_photo"`
	AllowLogo       bool `gorm:"not null;default:false" json:"allow_logo"`

	// --- Firma y evidencia ---
	// SignatureName es el nombre completo tipeado al firmar, y SignatureImage
	// el archivo PNG del trazo. Juntos con la marca de tiempo, la IP y el
	// navegador forman la evidencia de una firma electrónica simple: quién
	// firmó, qué texto aceptó, cuándo y desde dónde.
	SignatureName string `gorm:"size:255" json:"signature_name"`
	// SignatureMode es cómo se firmó (ver constantes de arriba). Vacío en los
	// testimonios anteriores a que existieran las tres modalidades: entonces
	// solo se podía trazar.
	SignatureMode   string     `gorm:"size:20" json:"signature_mode"`
	SignatureImage  string     `gorm:"size:500" json:"-"`
	SignedAt        *time.Time `json:"signed_at,omitempty"`
	SignerIP        string     `gorm:"size:64" json:"signer_ip"`
	SignerUserAgent string     `gorm:"size:500" json:"signer_user_agent"`

	// --- Revisión interna ---
	ReviewedBy *uint      `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote string     `gorm:"type:text" json:"review_note"`
	// PublishedQuote es la cita ya editada para publicar (recortada, con la
	// puntuación arreglada). Vacía = se usa Quote tal como se recibió. El
	// original NUNCA se sobrescribe: es parte de la evidencia.
	PublishedQuote string `gorm:"type:text" json:"published_quote"`

	// FiledAt marca que el testimonio ya quedó archivado en el expediente de su
	// autor (el de la empresa, o el del empleo si es un profesional).
	//
	// Existe para que archivar sea idempotente: aprobar es una acción que se
	// repite —se corrige la cita y se vuelve a aprobar, se descarta y se
	// recupera— y sin esta marca cada pasada dejaría una entrada más en el
	// expediente, convirtiendo un historial en un eco.
	FiledAt *time.Time `json:"filed_at,omitempty"`

	// --- Corrección ---
	// ChangeReason es lo que hay que arreglar, escrito para que lo lea su autor
	// (no es la nota interna: esto se le muestra y se le envía por correo).
	ChangeReason      string     `gorm:"type:text" json:"change_reason"`
	ChangeRequestedAt *time.Time `json:"change_requested_at,omitempty"`
	// Revisions cuenta cuántas veces se devolvió. Sirve para no insistir
	// eternamente sobre la misma persona.
	Revisions int `gorm:"not null;default:0" json:"revisions"`
	// SignatureTrail guarda las firmas ANTERIORES como JSON.
	//
	// Cuando alguien corrige su testimonio vuelve a firmar, y la firma vieja
	// deja de valer: autorizaba un texto que ya no existe. Pero tampoco puede
	// desaparecer sin más —hubo un acto de firma y quedó registrado—, así que se
	// aparta aquí con su evidencia. Lo vigente es siempre lo que está en los
	// campos de arriba.
	SignatureTrail string `gorm:"type:text" json:"-"`

	// User es quien firma. Se precarga solo en las vistas internas.
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Testimonial) TableName() string {
	return "testimonials"
}

// Open indica si el enlace admite respuesta: o nunca se ha contestado, o se
// devolvió para corregir.
func (t *Testimonial) Open() bool {
	return t != nil && (t.Status == TestimonialPending || t.Status == TestimonialChangesRequested)
}

// Expired indica si el enlace ya venció. Un testimonio vencido no se puede
// responder, pero se reenvía con un token nuevo desde el panel.
func (t *Testimonial) Expired() bool {
	return t != nil && t.Open() && time.Now().After(t.ExpiresAt)
}

// SignatureModeLabel describe la modalidad en palabras, para la constancia y el
// panel. Un testimonio viejo sin modalidad se describe como trazado, que es lo
// único que se podía hacer entonces.
func (t *Testimonial) SignatureModeLabel() string {
	if t == nil {
		return ""
	}
	switch t.SignatureMode {
	case SignatureUploaded:
		return "Imagen de firma cargada por el firmante"
	case SignatureTyped:
		return "Nombre escrito con una tipografía"
	default:
		return "Trazada a mano por el firmante"
	}
}

// Signed indica si ya hay una firma registrada.
func (t *Testimonial) Signed() bool {
	return t != nil && t.SignedAt != nil && t.SignatureName != ""
}

// DisplayQuote es el texto que se debe usar al publicar: la versión editada si
// existe, y si no la original.
func (t *Testimonial) DisplayQuote() string {
	if t == nil {
		return ""
	}
	if t.PublishedQuote != "" {
		return t.PublishedQuote
	}
	return t.Quote
}
