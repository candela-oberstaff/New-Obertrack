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
	GetAll(userID uint, role string, isManager, isSuperadmin bool, tenantID, companyFilter uint, boardIDStr, status, priority, assigneeIDStr, startDate, endDate string, offset, limit int) ([]models.Task, int64, error)
	GetBoardStatusCounts(isSuperadmin bool, tenantID, companyFilter uint) (map[uint]map[string]int, error)
	GetByID(id uint, tenantID uint, isSuperadmin bool) (*models.Task, error)
	Create(userID uint, isSuperadmin bool, tenantID uint, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error)
	Update(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}, assignees *[]uint) (*models.Task, []models.User, error)
	Delete(id uint, tenantID uint, userID uint, role string, isManager, isSuperadmin bool) error
	ToggleCompletion(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool) (*models.Task, error)
	AddComment(id uint, tenantID uint, userID uint, content string, isSuperadmin bool) (*models.Comment, error)
	AddAttachment(taskID uint, tenantID uint, fileName, fileURL string, fileSize int64, mimeType string, uploadedBy uint, isSuperadmin bool) (*models.TaskAttachment, error)
	DeleteAttachment(attachmentID uint, tenantID uint, isSuperadmin bool) error

	// SetSystemDM cablea el emisor de DMs de sistema al chat interno (lo apunta a
	// channelService.PostSystemDM en deps.go). Callback inyectado para no acoplar
	// taskService→channelService, mismo patrón que ChannelService.SetBroadcaster.
	// Puede quedar sin cablear (nil): en ese caso no se envían DMs.
	SetSystemDM(fn func(recipientID uint, content string))
}

type taskService struct {
	repo      repository.TaskRepository
	userRepo  repository.UserRepository
	boardRepo repository.BoardRepository
	notifSvc  NotificationService
	// postSystemDM publica un DM del bot "Obertrack" en el chat interno. Inyectado
	// por SetSystemDM; nil = sin DMs (p. ej. en tests que no lo cablean).
	postSystemDM func(recipientID uint, content string)
}

func (s *taskService) SetSystemDM(fn func(recipientID uint, content string)) {
	s.postSystemDM = fn
}

// sendSystemDM envía un DM del bot si el emisor está cableado. Best-effort: el
// aviso principal es la notificación de campanita.
func (s *taskService) sendSystemDM(recipientID uint, content string) {
	if s.postSystemDM != nil {
		s.postSystemDM(recipientID, content)
	}
}

// taskDueSuffix devuelve " · vence DD/MM/AAAA" si la tarea tiene fecha límite, o
// "" si no. Se anexa al DM de asignación para dar contexto sin otra línea.
func taskDueSuffix(t *models.Task) string {
	if t != nil && t.EndDate != nil {
		return " · vence " + t.EndDate.Format("02/01/2006")
	}
	return ""
}

func (s *taskService) authorizeBoardTenant(boardID, tenantID uint, isSuperadmin bool) error {
	if isSuperadmin {
		return nil
	}

	board, err := s.boardRepo.GetByID(boardID)
	if err != nil {
		return errors.New("El tablero especificado no existe o fue eliminado")
	}

	if board.CreatedBy == tenantID {
		return nil
	}
	if board.Creator.EmpleadorID != nil && *board.Creator.EmpleadorID == tenantID {
		return nil
	}

	return errors.New("No tienes permiso para acceder a ese tablero")
}

func (s *taskService) authorizeTaskTenant(task *models.Task, tenantID uint, isSuperadmin bool) error {
	if isSuperadmin {
		return nil
	}

	board, err := s.boardRepo.GetByID(task.BoardID)
	if err != nil {
		return errors.New("Tarea no encontrada")
	}

	if board.CreatedBy == tenantID {
		return nil
	}
	if board.Creator.EmpleadorID != nil && *board.Creator.EmpleadorID == tenantID {
		return nil
	}

	return errors.New("No tienes permiso para acceder a esta tarea")
}

func (s *taskService) canModifyTask(task *models.Task, userID uint, role string, isManager bool) bool {
	if isEmployerRole(role) || isManager {
		return true
	}
	if task.CreatedBy == userID {
		return true
	}
	for _, a := range task.Assignees {
		if a.ID == userID {
			return true
		}
	}
	return false
}

func (s *taskService) authorizeTaskByID(id, tenantID uint, isSuperadmin bool) (*models.Task, error) {
	var task *models.Task
	var err error
	if isSuperadmin || tenantID == 0 {
		task, err = s.repo.GetByID(id)
	} else {
		task, err = s.repo.GetByIDAndTenant(id, tenantID)
	}
	if err != nil {
		return nil, errors.New("Tarea no encontrada")
	}

	if err := s.authorizeTaskTenant(task, tenantID, isSuperadmin); err != nil {
		return nil, err
	}

	return task, nil
}

