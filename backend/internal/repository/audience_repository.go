package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type AudienceRepository interface {
	CreateGroup(group *models.AudienceGroup) error
	GetGroups() ([]models.AudienceGroup, error)
	GetGroupByID(id uint) (*models.AudienceGroup, error)
	UpdateGroup(group *models.AudienceGroup) error
	DeleteGroup(id uint) error
	AddMember(groupID uint, userID uint) error
	RemoveMember(groupID uint, userID uint) error
}

type audienceRepository struct {
	db *gorm.DB
}

func NewAudienceRepository(db *gorm.DB) AudienceRepository {
	return &audienceRepository{db: db}
}

func (r *audienceRepository) CreateGroup(group *models.AudienceGroup) error {
	return r.db.Create(group).Error
}

func (r *audienceRepository) GetGroups() ([]models.AudienceGroup, error) {
	var groups []models.AudienceGroup
	err := r.db.Preload("Members").Find(&groups).Error
	return groups, err
}

func (r *audienceRepository) GetGroupByID(id uint) (*models.AudienceGroup, error) {
	var group models.AudienceGroup
	err := r.db.Preload("Members").First(&group, id).Error
	return &group, err
}

func (r *audienceRepository) UpdateGroup(group *models.AudienceGroup) error {
	return r.db.Model(group).Updates(map[string]interface{}{
		"name":        group.Name,
		"description": group.Description,
	}).Error
}

func (r *audienceRepository) DeleteGroup(id uint) error {
	return r.db.Delete(&models.AudienceGroup{}, id).Error
}

func (r *audienceRepository) AddMember(groupID uint, userID uint) error {
	var group models.AudienceGroup
	if err := r.db.First(&group, groupID).Error; err != nil {
		return err
	}
	var user models.User
	if err := r.db.First(&user, userID).Error; err != nil {
		return err
	}
	return r.db.Model(&group).Association("Members").Append(&user)
}

func (r *audienceRepository) RemoveMember(groupID uint, userID uint) error {
	var group models.AudienceGroup
	if err := r.db.First(&group, groupID).Error; err != nil {
		return err
	}
	var user models.User
	if err := r.db.First(&user, userID).Error; err != nil {
		return err
	}
	return r.db.Model(&group).Association("Members").Delete(&user)
}
