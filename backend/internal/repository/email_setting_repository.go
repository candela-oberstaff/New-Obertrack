package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EmailSettingRepository guarda los interruptores y configuración de correo por tipo.
type EmailSettingRepository interface {
	List() ([]models.EmailSetting, error)
	Get(key string) (*models.EmailSetting, error)
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

func (r *emailSettingRepository) Get(key string) (*models.EmailSetting, error) {
	var setting models.EmailSetting
	err := r.db.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *emailSettingRepository) Upsert(setting *models.EmailSetting) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled", "frequency", "day_of_month", "weekday", "hour", "minute", "timezone", "recipients", "updated_by", "updated_at",
		}),
	}).Create(setting).Error
}
