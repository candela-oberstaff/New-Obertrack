package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type TaskHandler struct {
	service service.TaskService
}

func NewTaskHandler(service service.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
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
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Status      *string    `json:"status"`
	Priority    *string    `json:"priority"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	Completed   *bool      `json:"completed"`
	Assignees   *[]uint    `json:"assignees"`
}

func (h *TaskHandler) GetAll(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	tenantID := middleware.GetTenantID(c)

	boardIDStr := c.Query("board_id")
	status := c.Query("status")
	priority := c.Query("priority")
	assigneeIDStr := c.Query("assignee_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// Superadmin scopes to a company via ?company_id=. Ignored for tenant-scoped users.
	var companyFilter uint
	if isSuperadmin {
		if v, err := strconv.ParseUint(c.Query("company_id"), 10, 32); err == nil {
			companyFilter = uint(v)
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	tasks, total, err := h.service.GetAll(userID, role, isManager, isSuperadmin, tenantID, companyFilter, boardIDStr, status, priority, assigneeIDStr, startDate, endDate, offset, limit)
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

	tenantID := middleware.GetTenantID(c)
	task, err := h.service.GetByID(uint(id), tenantID, middleware.IsSuperadmin(c))
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
	isSuperadmin := middleware.IsSuperadmin(c)
	tenantID := middleware.GetTenantID(c)

	task, _, err := h.service.Create(userID, isSuperadmin, tenantID, req.Title, req.Description, req.Priority, req.EndDate, req.Assignees, req.BoardID)
	if err != nil {
		fmt.Printf("[DEBUG] Create Task Error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

	// Maps req elements dynamically — use pointers so empty strings are included
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Completed != nil {
		updates["completed"] = *req.Completed
	}
	if req.StartDate != nil {
		updates["start_date"] = *req.StartDate
	}
	if req.EndDate != nil {
		updates["end_date"] = *req.EndDate
	}

	if len(updates) == 0 && req.Assignees == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	tenantID := middleware.GetTenantID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	task, _, err := h.service.Update(uint(id), tenantID, userID, role, isManager, isSuperadmin, updates, req.Assignees)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	if err := h.service.Delete(uint(id), tenantID, userID, role, isManager, middleware.IsSuperadmin(c)); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
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

	userID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	task, err := h.service.ToggleCompletion(uint(id), tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
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
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	comment, err := h.service.AddComment(uint(id), tenantID, userID, req.Content, isSuperadmin)
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
	tenantID := middleware.GetTenantID(c)

	task, err := h.service.GetByID(uint(taskID), tenantID, middleware.IsSuperadmin(c))
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
	isSuperadmin := middleware.IsSuperadmin(c)
	filename := fmt.Sprintf("task_%d_%d_%d%s", taskID, userID, time.Now().UnixNano(), ext)
	filepath_ := filepath.Join(uploadPath, filename)

	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		log.Printf("failed to create upload directory %q: %v", uploadPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare upload directory"})
		return
	}

	if err := c.SaveUploadedFile(file, filepath_); err != nil {
		log.Printf("failed to save task attachment %q: %v", filepath_, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Now handled by service
	attachment, err := h.service.AddAttachment(uint(taskID), tenantID, file.Filename, fmt.Sprintf("/api/uploads/%s", filename), file.Size, contentType, userID, isSuperadmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save attachment info"})
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

	tenantID := middleware.GetTenantID(c)
	if err := h.service.DeleteAttachment(uint(attachmentID), tenantID, middleware.IsSuperadmin(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attachment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted"})
}
