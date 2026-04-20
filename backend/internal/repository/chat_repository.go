package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type ChatRepository interface {
	Create(message *models.Message) error
	FindAllWithFilters(filters map[string]interface{}, limit int) ([]models.Message, error)
}

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *chatRepository) Create(message *models.Message) error {
	return r.db.Create(message).Error
}

func (r *chatRepository) FindAllWithFilters(filters map[string]interface{}, limit int) ([]models.Message, error) {
	var messages []models.Message
	query := r.db.Model(&models.Message{})

	if employerID, ok := filters["employer_id"].(uint); ok {
		query = query.Joins("JOIN users ON users.id = messages.user_id").
			Where("users.empleador_id = ? OR (messages.company_id = ? AND messages.company_id IS NOT NULL)", employerID, employerID)
	} else {
		if userIDs, ok := filters["user_ids"].([]uint); ok {
			query = query.Where("user_id IN (?)", userIDs)
		}

		if companyID, ok := filters["company_id"].(uint); ok {
			query = query.Where("company_id = ?", companyID)
		}

		if userID, ok := filters["user_id"].(uint); ok {
			query = query.Where("user_id = ?", userID)
		}
	}

	err := query.Order("created_at DESC").Limit(limit).Find(&messages).Error
	return messages, err
}
