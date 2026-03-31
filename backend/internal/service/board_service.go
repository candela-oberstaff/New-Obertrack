package service

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"gorm.io/gorm"
)

type BoardService interface {
	GetAll(userID uint, role string, isSuperadmin bool) ([]models.Board, error)
	GetPublicBoards(userID uint) ([]models.Board, error)
	JoinBoard(userID, boardID uint) (*models.Board, error)
	GetByID(userID uint, role string, isSuperadmin bool, boardID uint) (*models.Board, error)
	Create(userID uint, name, description, color string, memberIDs []uint, phases []struct {
		Name  string
		Color string
	}) (*models.Board, error)
	Update(boardID uint, updates map[string]interface{}, memberIDs []uint) (*models.Board, error)
	Delete(userID, boardID uint) error
	AddPhase(boardID uint, name, color string) (*models.Board, error)
	RemovePhase(boardID, phaseID uint) (*models.Board, error)
	ReorderPhases(boardID uint, phaseIDs []uint) (*models.Board, error)
}

type boardService struct {
	repo repository.BoardRepository
}

func NewBoardService(repo repository.BoardRepository) BoardService {
	return &boardService{repo: repo}
}

func (s *boardService) GetAll(userID uint, role string, isSuperadmin bool) ([]models.Board, error) {
	var boards []models.Board
	db := s.repo.GetDB()

	if isSuperadmin || role == "superadmin" {
		db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).Find(&boards)
	} else {
		var boardIDs []uint
		db.Model(&models.Board{}).
			Select("boards.id").
			Joins("LEFT JOIN board_members ON board_members.board_id = boards.id").
			Where("board_members.user_id = ? OR boards.created_by = ?", userID, userID).
			Group("boards.id").
			Pluck("boards.id", &boardIDs)

		if len(boardIDs) > 0 {
			db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
				return db.Order("\"order\" ASC")
			}).Where("boards.id IN ?", boardIDs).Find(&boards)
		}
	}
	return boards, nil
}

func (s *boardService) GetPublicBoards(userID uint) ([]models.Board, error) {
	db := s.repo.GetDB()
	var myBoardIDs []uint
	db.Model(&models.Board{}).
		Select("boards.id").
		Joins("LEFT JOIN board_members ON board_members.board_id = boards.id").
		Where("board_members.user_id = ? OR boards.created_by = ?", userID, userID).
		Group("boards.id").
		Pluck("boards.id", &myBoardIDs)

	var boards []models.Board
	query := db.Preload("Creator").Preload("Members")
	if len(myBoardIDs) > 0 {
		query = query.Where("id NOT IN ?", myBoardIDs)
	}
	query.Find(&boards)
	return boards, nil
}

func (s *boardService) JoinBoard(userID, boardID uint) (*models.Board, error) {
	db := s.repo.GetDB()
	_, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	var existing int64
	db.Model(&models.BoardMember{}).Where("board_id = ? AND user_id = ?", boardID, userID).Count(&existing)
	if existing > 0 {
		return nil, errors.New("Ya eres miembro de este tablero")
	}

	bm := models.BoardMember{BoardID: boardID, UserID: userID}
	if err := db.Create(&bm).Error; err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) GetByID(userID uint, role string, isSuperadmin bool, boardID uint) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	if !isSuperadmin && role != "superadmin" {
		isMember := false
		for _, m := range board.Members {
			if m.ID == userID {
				isMember = true
				break
			}
		}
		if board.CreatedBy != userID && !isMember {
			return nil, errors.New("Access denied")
		}
	}
	return board, nil
}

