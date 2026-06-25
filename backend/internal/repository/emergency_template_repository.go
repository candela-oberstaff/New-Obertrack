package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type EmergencyTemplateRepository interface {
	List() ([]models.EmergencyTemplate, error)
	GetByID(id uint) (*models.EmergencyTemplate, error)
	Create(template *models.EmergencyTemplate) error
	Update(template *models.EmergencyTemplate, updates map[string]interface{}) error
	Delete(id uint) error
}

type emergencyTemplateRepository struct {
	db *gorm.DB
}

func NewEmergencyTemplateRepository(db *gorm.DB) EmergencyTemplateRepository {
	return &emergencyTemplateRepository{db: db}
}

func (r *emergencyTemplateRepository) List() ([]models.EmergencyTemplate, error) {
	var templates []models.EmergencyTemplate
	err := r.db.Order("created_at DESC").Find(&templates).Error
	return templates, err
}

func (r *emergencyTemplateRepository) GetByID(id uint) (*models.EmergencyTemplate, error) {
	var template models.EmergencyTemplate
	if err := r.db.First(&template, id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *emergencyTemplateRepository) Create(template *models.EmergencyTemplate) error {
	return r.db.Create(template).Error
}

func (r *emergencyTemplateRepository) Update(template *models.EmergencyTemplate, updates map[string]interface{}) error {
	return r.db.Model(template).Updates(updates).Error
}

func (r *emergencyTemplateRepository) Delete(id uint) error {
	return r.db.Delete(&models.EmergencyTemplate{}, id).Error
}
