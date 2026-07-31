package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// MaxCompanyCommentLength acota el comentario. Es más largo que la nota (2000)
// porque aquí es donde se pega el detalle —un error, una respuesta del cliente,
// los pasos para reproducir— mientras que la nota es el titular del hecho.
const MaxCompanyCommentLength = 4000

// MaxAttachmentsPerEntry evita que una entrada del expediente se convierta en
// un repositorio de archivos. No es un límite técnico: es que una nota con
// cuarenta adjuntos ya no se lee, y lo que se buscaba era una cronología legible.
const MaxAttachmentsPerEntry = 20

// CompanyEventComment es una respuesta a una entrada del expediente de una
// empresa. Cuelga del evento —una nota, un contacto, una suspensión— para que
// la conversación viva pegada al hecho que la provocó, y no en un tablón aparte
// donde nadie sabría de qué se estaba hablando.
//
// Solo pueden tener hilo las entradas que son filas de company_events. El resto
// del expediente (jornadas, altas y bajas, gestiones de CS) se deriva de otras
// tablas al vuelo y no existe como registro al que colgar nada.
type CompanyEventComment struct {
	ID      uint `gorm:"primaryKey" json:"id"`
	EventID uint `gorm:"not null;index" json:"event_id"`
	// CompanyID se guarda aunque sea deducible del evento: permite acotar las
	// consultas y las comprobaciones de permiso a una empresa sin unir tablas,
	// y es la defensa contra pedir el comentario de otra empresa por su id.
	CompanyID uint      `gorm:"not null;index" json:"company_id"`
	ByUserID  uint      `gorm:"not null" json:"by_user_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"-"`
	// EditedAt va aparte de updated_at por lo mismo que en las notas: corregir
	// el texto sí es editar y en una auditoría tiene que verse, pero tocar la
	// fila por cualquier otro motivo no debería parecerlo.
	EditedAt  *time.Time     `json:"edited_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Campos de presentación: se rellenan al leer, no son columnas.
	Author      string                   `gorm:"-" json:"author"`
	Attachments []CompanyEventAttachment `gorm:"-" json:"attachments,omitempty"`
}

func (CompanyEventComment) TableName() string {
	return "company_event_comments"
}

// CompanyEventAttachment es un archivo colgado de una entrada del expediente o
// de uno de sus comentarios.
//
// No guarda una URL pública: guarda el nombre con el que el archivo quedó en
// disco y se sirve por una ruta que comprueba permisos. La ruta genérica
// /api/uploads/:filename deja pasar a cualquier usuario autenticado que acierte
// el nombre, y aquí hay capturas de incidencias y documentos internos que no
// puede ver ni la propia empresa.
type CompanyEventAttachment struct {
	ID      uint `gorm:"primaryKey" json:"id"`
	EventID uint `gorm:"not null;index" json:"event_id"`
	// CompanyID acota igual que en el comentario.
	CompanyID uint `gorm:"not null;index" json:"company_id"`
	// CommentID cuelga el archivo de un comentario concreto. Nulo cuando va
	// directamente en la entrada del expediente.
	CommentID *uint `gorm:"index" json:"comment_id,omitempty"`
	ByUserID  uint  `gorm:"not null" json:"by_user_id"`
	// FileName es el nombre original, el que ve y descarga la persona.
	FileName string `gorm:"size:255;not null" json:"file_name"`
	// StoredName es el nombre en disco. Nunca se expone ni se acepta de la
	// petición: servir por el nombre que mande el cliente es cómo se leen
	// archivos ajenos.
	StoredName string         `gorm:"size:255;not null" json:"-"`
	FileSize   int64          `json:"file_size"`
	MimeType   string         `gorm:"size:128" json:"mime_type"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Author string `gorm:"-" json:"author"`
}

func (CompanyEventAttachment) TableName() string {
	return "company_event_attachments"
}

// IsImage decide si el adjunto se puede enseñar como miniatura en vez de como
// una fila de archivo.
func (a CompanyEventAttachment) IsImage() bool {
	return strings.HasPrefix(strings.ToLower(a.MimeType), "image/")
}
