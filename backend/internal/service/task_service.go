package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type TaskService interface {
	GetAll(userID uint, role string, isManager, isSuperadmin bool, empleadorID uint, boardIDStr, status, priority string, offset, limit int) ([]models.Task, int64, error)
	GetByID(id uint) (*models.Task, error)
	Create(userID uint, isSuperadmin bool, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error)
	Update(id uint, isSuperadmin bool, reqData map[string]interface{}, assignees *[]uint) (*models.Task, []models.User, error)
	Delete(id uint) error
	ToggleCompletion(id uint) (*models.Task, error)
	AddComment(id uint, userID uint, content string) (*models.Comment, error)
	AddAttachment(taskID uint, fileName, fileURL string, fileSize int64, mimeType string, uploadedBy uint) (*models.TaskAttachment, error)
	DeleteAttachment(attachmentID uint) error
}

type taskService struct {
	repo          repository.TaskRepository
	userRepo      repository.UserRepository
	boardRepo     repository.BoardRepository
	notifSvc      NotificationService
	googleChatSvc GoogleChatService
}

func NewTaskService(
	repo repository.TaskRepository,
	userRepo repository.UserRepository,
	boardRepo repository.BoardRepository,
	notifSvc NotificationService,
	googleChatSvc GoogleChatService,
) TaskService {
	return &taskService{
		repo:          repo,
		userRepo:      userRepo,
		boardRepo:     boardRepo,
		notifSvc:      notifSvc,
		googleChatSvc: googleChatSvc,
	}
}

func (s *taskService) GetAll(userID uint, role string, isManager, isSuperadmin bool, empleadorID uint, boardIDStr, status, priority string, offset, limit int) ([]models.Task, int64, error) {
	filters := make(map[string]interface{})

	if boardIDStr != "" && boardIDStr != "all" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err == nil {
			filters["board_id"] = uint(boardID)
		}
	}

	if !isSuperadmin {
		companyID := userID
		if role == string(models.UserTypeProfessional) || role == "profesional" {
			companyID = empleadorID
		}
		if companyID > 0 {
			filters["company_id"] = companyID
		}

		if role == string(models.UserTypeProfessional) || role == "profesional" {
			filters["assignee_id"] = userID
			filters["created_by"] = userID
		} else if (role == string(models.UserTypeEmployer) || role == "empleador") && empleadorID > 0 {
			// This logic could be a repository method for "FindAllByEmployer"
			filters["employer_id"] = empleadorID
		} else if !isManager {
			filters["created_by"] = userID
		}
	}

	if status != "" {
		filters["status"] = status
	}
	if priority != "" {
		filters["priority"] = priority
	}

	return s.repo.FindAll(filters, offset, limit)
}

func (s *taskService) GetByID(id uint) (*models.Task, error) {
	return s.repo.GetByID(id)
}

func (s *taskService) validateAssignees(boardID uint, assignees []uint, isSuperadmin bool) error {
	if isSuperadmin {
		return nil
	}
	board, err := s.boardRepo.GetByID(boardID)
	if err != nil {
		return errors.New("El tablero especificado no existe o fue eliminado")
	}

	memberIDs := make(map[uint]bool)
	for _, m := range board.Members {
		memberIDs[m.ID] = true
	}
	if board.CreatedBy != 0 {
		memberIDs[board.CreatedBy] = true
	}

	for _, assigneeID := range assignees {
		if !memberIDs[assigneeID] {
			user, _ := s.userRepo.GetByID(assigneeID)
			userName := fmt.Sprintf("ID %d", assigneeID)
			if user != nil {
				userName = user.Name
			}
			return fmt.Errorf("%s no es miembro del tablero", userName)
		}
	}
	return nil
}

