package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// CompanyThreadRepository guarda el hilo del expediente de una empresa: los
// comentarios de cada entrada y los archivos que cuelgan de ellas.
//
// Va en su propio repositorio y no en AdminRepository porque este ya pasa de
// mil líneas y mezcla métricas, expediente, archivados y tickets; añadirle una
// sexta responsabilidad no ayudaría a nadie a encontrar nada.
type CompanyThreadRepository interface {
	// EventBelongsTo comprueba que el evento existe y es de esa empresa. Es la
	// puerta de todo lo demás: sin ella, mandar el id de un evento ajeno dejaría
	// leer o escribir en el expediente de otra empresa.
	EventBelongsTo(eventID, companyID uint) (bool, error)

	CreateComment(c *models.CompanyEventComment) error
	GetComment(id uint) (*models.CompanyEventComment, error)
	UpdateComment(id, companyID uint, content string, editedAt time.Time) (int64, error)
	DeleteComment(id, companyID uint) (int64, error)
	// ListComments trae los comentarios de varios eventos de una vez, para no
	// hacer una consulta por entrada al pintar una página del expediente.
	ListComments(companyID uint, eventIDs []uint) ([]models.CompanyEventComment, error)

	CreateAttachment(a *models.CompanyEventAttachment) error
	GetAttachment(id, companyID uint) (*models.CompanyEventAttachment, error)
	DeleteAttachment(id, companyID uint) (int64, error)
	ListAttachments(companyID uint, eventIDs []uint) ([]models.CompanyEventAttachment, error)
	CountAttachmentsForEvent(eventID uint) (int64, error)

	// CountThread dice cuántos comentarios y archivos se llevaría por delante
	// borrar una entrada, para poder avisarlo antes en vez de después.
	CountThread(eventID, companyID uint) (comments int64, attachments int64, err error)
	// DeleteThreadForEvent limpia el hilo al borrar la entrada que lo sostenía.
	DeleteThreadForEvent(eventID, companyID uint) error
}

type companyThreadRepository struct {
	db *gorm.DB
}

func NewCompanyThreadRepository(db *gorm.DB) CompanyThreadRepository {
	return &companyThreadRepository{db: db}
}

func (r *companyThreadRepository) EventBelongsTo(eventID, companyID uint) (bool, error) {
	var n int64
	err := r.db.Model(&models.CompanyEvent{}).
		Where("id = ? AND company_id = ?", eventID, companyID).
		Count(&n).Error
	return n > 0, err
}

func (r *companyThreadRepository) CreateComment(c *models.CompanyEventComment) error {
	return r.db.Create(c).Error
}

func (r *companyThreadRepository) GetComment(id uint) (*models.CompanyEventComment, error) {
	var c models.CompanyEventComment
	if err := r.db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *companyThreadRepository) UpdateComment(id, companyID uint, content string, editedAt time.Time) (int64, error) {
	res := r.db.Model(&models.CompanyEventComment{}).
		Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]interface{}{"content": content, "edited_at": editedAt})
	return res.RowsAffected, res.Error
}

func (r *companyThreadRepository) DeleteComment(id, companyID uint) (int64, error) {
	res := r.db.Where("id = ? AND company_id = ?", id, companyID).
		Delete(&models.CompanyEventComment{})
	return res.RowsAffected, res.Error
}

func (r *companyThreadRepository) ListComments(companyID uint, eventIDs []uint) ([]models.CompanyEventComment, error) {
	if len(eventIDs) == 0 {
		return []models.CompanyEventComment{}, nil
	}
	// El autor se resuelve aquí con un JOIN en vez de con Preload: solo hace
	// falta el nombre, y traer el usuario entero cargaría contraseñas y tokens
	// para pintar una línea de texto.
	rows := []struct {
		models.CompanyEventComment
		AuthorName string
	}{}
	err := r.db.Model(&models.CompanyEventComment{}).
		Select("company_event_comments.*, COALESCE(u.name, '') AS author_name").
		Joins("LEFT JOIN users u ON u.id = company_event_comments.by_user_id").
		Where("company_event_comments.company_id = ? AND company_event_comments.event_id IN ?", companyID, eventIDs).
		Where("company_event_comments.deleted_at IS NULL").
		Order("company_event_comments.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.CompanyEventComment, 0, len(rows))
	for _, row := range rows {
		c := row.CompanyEventComment
		c.Author = row.AuthorName
		out = append(out, c)
	}
	return out, nil
}

func (r *companyThreadRepository) CreateAttachment(a *models.CompanyEventAttachment) error {
	return r.db.Create(a).Error
}

func (r *companyThreadRepository) GetAttachment(id, companyID uint) (*models.CompanyEventAttachment, error) {
	var a models.CompanyEventAttachment
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *companyThreadRepository) DeleteAttachment(id, companyID uint) (int64, error) {
	res := r.db.Where("id = ? AND company_id = ?", id, companyID).
		Delete(&models.CompanyEventAttachment{})
	return res.RowsAffected, res.Error
}

func (r *companyThreadRepository) ListAttachments(companyID uint, eventIDs []uint) ([]models.CompanyEventAttachment, error) {
	if len(eventIDs) == 0 {
		return []models.CompanyEventAttachment{}, nil
	}
	rows := []struct {
		models.CompanyEventAttachment
		AuthorName string
	}{}
	err := r.db.Model(&models.CompanyEventAttachment{}).
		Select("company_event_attachments.*, COALESCE(u.name, '') AS author_name").
		Joins("LEFT JOIN users u ON u.id = company_event_attachments.by_user_id").
		Where("company_event_attachments.company_id = ? AND company_event_attachments.event_id IN ?", companyID, eventIDs).
		Where("company_event_attachments.deleted_at IS NULL").
		Order("company_event_attachments.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]models.CompanyEventAttachment, 0, len(rows))
	for _, row := range rows {
		a := row.CompanyEventAttachment
		a.Author = row.AuthorName
		out = append(out, a)
	}
	return out, nil
}

func (r *companyThreadRepository) CountAttachmentsForEvent(eventID uint) (int64, error) {
	var n int64
	err := r.db.Model(&models.CompanyEventAttachment{}).
		Where("event_id = ?", eventID).
		Count(&n).Error
	return n, err
}

func (r *companyThreadRepository) CountThread(eventID, companyID uint) (int64, int64, error) {
	var comments, attachments int64
	if err := r.db.Model(&models.CompanyEventComment{}).
		Where("event_id = ? AND company_id = ?", eventID, companyID).
		Count(&comments).Error; err != nil {
		return 0, 0, err
	}
	if err := r.db.Model(&models.CompanyEventAttachment{}).
		Where("event_id = ? AND company_id = ?", eventID, companyID).
		Count(&attachments).Error; err != nil {
		return 0, 0, err
	}
	return comments, attachments, nil
}

// DeleteThreadForEvent borra comentarios y adjuntos en una transacción: dejar
// medio hilo huérfano porque falló la segunda consulta sería peor que no borrar.
//
// Los archivos NO se quitan del disco. Es deliberado y consistente con el resto
// de la aplicación (los adjuntos de tarea tampoco), y prefiero eso a borrar por
// error un fichero que otra fila siga usando.
func (r *companyThreadRepository) DeleteThreadForEvent(eventID, companyID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("event_id = ? AND company_id = ?", eventID, companyID).
			Delete(&models.CompanyEventAttachment{}).Error; err != nil {
			return err
		}
		return tx.Where("event_id = ? AND company_id = ?", eventID, companyID).
			Delete(&models.CompanyEventComment{}).Error
	})
}
