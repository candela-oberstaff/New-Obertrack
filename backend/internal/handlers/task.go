package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TaskHandler struct {
	service service.TaskService
	db      *gorm.DB // Mantenemos la DB para inyectar notificaciones sin circular dependency con NotifyUser
}

func NewTaskHandler(service service.TaskService, db *gorm.DB) *TaskHandler {
	return &TaskHandler{service: service, db: db}
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
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := middleware.GetEmpleadorID(c)

	boardIDStr := c.Query("board_id")
	status := c.Query("status")
	priority := c.Query("priority")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	tasks, total, err := h.service.GetAll(userID, role, isManager, isSuperadmin, empleadorID, boardIDStr, status, priority, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks", "details": err.Error()})
		return
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

	task, err := h.service.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	task, usersToNotify, err := h.service.Create(userID, req.Title, req.Description, req.Priority, req.EndDate, req.Assignees, req.BoardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handles WebSockets + Notification Tracking independently
	for _, assignee := range usersToNotify {
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

	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Maps req elements dynamically
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
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

	if len(updates) == 0 && req.Assignees == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	task, usersToNotify, err := h.service.Update(uint(id), updates, req.Assignees)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, assignee := range usersToNotify {
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

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
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

	task, err := h.service.ToggleCompletion(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	comment, err := h.service.AddComment(uint(id), userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

func (h *TaskHandler) AddAttachment(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	task, err := h.service.GetByID(uint(taskID))
	if err != nil || task == nil {
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

	// This falls under simple CRUD, we can safely invoke DB here to keep simplicity
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