func (s *boardService) Create(userID uint, name, description, color string, memberIDs []uint, phases []struct {
	Name  string
	Color string
}) (*models.Board, error) {
	db := s.repo.GetDB()

	if color == "" {
		color = "#3b82f6"
	}

	board := &models.Board{
		Name:        name,
		Description: description,
		Color:       color,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(board); err != nil {
		return nil, err
	}

	if len(memberIDs) > 0 {
		var members []models.User
		db.Find(&members, memberIDs)
		db.Model(board).Association("Members").Append(members)
	}

	bm := models.BoardMember{
		BoardID: board.ID,
		UserID:  userID,
	}
	db.Create(&bm)

	phasesToCreate := phases
	if len(phasesToCreate) == 0 {
		phasesToCreate = []struct {
			Name  string
			Color string
		}{
			{Name: "Por hacer", Color: "#6b7280"},
			{Name: "En proceso", Color: "#3b82f6"},
			{Name: "Finalizado", Color: "#22c55e"},
		}
	}

	statusNames := []string{"por_hacer", "en_proceso", "finalizado", "", "", ""}
	for i, p := range phasesToCreate {
		scolor := p.Color
		if scolor == "" {
			scolor = "#6b7280"
		}
		status := ""
		if i < len(statusNames) {
			status = statusNames[i]
		}
		phase := models.Phase{
			Name:   p.Name,
			Color:  scolor,
			Status: status,
			Order:  i,
		}
		db.Create(&phase)
		db.Create(&models.BoardPhase{
			BoardID: board.ID,
			PhaseID: phase.ID,
		})
	}

	return s.repo.GetByID(board.ID)
}

func (s *boardService) Update(boardID uint, updates map[string]interface{}, memberIDs []uint) (*models.Board, error) {
	db := s.repo.GetDB()
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	if len(updates) > 0 {
		s.repo.Update(board, updates)
	}

	if len(memberIDs) > 0 {
		var members []models.User
		db.Find(&members, memberIDs)
		db.Model(board).Association("Members").Replace(members)
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) Delete(userID, boardID uint) error {
	db := s.repo.GetDB()
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return errors.New("Board not found")
	}

	if board.CreatedBy != userID {
		return errors.New("Solo el creador puede eliminar el tablero")
	}

	db.Unscoped().Where("board_id = ?", boardID).Delete(&models.Phase{})
	db.Unscoped().Where("board_id = ?", boardID).Delete(&models.BoardMember{})

	var taskIDs []uint
	db.Unscoped().Model(&models.Task{}).Where("board_id = ?", boardID).Pluck("id", &taskIDs)
	if len(taskIDs) > 0 {
		db.Unscoped().Where("task_id IN ?", taskIDs).Delete(&models.Comment{})
		db.Unscoped().Where("task_id IN ?", taskIDs).Delete(&models.TaskAttachment{})
		db.Unscoped().Where("task_id IN ?", taskIDs).Delete(&models.TaskUser{})
	}

	db.Unscoped().Where("board_id = ?", boardID).Delete(&models.Task{})
	return s.repo.Delete(board)
}

func (s *boardService) AddPhase(boardID uint, name, color string) (*models.Board, error) {
	db := s.repo.GetDB()
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	var maxOrder int
	db.Model(&models.Phase{}).
		Joins("JOIN board_phases ON board_phases.phase_id = phases.id").
		Where("board_phases.board_id = ?", boardID).
		Select("COALESCE(MAX(phases.\"order\"), -1)").Scan(&maxOrder)

	if color == "" {
		color = "#6b7280"
	}

	phase := models.Phase{
		Name:  name,
		Color: color,
		Order: maxOrder + 1,
	}

	if err := db.Create(&phase).Error; err != nil {
		return nil, err
	}

	db.Create(&models.BoardPhase{
		BoardID: board.ID,
		PhaseID: phase.ID,
	})

	return s.repo.GetByID(boardID)
}

func getPhaseStatusName(phaseID uint) string {
	names := map[uint]string{
		1: "por_hacer",
		2: "en_proceso",
		3: "finalizado",
	}
	return names[phaseID]
}

func (s *boardService) RemovePhase(boardID, phaseID uint) (*models.Board, error) {
	db := s.repo.GetDB()
	_, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	var count int64
	db.Model(&models.BoardPhase{}).Where("board_id = ? AND phase_id = ?", boardID, phaseID).Count(&count)
	if count == 0 {
		return nil, errors.New("Phase not found on this board")
	}

	db.Where("board_id = ? AND phase_id = ?", boardID, phaseID).Delete(&models.BoardPhase{})

	var taskCount int64
	db.Model(&models.Task{}).Where("board_id = ? AND status = ?", boardID, getPhaseStatusName(phaseID)).Count(&taskCount)
	if taskCount > 0 {
		return nil, errors.New("Cannot remove phase with tasks. Move or delete tasks first.")
	}

	var otherBoards int64
	db.Model(&models.BoardPhase{}).Where("phase_id = ? AND board_id != ?", phaseID, boardID).Count(&otherBoards)
	if otherBoards == 0 {
		db.Delete(&models.Phase{}, phaseID)
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) ReorderPhases(boardID uint, phaseIDs []uint) (*models.Board, error) {
	db := s.repo.GetDB()
	_, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	for i, phaseID := range phaseIDs {
		db.Model(&models.Phase{}).Where("id = ?", phaseID).Update("order", i)
	}

	return s.repo.GetByID(boardID)
}
