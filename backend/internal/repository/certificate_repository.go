package repository

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// CertificateRepository guarda las plantillas de certificado y los
// certificados emitidos.
type CertificateRepository interface {
	ListTemplates() ([]models.CertificateTemplate, error)
	GetTemplate(id uint) (*models.CertificateTemplate, error)
	CreateTemplate(t *models.CertificateTemplate) error
	UpdateTemplate(id uint, updates map[string]interface{}) error
	DeleteTemplate(id uint) error
	// ProgramsUsingTemplate devuelve los nombres de los programas que la usan.
	ProgramsUsingTemplate(id uint) ([]string, error)

	CreateCertificate(c *models.Certificate) error
	UpdateCertificate(id uint, updates map[string]interface{}) error
	GetCertificate(id uint) (*models.Certificate, error)
	// GetByInvite devuelve el certificado de una invitación, o nil sin error.
	GetByInvite(inviteID uint) (*models.Certificate, error)
	GetByCode(code string) (*models.Certificate, error)
	ListByUser(userID uint) ([]models.Certificate, error)
	// ListByProgram devuelve los emitidos para un programa, con el nombre de
	// la persona, del más reciente al más antiguo.
	ListByProgram(programID uint, limit int) ([]models.IssuedCertificate, error)
	CountByProgram(programID uint) (int64, error)
}

type certificateRepository struct {
	db *gorm.DB
}

func NewCertificateRepository(db *gorm.DB) CertificateRepository {
	return &certificateRepository{db: db}
}

func (r *certificateRepository) ListTemplates() ([]models.CertificateTemplate, error) {
	var templates []models.CertificateTemplate
	if err := r.db.Order("name ASC, id ASC").Find(&templates).Error; err != nil {
		return nil, err
	}
	for i := range templates {
		names, err := r.ProgramsUsingTemplate(templates[i].ID)
		if err != nil {
			return nil, err
		}
		templates[i].ProgramNames = names
	}
	return templates, nil
}

func (r *certificateRepository) GetTemplate(id uint) (*models.CertificateTemplate, error) {
	var t models.CertificateTemplate
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	names, err := r.ProgramsUsingTemplate(id)
	if err != nil {
		return nil, err
	}
	t.ProgramNames = names
	return &t, nil
}

func (r *certificateRepository) CreateTemplate(t *models.CertificateTemplate) error {
	return r.db.Create(t).Error
}

func (r *certificateRepository) UpdateTemplate(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.CertificateTemplate{}).Where("id = ?", id).Updates(updates).Error
}

func (r *certificateRepository) DeleteTemplate(id uint) error {
	return r.db.Delete(&models.CertificateTemplate{}, id).Error
}

func (r *certificateRepository) ProgramsUsingTemplate(id uint) ([]string, error) {
	names := []string{}
	err := r.db.Table("induction_programs").
		Where("certificate_template_id = ? AND deleted_at IS NULL", id).
		Order("name ASC").Pluck("name", &names).Error
	return names, err
}

func (r *certificateRepository) CreateCertificate(c *models.Certificate) error {
	return r.db.Create(c).Error
}

func (r *certificateRepository) UpdateCertificate(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.Certificate{}).Where("id = ?", id).Updates(updates).Error
}

func (r *certificateRepository) GetCertificate(id uint) (*models.Certificate, error) {
	var c models.Certificate
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *certificateRepository) GetByInvite(inviteID uint) (*models.Certificate, error) {
	var c models.Certificate
	err := r.db.Where("invite_id = ?", inviteID).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *certificateRepository) GetByCode(code string) (*models.Certificate, error) {
	var c models.Certificate
	if err := r.db.Where("code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *certificateRepository) ListByProgram(programID uint, limit int) ([]models.IssuedCertificate, error) {
	var out []models.IssuedCertificate
	q := r.db.Table("certificates c").
		Select("c.id, c.user_id, u.name AS user_name, c.code, c.program_name, c.issued_at, c.reissued_at").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Where("c.program_id = ?", programID).
		Order("c.issued_at DESC, c.id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Scan(&out).Error
	return out, err
}

func (r *certificateRepository) CountByProgram(programID uint) (int64, error) {
	var n int64
	err := r.db.Model(&models.Certificate{}).Where("program_id = ?", programID).Count(&n).Error
	return n, err
}

func (r *certificateRepository) ListByUser(userID uint) ([]models.Certificate, error) {
	var certs []models.Certificate
	err := r.db.Where("user_id = ?", userID).Order("issued_at DESC, id DESC").Find(&certs).Error
	return certs, err
}
