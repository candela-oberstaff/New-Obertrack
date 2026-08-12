package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EmailSettingRepository guarda los interruptores de correo por tipo. Solo
// existe fila para los tipos que alguien tocó: la ausencia significa activo.
type EmailSettingRepository interface {
	List() ([]models.EmailSetting, error)
	Upsert(setting *models.EmailSetting) error
}

type emailSettingRepository struct {
	db *gorm.DB
}

func NewEmailSettingRepository(db *gorm.DB) EmailSettingRepository {
	return &emailSettingRepository{db: db}
}

func (r *emailSettingRepository) List() ([]models.EmailSetting, error) {
	var settings []models.EmailSetting
	err := r.db.Find(&settings).Error
	return settings, err
}

func (r *emailSettingRepository) Upsert(setting *models.EmailSetting) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_by", "updated_at"}),
	}).Create(setting).Error
}
