package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type TaskService interface {
	GetAll(userID uint, role string, isManager, isSuperadmin bool, empleadorID uint, boardIDStr, status, priority string, offset, limit int) ([]models.Task, int64, error)
	GetByID(id uint) (*models.Task, error)
	Create(userID uint, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error)
	Update(id uint, reqData map[string]interface{}, assignees *[]uint) (*models.Task, []models.User, error)
	Delete(id uint) error
	ToggleCompletion(id uint) (*models.Task, error)
	AddComment(id uint, userID uint, content string) (*models.Comment, error)
}

type taskService struct {
	repo repository.TaskRepository
}

func NewTaskService(repo repository.TaskRepository) TaskService {
	return &taskService{repo: repo}
}

func (s *taskService) GetAll(userID uint, role string, isManager, isSuperadmin bool, empleadorID uint, boardIDStr, status, priority string, offset, limit int) ([]models.Task, int64, error) {
	db := s.repo.GetDB()
	query := db.Model(&models.Task{})

	if boardIDStr != "" && boardIDStr != "all" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err == nil {
			query = query.Where("board_id = ?", boardID)
		}
	} else {
		query = query.Where("board_id IN (?)", db.Model(&models.Board{}).Select("id"))
	}

	if isSuperadmin {
		// all tasks
	} else if role == string(models.UserTypeProfessional) {
		var assignedTaskIDs []uint
		db.Table("task_users").Where("user_id = ?", userID).Pluck("task_id", &assignedTaskIDs)
		if len(assignedTaskIDs) > 0 {
			query = query.Where("(created_by = ? OR id IN (?))", userID, assignedTaskIDs)
		} else {
			query = query.Where("created_by = ?", userID)
		}
	} else if role == string(models.UserTypeEmployer) || role == "empleador" {
		if empleadorID > 0 {
			subquery := db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
			query = query.Where("created_by IN (?)", subquery)
		}
	} else if !isManager {
		query = query.Where("created_by = ?", userID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	return s.repo.GetAll(query, offset, limit)
}

func (s *taskService) GetByID(id uint) (*models.Task, error) {
	return s.repo.GetByID(id)
}

func (s *taskService) validateAssignees(boardID uint, assignees []uint) error {
	db := s.repo.GetDB()
	var board models.Board
	if err := db.First(&board, boardID).Error; err != nil {
		return errors.New("El tablero especificado no existe o fue eliminado")
	}

	var boardMembers []models.BoardMember
	db.Where("board_id = ?", boardID).Find(&boardMembers)
	memberIDs := make(map[uint]bool)
	for _, m := range boardMembers {
		memberIDs[m.UserID] = true
	}
	if board.CreatedBy != 0 {
		memberIDs[board.CreatedBy] = true
	}

	for _, assigneeID := range assignees {
		if !memberIDs[assigneeID] {
			var assigneeUser models.User
			userName := fmt.Sprintf("ID %d", assigneeID)
			if db.First(&assigneeUser, assigneeID).Error == nil {
				userName = assigneeUser.Name
			}
			return fmt.Errorf("%s no es miembro del tablero", userName)
		}
	}
	return nil
}

func (s *taskService) Create(userID uint, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error) {

	if title == "" {
		return nil, nil, errors.New("Title is required")
	}

	if len(assignees) > 0 && boardID > 0 {
		if err := s.validateAssignees(boardID, assignees); err != nil {
			return nil, nil, err
		}
	}

	task := &models.Task{
		Title:       utils.SanitizeHTML(title),
		Description: utils.SanitizeHTML(description),
		Status:      models.TaskStatusTodo,
		Priority:    models.PriorityMedium,
		CreatedBy:   userID,
		BoardID:     boardID,
	}

	if priority != "" {
		task.Priority = models.TaskPriority(priority)
	}

	if endDate != nil && *endDate != "" {
		parsedEndDate, err := time.Parse("2006-01-02", *endDate)
		if err == nil {
			task.EndDate = &parsedEndDate
		}
	}

	if err := s.repo.Create(task); err != nil {
		return nil, nil, err
	}

	var usersToNotify []models.User
	if len(assignees) > 0 {
		s.repo.GetDB().Find(&usersToNotify, assignees)
		s.repo.GetDB().Model(task).Association("Assignees").Append(usersToNotify)
	}

	finalTask, _ := s.repo.GetByID(task.ID)
	return finalTask, usersToNotify, nil
}

func (s *taskService) Update(id uint, reqData map[string]interface{}, assignees *[]uint) (*models.Task, []models.User, error) {
	task, err := s.repo.GetByID(id)
	if err != nil {
		return nil, nil, errors.New("Task not found")
	}

	if len(reqData) > 0 {
		if err := s.repo.Update(task, reqData); err != nil {
			return nil, nil, err
		}
	}

	var usersToNotify []models.User
	if assignees != nil {
		if err := s.validateAssignees(task.BoardID, *assignees); err != nil {
			return nil, nil, err
		}

		var currentAssignees []models.User
		s.repo.GetDB().Model(task).Association("Assignees").Find(&currentAssignees)
		currentAssigneeIDs := make(map[uint]bool)
		for _, a := range currentAssignees {
			currentAssigneeIDs[a.ID] = true
		}

		if len(*assignees) == 0 {
			s.repo.GetDB().Model(task).Association("Assignees").Clear()
		} else {
			s.repo.GetDB().Find(&usersToNotify, *assignees)
			s.repo.GetDB().Model(task).Association("Assignees").Replace(usersToNotify)

			// Solo notificar a los nuevos
			var newUsers []models.User
			for _, u := range usersToNotify {
				if !currentAssigneeIDs[u.ID] {
					newUsers = append(newUsers, u)
				}
			}
			usersToNotify = newUsers
		}
	}

	finalTask, _ := s.repo.GetByID(task.ID)
	return finalTask, usersToNotify, nil
}

func (s *taskService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *taskService) ToggleCompletion(id uint) (*models.Task, error) {
	task, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("Task not found")
	}

	completed := !task.Completed
	status := models.TaskStatusTodo
	if completed {
		status = models.TaskStatusDone
	}

	updates := map[string]interface{}{
		"completed": completed,
		"status":    status,
	}

	if err := s.repo.Update(task, updates); err != nil {
		return nil, err
	}

	return s.repo.GetByID(id)
}

func (s *taskService) AddComment(id uint, userID uint, content string) (*models.Comment, error) {
	_, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("Task not found")
	}

	comment := &models.Comment{
		TaskID:  id,
		UserID:  userID,
		Content: utils.SanitizeHTML(content),
	}

	if err := s.repo.AddComment(comment); err != nil {
		return nil, err
	}

	s.repo.GetDB().Preload("User").First(comment, comment.ID)
	return comment, nil
}
