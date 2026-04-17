package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type BoardRepository interface {
	FindAll(filters map[string]interface{}) ([]models.Board, error)
	GetByID(id uint) (*models.Board, error)
	Create(board *models.Board) error
	Update(board *models.Board, updates map[string]interface{}) error
	Delete(board *models.Board) error
	AddMember(board *models.Board, user *models.User) error
	RemoveMember(board *models.Board, userID uint) error
	AddPhase(board *models.Board, phase *models.Phase) error
	RemovePhase(board *models.Board, phaseID uint) error
	UpdatePhasesOrder(board *models.Board, orderedPhaseIDs []uint) error
	FindTasksByPhase(boardID uint, status string) ([]models.Task, int64, error)
}

type boardRepository struct {
	db *gorm.DB
}

func NewBoardRepository(db *gorm.DB) BoardRepository {
	return &boardRepository{db: db}
}

func (r *boardRepository) FindAll(filters map[string]interface{}) ([]models.Board, error) {
	var boards []models.Board
	query := r.db.Model(&models.Board{})

	if public, ok := filters["public"].(bool); ok && public {
		// Example: in this system, maybe boards without an employer are public?
		// Or if there's an is_public field (not seen in models yet, but let's assume filtering)
	}

	if userID, ok := filters["user_id"].(uint); ok {
		query = query.Joins("JOIN board_members ON board_members.board_id = boards.id").Where("board_members.user_id = ?", userID).Or("boards.created_by = ?", userID)
	}

	err := query.Preload("Members").Preload("Creator").Order("created_at DESC").Find(&boards).Error
	return boards, err
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

func (r *boardRepository) AddMember(board *models.Board, user *models.User) error {
	return r.db.Model(board).Association("Members").Append(user)
}

func (r *boardRepository) RemoveMember(board *models.Board, userID uint) error {
	return r.db.Model(board).Association("Members").Delete(&models.User{ID: userID})
}

func (r *boardRepository) AddPhase(board *models.Board, phase *models.Phase) error {
	tx := r.db.Begin()
	if err := tx.Create(phase).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(board).Association("Phases").Append(phase); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *boardRepository) RemovePhase(board *models.Board, phaseID uint) error {
	return r.db.Model(board).Association("Phases").Delete(&models.Phase{ID: phaseID})
}

func (r *boardRepository) UpdatePhasesOrder(board *models.Board, orderedPhaseIDs []uint) error {
	tx := r.db.Begin()
	for i, id := range orderedPhaseIDs {
		if err := tx.Model(&models.Phase{}).Where("id = ?", id).Update("order", i).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *boardRepository) FindTasksByPhase(boardID uint, status string) ([]models.Task, int64, error) {
	var tasks []models.Task
	var total int64
	query := r.db.Model(&models.Task{}).Where("board_id = ? AND status = ?", boardID, status)
	err := query.Count(&total).Preload("Creator").Find(&tasks).Error
	return tasks, total, err
}
