package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
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
}

type UpdateTaskRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	EndDate     time.Time `json:"end_date"`
	Completed   *bool     `json:"completed"`
	Assignees   []uint    `json:"assignees"`
}

func (h *TaskHandler) GetAll(c *gin.Context) {
	var tasks []models.Task
	query := h.db.Model(&models.Task{})

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := middleware.GetEmpleadorID(c)

	if !isSuperadmin && role == string(models.UserTypeEmployee) {
		query = query.Joins("JOIN task_users ON task_users.task_id = tasks.id").
			Where("task_users.user_id = ? OR tasks.created_by = ?", userID, userID)
	} else if !isSuperadmin && !isManager {
		query = query.Where("created_by = ?", userID)
	}

	if !isSuperadmin && empleadorID > 0 {
		subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
		query = query.Joins("LEFT JOIN task_users ON task_users.task_id = tasks.id").
			Where("tasks.created_by IN (?) OR task_users.user_id IN (?)", subquery, subquery).
			Distinct()
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

	if err := query.Preload("Creator").Preload("Assignees").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks"})
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

	var task models.Task
	if err := h.db.Preload("Creator").Preload("Assignees").Preload("Comments").Preload("Comments.User").First(&task, id).Error; err != nil {
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

	task := models.Task{
		Title:       req.Title,
		Description: req.Description,
		Status:      models.TaskStatusTodo,
		Priority:    models.PriorityMedium,
		CreatedBy:   userID,
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
	}

	h.db.Preload("Creator").Preload("Assignees").First(&task, task.ID)

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

	if len(updates) == 0 {
		log.Printf("No updates to apply")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	log.Printf("Applying updates: %+v", updates)

	if err := h.db.Model(&task).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	if len(req.Assignees) > 0 {
		var assignees []models.User
		h.db.Find(&assignees, req.Assignees)
		h.db.Model(&task).Association("Assignees").Replace(assignees)
	}

	h.db.Preload("Creator").Preload("Assignees").First(&task, task.ID)

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
		Content: req.Content,
	}

	if err := h.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add comment"})
		return
	}

	h.db.Preload("User").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, comment)
}
