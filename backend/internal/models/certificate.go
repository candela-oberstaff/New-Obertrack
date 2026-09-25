package models

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Campos que una plantilla de certificado puede colocar sobre el diseño.
const (
	CertificateFieldName    = "name"    // Nombre del profesional
	CertificateFieldProgram = "program" // Nombre del programa
	CertificateFieldDate    = "date"    // Fecha de emisión
	CertificateFieldCode    = "code"    // Código de verificación
	CertificateFieldText    = "text"    // Texto libre ("por haber completado...")
)

func IsValidCertificateFieldKey(key string) bool {
	switch key {
	case CertificateFieldName, CertificateFieldProgram, CertificateFieldDate, CertificateFieldCode, CertificateFieldText:
		return true
	}
	return false
}

// CertificateField es un texto colocado sobre el diseño. X e Y van en
// porcentaje del ancho y alto de la página, que es lo que hace que la
// posición sea la misma en el editor (imagen a escala) y en el PDF.
type CertificateField struct {
	Key   string `json:"key"`
	Text  string `json:"text,omitempty"` // Solo para el campo libre
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Size  float64 `json:"size"`  // Puntos
	Color string  `json:"color"` // #rrggbb
	Align string  `json:"align"` // L | C | R
	Bold  bool    `json:"bold"`
	Font  string  `json:"font"` // Helvetica | Times | Courier
}

// CertificateTemplate es un diseño subido por el equipo (imagen A4) más la
// posición de los campos que se imprimen encima. Se asigna por programa.
type CertificateTemplate struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:160;not null" json:"name"`
	// ImageFilename es el archivo del diseño en la carpeta de uploads.
	ImageFilename string `gorm:"size:255;not null" json:"image_filename"`
	// Orientation es la de la página: L (horizontal) o P (vertical). Se
	// deduce del tamaño de la imagen al subirla.
	Orientation string `gorm:"size:1;not null;default:'L'" json:"orientation"`
	// FieldsJSON es la lista de campos serializada; Fields es la misma lista
	// desempaquetada, que es como viaja al panel.
	FieldsJSON string             `gorm:"type:text" json:"-"`
	Fields     []CertificateField `gorm:"-" json:"fields"`
	CreatedBy  uint               `gorm:"not null;index" json:"created_by"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
	DeletedAt  gorm.DeletedAt     `gorm:"index" json:"-"`

	// ProgramNames son los programas que la usan (solo lectura, para el panel).
	ProgramNames []string `gorm:"-" json:"program_names"`
}

func (CertificateTemplate) TableName() string {
	return "certificate_templates"
}

func (t *CertificateTemplate) AfterFind(tx *gorm.DB) error {
	t.Fields = []CertificateField{}
	if strings.TrimSpace(t.FieldsJSON) == "" {
		return nil
	}
	_ = json.Unmarshal([]byte(t.FieldsJSON), &t.Fields)
	return nil
}

// ImageURL es la ruta pública del diseño, para el editor.
func (t *CertificateTemplate) ImageURL() string {
	return "/api/public/uploads/" + t.ImageFilename
}

// Certificate es un certificado emitido: un PDF inmutable con su código de
// verificación. Se copia el nombre del programa y de la plantilla porque el
// documento tiene que seguir diciendo lo que decía aunque después se
// renombren o se borren.
type Certificate struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	InviteID     uint       `gorm:"not null;uniqueIndex" json:"invite_id"`
	ProgramID    uint       `gorm:"not null;index" json:"program_id"`
	ProgramName  string     `gorm:"size:160;not null" json:"program_name"`
	TemplateID   uint       `gorm:"not null" json:"template_id"`
	TemplateName string     `gorm:"size:160" json:"template_name"`
	Code         string     `gorm:"size:32;not null;uniqueIndex" json:"code"`
	Filename     string     `gorm:"size:255;not null" json:"-"`
	IssuedAt     time.Time  `gorm:"not null;index" json:"issued_at"`
	ReissuedAt   *time.Time `json:"reissued_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (Certificate) TableName() string {
	return "certificates"
}

// IssuedCertificate es una fila del listado por programa: el certificado con
// el nombre de la persona, para el panel.
type IssuedCertificate struct {
	ID          uint       `json:"id"`
	UserID      uint       `json:"user_id"`
	UserName    string     `json:"user_name"`
	Code        string     `json:"code"`
	ProgramName string     `json:"program_name"`
	IssuedAt    time.Time  `json:"issued_at"`
	ReissuedAt  *time.Time `json:"reissued_at,omitempty"`
}

// ProgramCertificates es lo que ve el editor del programa: cuántos se han
// emitido y los más recientes.
type ProgramCertificates struct {
	Total  int64               `json:"total"`
	Recent []IssuedCertificate `json:"recent"`
}

// DownloadURL es la descarga pública por código: el código impreso en el
// certificado es lo que permite verificarlo y bajarlo desde fuera.
func (c *Certificate) DownloadURL() string {
	return "/api/public/certificates/" + c.Code + "/pdf"
}
