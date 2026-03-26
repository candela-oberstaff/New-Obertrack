package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/utils"
)

type TaskHandler struct {
	db *gorm.DB
}

func NewTaskHandler(db *gorm.DB) *TaskHandler {
	return &TaskHandler{db: db}
}

type CreateTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	EndDate     *string `json:"end_date"`
	Assignees   []uint  `json:"assignees"`
	BoardID     uint    `json:"board_id"`
}

type UpdateTaskRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	EndDate     time.Time `json:"end_date"`
	Completed   *bool     `json:"completed"`
	Assignees   *[]uint   `json:"assignees"`
}

func (h *TaskHandler) GetAll(c *gin.Context) {
	var tasks []models.Task
	query := h.db.Model(&models.Task{})

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := middleware.GetEmpleadorID(c)

	boardIDStr := c.Query("board_id")
	log.Printf("[GetAll] userID=%d, role=%s, boardID=%s, isManager=%v, isSuperadmin=%v", userID, role, boardIDStr, isManager, isSuperadmin)

	if boardIDStr != "" && boardIDStr != "all" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err == nil {
			query = query.Where("board_id = ?", boardID)
		}
	} else {
		// When not filtering by specific board, only show tasks from non-deleted boards
		query = query.Where("board_id IN (?)", h.db.Model(&models.Board{}).Select("id"))
	}

	if isSuperadmin {
		// Superadmin ve todo
	} else if role == string(models.UserTypeProfessional) {
		// Profesional ve tareas donde es creador o asignado
		var assignedTaskIDs []uint
		h.db.Table("task_users").Where("user_id = ?", userID).Pluck("task_id", &assignedTaskIDs)
		if len(assignedTaskIDs) > 0 {
			query = query.Where("(created_by = ? OR id IN (?))", userID, assignedTaskIDs)
		} else {
			query = query.Where("created_by = ?", userID)
		}
	} else if role == string(models.UserTypeEmployer) || role == "empleador" {
		// Empleador ve las tareas de sus empleados
		if empleadorID > 0 {
			subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
			query = query.Where("created_by IN (?)", subquery)
		}
	} else if !isManager {
		// Otros usuarios ven solo sus propias tareas
		query = query.Where("created_by = ?", userID)
	}

	status := c.Query("status")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	priority := c.Query("priority")
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	var total int64
	query.Count(&total)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	if err := query.Preload("Creator").Preload("Assignees").Preload("Board").Preload("Attachments").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		log.Printf("[GetAll] Error fetching tasks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks", "details": err.Error()})
		return
	}

	log.Printf("[GetAll] Returning %d tasks for userID=%d, boardID=%s", len(tasks), userID, boardIDStr)
	for i, t := range tasks {
		log.Printf("  Task[%d]: id=%d, title=%s, board_id=%d, created_by=%d", i, t.ID, t.Title, t.BoardID, t.CreatedBy)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  tasks,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *TaskHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var task models.Task
	if err := h.db.Preload("Creator").Preload("Assignees").Preload("Comments").Preload("Comments.User").Preload("Attachments").First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Creating task: title=%s, description=%s, priority=%s, end_date=%v, assignees=%v",
		req.Title, req.Description, req.Priority, req.EndDate, req.Assignees)

	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}

	userID := middleware.GetUserID(c)

	if len(req.Assignees) > 0 && req.BoardID > 0 {
		var board models.Board
		h.db.First(&board, req.BoardID)

		var boardMembers []models.BoardMember
		h.db.Where("board_id = ?", req.BoardID).Find(&boardMembers)
		memberIDs := make(map[uint]bool)
		for _, m := range boardMembers {
			memberIDs[m.UserID] = true
		}
		// Also include the creator as a valid member
		if board.CreatedBy != 0 {
			memberIDs[board.CreatedBy] = true
		}

		for _, assigneeID := range req.Assignees {
			if !memberIDs[assigneeID] {
				var assigneeUser models.User
				userName := fmt.Sprintf("ID %d", assigneeID)
				if h.db.First(&assigneeUser, assigneeID).Error == nil {
					userName = assigneeUser.Name
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s no es miembro del tablero", userName)})
				return
			}
		}
	}

	task := models.Task{
		Title:       utils.SanitizeHTML(req.Title),
		Description: utils.SanitizeHTML(req.Description),
		Status:      models.TaskStatusTodo,
		Priority:    models.PriorityMedium,
		CreatedBy:   userID,
		BoardID:     req.BoardID,
	}

	if req.Priority != "" {
		task.Priority = models.TaskPriority(req.Priority)
	}

	if req.EndDate != nil && *req.EndDate != "" {
		endDate, err := time.Parse("2006-01-02", *req.EndDate)
		if err == nil {
			task.EndDate = &endDate
		}
	}

	if err := h.db.Create(&task).Error; err != nil {
		log.Printf("Error creating task: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task: " + err.Error()})
		return
	}

	if len(req.Assignees) > 0 {
		var assignees []models.User
		h.db.Find(&assignees, req.Assignees)
		h.db.Model(&task).Association("Assignees").Append(assignees)

		for _, assignee := range assignees {
			notificationData, _ := json.Marshal(map[string]interface{}{
				"task_id":  task.ID,
				"board_id": task.BoardID,
			})
			notification := models.Notification{
				UserID:  assignee.ID,
				Type:    "task_assigned",
				Title:   "Nueva tarea asignada",
				Message: fmt.Sprintf("Se te asignó la tarea: %s", task.Title),
				Data:    string(notificationData),
			}
			h.db.Create(&notification)
			NotifyUser(assignee.ID, "task_assigned", notification)
		}
	}

	h.db.Preload("Creator").Preload("Assignees").First(&task, task.ID)

	log.Printf("[Create] Task created: id=%d, title=%s, board_id=%d, created_by=%d, assignees=%v",
		task.ID, task.Title, task.BoardID, task.CreatedBy, task.Assignees)

	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var task models.Task
	if err := h.db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON for update: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Update task %d: status=%s, priority=%s", id, req.Status, req.Priority)

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = utils.SanitizeHTML(req.Title)
	}
	if req.Description != "" {
		updates["description"] = utils.SanitizeHTML(req.Description)
	}
	if req.Status != "" {
		updates["status"] = req.Status
		log.Printf("[Update] Setting status to: '%s'", req.Status)
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Completed != nil {
		updates["completed"] = *req.Completed
	}
	if !req.EndDate.IsZero() {
		updates["end_date"] = req.EndDate
	}

	if len(updates) == 0 {
		log.Printf("No updates to apply")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	log.Printf("Applying updates to task ID %d: %+v", task.ID, updates)

	result := h.db.Model(&task).Updates(updates)
	log.Printf("Rows affected: %d, Error: %v", result.RowsAffected, result.Error)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	if req.Assignees != nil {
		var board models.Board
		h.db.First(&board, task.BoardID)

		var boardMembers []models.BoardMember
		h.db.Where("board_id = ?", task.BoardID).Find(&boardMembers)
		memberIDs := make(map[uint]bool)
		for _, m := range boardMembers {
			memberIDs[m.UserID] = true
		}
		// Also include the creator as a valid member
		if board.CreatedBy != 0 {
			memberIDs[board.CreatedBy] = true
		}

		for _, assigneeID := range *req.Assignees {
			if !memberIDs[assigneeID] {
				var assigneeUser models.User
				userName := fmt.Sprintf("ID %d", assigneeID)
				if h.db.First(&assigneeUser, assigneeID).Error == nil {
					userName = assigneeUser.Name
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s no es miembro del tablero", userName)})
				return
			}
		}

		var currentAssignees []models.User
		h.db.Model(&task).Association("Assignees").Find(&currentAssignees)
		currentAssigneeIDs := make(map[uint]bool)
		for _, a := range currentAssignees {
			currentAssigneeIDs[a.ID] = true
		}

		if len(*req.Assignees) == 0 {
			h.db.Model(&task).Association("Assignees").Clear()
		} else {
			var assignees []models.User
			h.db.Find(&assignees, *req.Assignees)
			h.db.Model(&task).Association("Assignees").Replace(assignees)

			for _, assignee := range assignees {
				if !currentAssigneeIDs[assignee.ID] {
					notificationData, _ := json.Marshal(map[string]interface{}{
						"task_id":  task.ID,
						"board_id": task.BoardID,
					})
					notification := models.Notification{
						UserID:  assignee.ID,
						Type:    "task_assigned",
						Title:   "Nueva tarea asignada",
						Message: fmt.Sprintf("Se te asignó la tarea: %s", task.Title),
						Data:    string(notificationData),
					}
					h.db.Create(&notification)
				}
			}
		}
	}

	h.db.Preload("Creator").Preload("Assignees").First(&task, task.ID)
	log.Printf("[Update] Task %d updated successfully. New status: '%s'", task.ID, task.Status)

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	if err := h.db.Delete(&models.Task{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task deleted successfully"})
}

func (h *TaskHandler) ToggleCompletion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var task models.Task
	if err := h.db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	completed := !task.Completed
	status := models.TaskStatusTodo
	if completed {
		status = models.TaskStatusDone
	}

	if err := h.db.Model(&task).Updates(map[string]interface{}{
		"completed": completed,
		"status":    status,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) AddComment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var task models.Task
	if err := h.db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	comment := models.Comment{
		TaskID:  uint(id),
		UserID:  userID,
		Content: utils.SanitizeHTML(req.Content),
	}

	if err := h.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add comment"})
		return
	}

	h.db.Preload("User").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, comment)
}

func (h *TaskHandler) AddAttachment(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var task models.Task
	if err := h.db.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	const maxSize = 50 << 20
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 50MB)"})
		return
	}

	uploadPath := os.Getenv("UPLOAD_PATH")
	if uploadPath == "" {
		uploadPath = "./uploads"
	}

	contentType := file.Header.Get("Content-Type")
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".bin"
	}

	userID := middleware.GetUserID(c)
	filename := fmt.Sprintf("task_%d_%d_%d%s", taskID, userID, time.Now().UnixNano(), ext)
	filepath_ := filepath.Join(uploadPath, filename)

	if err := c.SaveUploadedFile(file, filepath_); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	attachment := models.TaskAttachment{
		TaskID:     uint(taskID),
		FileName:   file.Filename,
		FileURL:    fmt.Sprintf("/api/uploads/%s", filename),
		FileSize:   file.Size,
		MimeType:   contentType,
		UploadedBy: userID,
	}

	if err := h.db.Create(&attachment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save attachment"})
		return
	}

	c.JSON(http.StatusCreated, attachment)
}

func (h *TaskHandler) DeleteAttachment(c *gin.Context) {
	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	var attachment models.TaskAttachment
	if err := h.db.First(&attachment, attachmentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}

	if err := h.db.Delete(&attachment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attachment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted"})
}
