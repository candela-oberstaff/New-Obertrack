package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type BoardRepository interface {
	GetDB() *gorm.DB
	GetByID(id uint) (*models.Board, error)
	Create(board *models.Board) error
	Update(board *models.Board, updates map[string]interface{}) error
	Delete(board *models.Board) error
}

type boardRepository struct {
	db *gorm.DB
}

func NewBoardRepository(db *gorm.DB) BoardRepository {
	return &boardRepository{db: db}
}

func (r *boardRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *boardRepository) GetByID(id uint) (*models.Board, error) {
	var board models.Board
	if err := r.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC")
	}).First(&board, id).Error; err != nil {
		return nil, err
	}
	return &board, nil
}

func (r *boardRepository) Create(board *models.Board) error {
	return r.db.Create(board).Error
}

func (r *boardRepository) Update(board *models.Board, updates map[string]interface{}) error {
	return r.db.Model(board).Updates(updates).Error
}

func (r *boardRepository) Delete(board *models.Board) error {
	return r.db.Delete(board).Error
}
