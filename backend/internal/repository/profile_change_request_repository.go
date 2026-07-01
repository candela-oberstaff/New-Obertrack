package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type ProfileChangeRequestRepository interface {
	Create(req *models.ProfileChangeRequest) error
	GetByID(id uint) (*models.ProfileChangeRequest, error)
	GetPendingByUser(userID uint) (*models.ProfileChangeRequest, error)
	Update(req *models.ProfileChangeRequest, updates map[string]interface{}) error
}

type profileChangeRequestRepository struct {
	db *gorm.DB
}

func NewProfileChangeRequestRepository(db *gorm.DB) ProfileChangeRequestRepository {
	return &profileChangeRequestRepository{db: db}
}

func (r *profileChangeRequestRepository) Create(req *models.ProfileChangeRequest) error {
	return r.db.Create(req).Error
}

func (r *profileChangeRequestRepository) GetByID(id uint) (*models.ProfileChangeRequest, error) {
	var req models.ProfileChangeRequest
	if err := r.db.First(&req, id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *profileChangeRequestRepository) GetPendingByUser(userID uint) (*models.ProfileChangeRequest, error) {
	var req models.ProfileChangeRequest
	err := r.db.Where("user_id = ? AND status = ?", userID, models.ProfileChangePending).
		Order("created_at DESC").First(&req).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *profileChangeRequestRepository) Update(req *models.ProfileChangeRequest, updates map[string]interface{}) error {
	return r.db.Model(req).Updates(updates).Error
}
