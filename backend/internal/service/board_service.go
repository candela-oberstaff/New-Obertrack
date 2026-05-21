package service

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type BoardService interface {
	GetAll(userID uint, role string, isSuperadmin bool, companyID uint) ([]models.Board, error)
	GetPublicBoards(userID uint, companyID uint) ([]models.Board, error)
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
	repo     repository.BoardRepository
	userRepo repository.UserRepository
}

func NewBoardService(repo repository.BoardRepository, userRepo repository.UserRepository) BoardService {
	return &boardService{
		repo:     repo,
		userRepo: userRepo,
	}
}

func (s *boardService) GetAll(userID uint, role string, isSuperadmin bool, companyID uint) ([]models.Board, error) {
	filters := make(map[string]interface{})

	if !isSuperadmin && role != "superadmin" {
		filters["user_id"] = userID
	}

	if companyID > 0 {
		filters["company_id"] = companyID
	}

	return s.repo.FindAll(filters)
}

func (s *boardService) GetPublicBoards(userID uint, companyID uint) ([]models.Board, error) {
	// Simplified: boards that I'm NOT in.
	filters := make(map[string]interface{})
	if companyID > 0 {
		filters["company_id"] = companyID
	}
	all, err := s.repo.FindAll(filters)
	if err != nil {
		return nil, err
	}

	var public []models.Board
	for _, b := range all {
		isMe := b.CreatedBy == userID
		if !isMe {
			isMember := false
			for _, m := range b.Members {
				if m.ID == userID {
					isMember = true
					break
				}
			}
			if !isMember {
				public = append(public, b)
			}
		}
	}
	return public, nil
}

func (s *boardService) JoinBoard(userID, boardID uint) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	for _, m := range board.Members {
		if m.ID == userID {
			return nil, errors.New("Ya eres miembro de este tablero")
		}
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if err := s.repo.AddMember(board, user); err != nil {
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

	user, _ := s.userRepo.GetByID(userID)
	if user != nil {
		s.repo.AddMember(board, user)
	}

	if len(memberIDs) > 0 {
		for _, mid := range memberIDs {
			if mid != userID {
				m, _ := s.userRepo.GetByID(mid)
				if m != nil {
					s.repo.AddMember(board, m)
				}
			}
		}
	}

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
		phase := &models.Phase{
			Name:   p.Name,
			Color:  scolor,
			Status: status,
			Order:  i,
		}
		s.repo.AddPhase(board, phase)
	}

	return s.repo.GetByID(board.ID)
}

func (s *boardService) Update(boardID uint, updates map[string]interface{}, memberIDs []uint) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	if len(updates) > 0 {
		if err := s.repo.Update(board, updates); err != nil {
			return nil, err
		}
	}

	if memberIDs != nil {
		// Replace all members
		// In a real refactor we should have a more granular update method in Repo
		// but let's assume we replace them all for now to keep it consistent with original
		for _, m := range board.Members {
			s.repo.RemoveMember(board, m.ID)
		}
		for _, mid := range memberIDs {
			user, _ := s.userRepo.GetByID(mid)
			if user != nil {
				s.repo.AddMember(board, user)
			}
		}
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) Delete(userID, boardID uint) error {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return errors.New("Board not found")
	}

	if board.CreatedBy != userID {
		return errors.New("Solo el creador puede eliminar el tablero")
	}

	// In a real implementation, Repo.Delete should handle cleanup of phases/members via cascades
	// or we can add a Repo.HardDeleteBoard that does all this.
	return s.repo.Delete(board)
}

func (s *boardService) AddPhase(boardID uint, name, color string) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	maxOrder := -1
	for _, p := range board.Phases {
		if p.Order > maxOrder {
			maxOrder = p.Order
		}
	}

	if color == "" {
		color = "#6b7280"
	}

	phase := &models.Phase{
		Name:  name,
		Color: color,
		Order: maxOrder + 1,
	}

	if err := s.repo.AddPhase(board, phase); err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) RemovePhase(boardID, phaseID uint) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	found := false
	var phaseToRemove *models.Phase
	for _, p := range board.Phases {
		if p.ID == phaseID {
			found = true
			phaseToRemove = &p
			break
		}
	}
	if !found {
		return nil, errors.New("Phase not found on this board")
	}

	// Safety check: are there tasks in this phase?
	// We can add a simple FindAll call to check
	tasks, _, _ := s.repo.FindTasksByPhase(boardID, phaseToRemove.Status)
	if len(tasks) > 0 {
		return nil, errors.New("Cannot remove phase with tasks. Move or delete tasks first.")
	}

	if err := s.repo.RemovePhase(board, phaseID); err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) ReorderPhases(boardID uint, phaseIDs []uint) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	if err := s.repo.UpdatePhasesOrder(board, phaseIDs); err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}