func NewTaskService(
	repo repository.TaskRepository,
	userRepo repository.UserRepository,
	boardRepo repository.BoardRepository,
	notifSvc NotificationService,
) TaskService {
	return &taskService{
		repo:      repo,
		userRepo:  userRepo,
		boardRepo: boardRepo,
		notifSvc:  notifSvc,
	}
}

func (s *taskService) GetAll(userID uint, role string, isManager, isSuperadmin bool, tenantID, companyFilter uint, boardIDStr, status, priority, assigneeIDStr, startDate, endDate string, offset, limit int) ([]models.Task, int64, error) {
	filters := make(map[string]interface{})

	if boardIDStr != "" && boardIDStr != "all" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err == nil {
			filters["board_id"] = uint(boardID)
		}
	}

	if assigneeIDStr != "" && assigneeIDStr != "all" {
		assigneeID, err := strconv.ParseUint(assigneeIDStr, 10, 32)
		if err == nil {
			filters["assignee_id"] = uint(assigneeID)
		}
	}

	if startDate != "" {
		filters["start_date"] = startDate
	}
	if endDate != "" {
		filters["end_date"] = endDate
	}

	if isSuperadmin {
		// Superadmin must scope to a company explicitly. Without it, no tasks are
		// returned so we never mix tasks from different tenants in the same view.
		if companyFilter == 0 {
			return []models.Task{}, 0, nil
		}
		filters["tenant_id"] = companyFilter
	} else if tenantID > 0 {
		filters["tenant_id"] = tenantID
		// Empresas y managers supervisan al equipo: ven todas las tareas del
		// tenant. Un profesional regular solo ve las tareas de los tableros a
		// los que pertenece (igual que la lista de tableros, que es por
		// membresía); así no aparecen en su dashboard tareas inaccesibles.
		if !isManager && role != string(models.UserTypeEmployer) {
			filters["member_board_user_id"] = userID
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

func (s *taskService) GetBoardStatusCounts(isSuperadmin bool, tenantID, companyFilter uint) (map[uint]map[string]int, error) {
	var scope uint
	if isSuperadmin {
		// Superadmin must scope to a company; without it return nothing.
		if companyFilter == 0 {
			return map[uint]map[string]int{}, nil
		}
		scope = companyFilter
	} else {
		scope = tenantID
	}

	rows, err := s.repo.CountByBoardAndStatus(scope)
	if err != nil {
		return nil, err
	}

	result := make(map[uint]map[string]int)
	for _, r := range rows {
		if result[r.BoardID] == nil {
			result[r.BoardID] = make(map[string]int)
		}
		result[r.BoardID][r.Status] = int(r.Count)
	}
	return result, nil
}

func (s *taskService) GetByID(id uint, tenantID uint, isSuperadmin bool) (*models.Task, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, err
	}
	return task, nil
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

func (s *taskService) Create(userID uint, isSuperadmin bool, tenantID uint, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error) {

	if title == "" {
		return nil, nil, errors.New("Title is required")
	}

	if boardID == 0 {
		return nil, nil, errors.New("Debes seleccionar un tablero para crear una tarea")
	}

	if boardID > 0 {
		if err := s.authorizeBoardTenant(boardID, tenantID, isSuperadmin); err != nil {
			return nil, nil, err
		}
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

	var boardTenant uint
	if b, err := s.boardRepo.GetByID(boardID); err == nil {
		boardTenant = b.TenantID
	}

	task := &models.Task{
		Title:       utils.SanitizeHTML(title),
		Description: utils.SanitizeHTML(description),
		Status:      models.TaskStatusTodo,
		Priority:    models.PriorityMedium,
		CreatedBy:   userID,
		BoardID:     boardID,
		TenantID:    boardTenant,
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
			"link":     "/tasks",
		})
		if err != nil {
			log.Printf("[TaskService] Error creating internal notification for user %d: %v", assignee.ID, err)
		}

		s.sendSystemDM(assignee.ID, fmt.Sprintf("📋 Se te asignó la tarea: %s%s", task.Title, taskDueSuffix(finalTask)))
	}

	// Notify employer/company
	creator, _ := s.userRepo.GetByID(userID)
	board, _ := s.boardRepo.GetByID(task.BoardID)
	employerIDs := make(map[uint]bool)

	// If the task creator is a professional and has an employer
	if creator != nil && creator.UserType == models.UserTypeProfessional && creator.EmpleadorID != nil {
		employerIDs[*creator.EmpleadorID] = true
	}

	// Also, if the board creator is an employer and not the task creator
	if board != nil && board.CreatedBy != userID {
		boardCreator, _ := s.userRepo.GetByID(board.CreatedBy)
		if boardCreator != nil && (boardCreator.UserType == models.UserTypeEmployer || boardCreator.IsManager) {
			employerIDs[board.CreatedBy] = true
		}
	}

	for empID := range employerIDs {
		if empID == userID {
			continue
		}
		
		creatorName := "Alguien"
		if creator != nil {
			creatorName = creator.Name
		}
		
		err := s.notifSvc.CreateNotification(empID, "task_created", "Nueva tarea creada", fmt.Sprintf("%s creó la tarea: %s", creatorName, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     "/tasks",
		})
		if err != nil {
			log.Printf("[TaskService] Error creating task_created notification for employer %d: %v", empID, err)
		}
	}

	return finalTask, finalTask.Assignees, nil
}

func (s *taskService) Update(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}, assignees *[]uint) (*models.Task, []models.User, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, nil, err
	}

	if !isSuperadmin && !s.canModifyTask(task, updaterUserID, role, isManager) {
		return nil, nil, errors.New("Access denied")
	}

	// Keep track of assignees before update (to detect who is new vs existing)
	currentAssigneeIDs := make(map[uint]bool)
	for _, a := range task.Assignees {
		currentAssigneeIDs[a.ID] = true
	}

	// Fecha límite ANTES del update. El frontend reenvía end_date en CADA edición
	// (aunque solo cambies la prioridad), así que la presencia de la clave en
	// reqData no significa que la fecha cambió: hay que comparar el valor real.
	var oldEndDate string
	if task.EndDate != nil {
		oldEndDate = task.EndDate.Format("2006-01-02")
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

		s.repo.SyncAssignees(task, *assignees)
	}

	// Fetch final refreshed task with preloads (assignees, board, creator)
	finalTask, err := s.repo.GetByID(task.ID)
	if err != nil {
		log.Printf("[TaskService] Error refreshing task %d for update notifications: %v", task.ID, err)
		return task, task.Assignees, nil
	}
	task = finalTask

	// ¿Cambió la fecha límite DE VERDAD? Comparamos el valor viejo con el nuevo
	// (recargado en task/finalTask), no la mera presencia de end_date en reqData.
	// Sirve para avisar por DM solo a los asignados que ya estaban (los nuevos
	// reciben el DM de asignación, más completo).
	var newEndDate string
	if task.EndDate != nil {
		newEndDate = task.EndDate.Format("2006-01-02")
	}
	deadlineChanged := oldEndDate != newEndDate

	// Notify new assignees (only if assignees changed)
	if assignees != nil {
		for _, u := range task.Assignees {
			if !currentAssigneeIDs[u.ID] {
				err := s.notifSvc.CreateNotification(u.ID, "task_assigned", "Nueva tarea asignada", fmt.Sprintf("Se te asignó la tarea: %s", task.Title), map[string]interface{}{
					"task_id":  task.ID,
					"board_id": task.BoardID,
					"link":     "/tasks",
				})
				if err != nil {
					log.Printf("[TaskService] Error creating internal notification for user %d: %v", u.ID, err)
				}

				s.sendSystemDM(u.ID, fmt.Sprintf("📋 Se te asignó la tarea: %s%s", task.Title, taskDueSuffix(task)))
			}
		}
	}

	// Now handle modification notifications for employer and other assignees
	updaterName := "Alguien"
	var updater *models.User
	if updaterUserID > 0 {
		updater, _ = s.userRepo.GetByID(updaterUserID)
		if updater != nil {
			updaterName = updater.Name
		}
	}

	// Employer IDs to notify
	board, _ := s.boardRepo.GetByID(task.BoardID)
	employerIDs := make(map[uint]bool)

	if updater != nil && updater.UserType == models.UserTypeProfessional && updater.EmpleadorID != nil {
		employerIDs[*updater.EmpleadorID] = true
	}

	if board != nil && board.CreatedBy != updaterUserID {
		boardCreator, _ := s.userRepo.GetByID(board.CreatedBy)
		if boardCreator != nil && (boardCreator.UserType == models.UserTypeEmployer || boardCreator.IsManager) {
			employerIDs[board.CreatedBy] = true
		}
	}

	for empID := range employerIDs {
		if empID == updaterUserID {
			continue
		}

		err := s.notifSvc.CreateNotification(empID, "task_updated", "Tarea modificada", fmt.Sprintf("%s modificó la tarea: %s", updaterName, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     "/tasks",
		})
		if err != nil {
			log.Printf("[TaskService] Error creating task_updated notification for employer %d: %v", empID, err)
		}

	}

	// Notify other assignees who were already assigned or whose assignment is preserved
	for _, assignee := range task.Assignees {
		if assignee.ID == updaterUserID {
			continue
		}
		if assignees != nil && !currentAssigneeIDs[assignee.ID] {
			// Skip newly assigned users, since they already got task_assigned
			continue
		}

		err := s.notifSvc.CreateNotification(assignee.ID, "task_updated", "Tarea modificada", fmt.Sprintf("%s modificó la tarea: %s", updaterName, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     "/tasks",
		})
		if err != nil {
			log.Printf("[TaskService] Error creating task_updated notification for assignee %d: %v", assignee.ID, err)
		}

		if deadlineChanged {
			var dueMsg string
			if task.EndDate != nil {
				dueMsg = fmt.Sprintf("📅 Cambió la fecha de \"%s\": ahora vence %s", task.Title, task.EndDate.Format("02/01/2006"))
			} else {
				dueMsg = fmt.Sprintf("📅 La tarea \"%s\" ya no tiene fecha límite", task.Title)
			}
			s.sendSystemDM(assignee.ID, dueMsg)
		}
	}

	return task, task.Assignees, nil
}

