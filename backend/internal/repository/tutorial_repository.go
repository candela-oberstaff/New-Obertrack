package repository

// TutorialRepository manages onboarding tutorials.
// NOTE: No tenant_id filtering — tutorials are platform-wide content managed only
// by superadmins (writes are behind RequireSuperadmin(), see routes/platform_routes.go).
// Read visibility is segmented by audience: 'all', 'empleador' or 'profesional'.

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TutorialRepository interface {
	FindAll(onlyActive bool, audience string) ([]models.Tutorial, error)
	GetByID(id uint) (*models.Tutorial, error)
	Create(tutorial *models.Tutorial) error
	Update(tutorial *models.Tutorial, updates map[string]interface{}) error
	Delete(id uint) error
	Reorder(ids []uint) error
	RecordView(tutorialID, userID uint) error
	GetUserViewedIDs(userID uint) ([]uint, error)
}

type tutorialRepository struct {
	db *gorm.DB
}

func NewTutorialRepository(db *gorm.DB) TutorialRepository {
	return &tutorialRepository{db: db}
}

// FindAll lists tutorials. audience == "" means no audience filter (platform staff);
// otherwise only tutorials targeted at that audience or at everyone are returned.
func (r *tutorialRepository) FindAll(onlyActive bool, audience string) ([]models.Tutorial, error) {
	var tutorials []models.Tutorial
	query := r.db.Model(&models.Tutorial{}).Preload("Creator")
	if onlyActive {
		query = query.Where("is_active = ?", true)
	}
	if audience != "" {
		query = query.Where("audience IN ?", []string{models.TutorialAudienceAll, audience})
	}
	if err := query.Order("order_index ASC, created_at DESC").Find(&tutorials).Error; err != nil {
		return nil, err
	}
	return tutorials, nil
}

func (r *tutorialRepository) GetByID(id uint) (*models.Tutorial, error) {
	var tutorial models.Tutorial
	if err := r.db.Preload("Creator").First(&tutorial, id).Error; err != nil {
		return nil, err
	}
	return &tutorial, nil
}

func (r *tutorialRepository) Create(tutorial *models.Tutorial) error {
	return r.db.Create(tutorial).Error
}

func (r *tutorialRepository) Update(tutorial *models.Tutorial, updates map[string]interface{}) error {
	return r.db.Model(tutorial).Updates(updates).Error
}

func (r *tutorialRepository) Delete(id uint) error {
	return r.db.Delete(&models.Tutorial{}, id).Error
}

func (r *tutorialRepository) Reorder(ids []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&models.Tutorial{}).Where("id = ?", id).Update("order_index", index).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *tutorialRepository) RecordView(tutorialID, userID uint) error {
	now := time.Now()
	view := models.TutorialView{
		TutorialID: tutorialID,
		UserID:     userID,
		ViewedAt:   now,
		UpdatedAt:  now,
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tutorial_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
	}).Create(&view).Error
}

func (r *tutorialRepository) GetUserViewedIDs(userID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&models.TutorialView{}).
		Where("user_id = ?", userID).
		Pluck("tutorial_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