func (s *taskService) Create(userID uint, isSuperadmin bool, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error) {

	if title == "" {
		return nil, nil, errors.New("Title is required")
	}

	if boardID == 0 {
		return nil, nil, errors.New("Debes seleccionar un tablero para crear una tarea")
	}

	if len(assignees) > 0 && boardID > 0 {
		if err := s.validateAssignees(boardID, assignees, isSuperadmin); err != nil {
			return nil, nil, err
		}

		if isSuperadmin {
			// Auto-add assignees to board if they are not members
			board, _ := s.boardRepo.GetByID(boardID)
			if board != nil {
				memberIDs := make(map[uint]bool)
				for _, m := range board.Members {
					memberIDs[m.ID] = true
				}
				for _, mid := range assignees {
					if !memberIDs[mid] {
						u, _ := s.userRepo.GetByID(mid)
						if u != nil {
							s.boardRepo.AddMember(board, u)
						}
					}
				}
			}
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

	if len(assignees) > 0 {
		s.repo.SyncAssignees(task, assignees)
	}

	finalTask, err := s.repo.GetByID(task.ID)
	if err != nil {
		log.Printf("[TaskService] Error refreshing task %d for notifications: %v", task.ID, err)
		return task, nil, nil // Return the original task but continue
	}

	log.Printf("[TaskService] Notifying %d assignees for new task: %s", len(finalTask.Assignees), task.Title)

	for _, assignee := range finalTask.Assignees {
		err := s.notifSvc.CreateNotification(assignee.ID, "task_assigned", "Nueva tarea asignada", fmt.Sprintf("Se te asignó la tarea: %s", task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
		})
		if err != nil {
			log.Printf("[TaskService] Error creating internal notification for user %d: %v", assignee.ID, err)
		}

		// Enviar DM vía Google Chat API
		if assignee.Email != "" {
			go s.googleChatSvc.SendDirectMessage(assignee.Email, fmt.Sprintf("¡Hola %s! Se te asignó una nueva tarea en Obertrack: *%s*", assignee.Name, task.Title))
		} else {
			log.Printf("[TaskService] Warning: Assignee %d (%s) has no email, skipping Google Chat notification", assignee.ID, assignee.Name)
		}
	}

	return finalTask, finalTask.Assignees, nil
}

func (s *taskService) Update(id uint, isSuperadmin bool, reqData map[string]interface{}, assignees *[]uint) (*models.Task, []models.User, error) {
	task, err := s.repo.GetByID(id)
	if err != nil {
		return nil, nil, errors.New("Task not found")
	}

	if len(reqData) > 0 {
		if err := s.repo.Update(task, reqData); err != nil {
			return nil, nil, err
		}
	}

	if assignees != nil {
		if err := s.validateAssignees(task.BoardID, *assignees, isSuperadmin); err != nil {
			return nil, nil, err
		}

		if isSuperadmin {
			// Auto-add assignees to board if they are not members
			board, _ := s.boardRepo.GetByID(task.BoardID)
			if board != nil {
				memberIDs := make(map[uint]bool)
				for _, m := range board.Members {
					memberIDs[m.ID] = true
				}
				for _, mid := range *assignees {
					if !memberIDs[mid] {
						u, _ := s.userRepo.GetByID(mid)
						if u != nil {
							s.boardRepo.AddMember(board, u)
						}
					}
				}
			}
		}

		currentAssigneeIDs := make(map[uint]bool)
		for _, a := range task.Assignees {
			currentAssigneeIDs[a.ID] = true
		}

		s.repo.SyncAssignees(task, *assignees)

		finalTask, err := s.repo.GetByID(task.ID)
		if err != nil {
			log.Printf("[TaskService] Error refreshing task %d for update notifications: %v", task.ID, err)
		} else {
			task = finalTask
		}

		log.Printf("[TaskService] Checking notifications for %d assignees in updated task: %s", len(task.Assignees), task.Title)

		// Notify new assignees
		for _, u := range task.Assignees {
			if !currentAssigneeIDs[u.ID] {
				err := s.notifSvc.CreateNotification(u.ID, "task_assigned", "Nueva tarea asignada", fmt.Sprintf("Se te asignó la tarea: %s", task.Title), map[string]interface{}{
					"task_id":  task.ID,
					"board_id": task.BoardID,
				})
				if err != nil {
					log.Printf("[TaskService] Error creating internal notification for user %d: %v", u.ID, err)
				}

				// Enviar DM vía Google Chat API
				if u.Email != "" {
					go s.googleChatSvc.SendDirectMessage(u.Email, fmt.Sprintf("¡Hola %s! Se te asignó a la tarea en Obertrack: *%s*", u.Name, task.Title))
				} else {
					log.Printf("[TaskService] Warning: Assignee %d (%s) has no email, skipping Google Chat notification", u.ID, u.Name)
				}
			}
		}
	} else {
		finalTask, _ := s.repo.GetByID(task.ID)
		task = finalTask
	}

	return task, task.Assignees, nil
}

func (s *taskService) Delete(id uint) error {
	// Delete related notifications
	_ = s.notifSvc.DeleteByTaskID(id)
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

	return s.repo.GetComment(comment.ID)
}

func (s *taskService) AddAttachment(taskID uint, fileName, fileURL string, fileSize int64, mimeType string, uploadedBy uint) (*models.TaskAttachment, error) {
	attachment := &models.TaskAttachment{
		TaskID:     taskID,
		FileName:   fileName,
		FileURL:    fileURL,
		FileSize:   fileSize,
		MimeType:   mimeType,
		UploadedBy: uploadedBy,
	}

	if err := s.repo.AddAttachment(attachment); err != nil {
		return nil, err
	}
	return attachment, nil
}

func (s *taskService) DeleteAttachment(attachmentID uint) error {
	attachment, err := s.repo.GetAttachmentByID(attachmentID)
	if err != nil {
		return err
	}
	return s.repo.DeleteAttachment(attachment)
}