func (s *taskService) Delete(id uint, tenantID uint, userID uint, role string, isManager, isSuperadmin bool) error {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return err
	}
	if !isSuperadmin && !s.canModifyTask(task, userID, role, isManager) {
		return errors.New("Access denied")
	}
	// Delete related notifications
	_ = s.notifSvc.DeleteByTaskID(id)
	return s.repo.Delete(id)
}

func (s *taskService) ToggleCompletion(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool) (*models.Task, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, err
	}

	if !isSuperadmin && !s.canModifyTask(task, updaterUserID, role, isManager) {
		return nil, errors.New("Access denied")
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

	// Fetch final refreshed task with preloads
	finalTask, err := s.repo.GetByID(id)
	if err != nil {
		return task, nil
	}
	task = finalTask

	updaterName := "Alguien"
	var updater *models.User
	if updaterUserID > 0 {
		updater, _ = s.userRepo.GetByID(updaterUserID)
		if updater != nil {
			updaterName = updater.Name
		}
	}

	actionVerb := "reabrió"
	notifType := "task_updated"
	title := "Tarea reabierta"
	if completed {
		actionVerb = "completó"
		notifType = "task_completed"
		title = "Tarea completada"
	}

	// Employer IDs to notify
	board, _ := s.boardRepo.GetByID(task.BoardID)
	employerIDs := make(map[uint]bool)

	if updater != nil && updater.UserType == models.UserTypeProfessional && updater.EmpleadorID != nil {
		employerIDs[*updater.EmpleadorID] = true
	}

	if board != nil && board.CreatedBy != updaterUserID {
		boardCreator, _ := s.userRepo.GetByID(board.CreatedBy)
		if boardCreator != nil && (boardCreator.UserType == models.UserTypeEmployer || boardCreator.IsManager) {
			employerIDs[board.CreatedBy] = true
		}
	}

	for empID := range employerIDs {
		if empID == updaterUserID {
			continue
		}

		err := s.notifSvc.CreateNotification(empID, notifType, title, fmt.Sprintf("%s %s la tarea: %s", updaterName, actionVerb, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     "/tasks",
		})
		if err != nil {
			log.Printf("[TaskService] Error creating ToggleCompletion notification for employer %d: %v", empID, err)
		}

	}

	// Notify other assignees
	for _, assignee := range task.Assignees {
		if assignee.ID == updaterUserID {
			continue
		}

		err := s.notifSvc.CreateNotification(assignee.ID, notifType, title, fmt.Sprintf("%s %s la tarea: %s", updaterName, actionVerb, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     "/tasks",
		})
		if err != nil {
			log.Printf("[TaskService] Error creating ToggleCompletion notification for assignee %d: %v", assignee.ID, err)
		}

		// Solo al completar (no al reabrir): el DM de "✅ completada" es señal de
		// cierre; reabrir es un cambio menor que no amerita un mensaje.
		if completed {
			s.sendSystemDM(assignee.ID, fmt.Sprintf("✅ Se completó la tarea: %s", task.Title))
		}
	}

	return task, nil
}
