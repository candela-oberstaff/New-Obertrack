package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

type BoardHandler struct {
	db *gorm.DB
}

func NewBoardHandler(db *gorm.DB) *BoardHandler {
	return &BoardHandler{db: db}
}

type CreateBoardRequest struct {
	Name        string             `json:"name" binding:"required"`
	Description string             `json:"description"`
	Color       string             `json:"color"`
	MemberIDs   []uint             `json:"member_ids"`
	Phases      []CreatePhaseInput `json:"phases"`
}

type CreatePhaseInput struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

type UpdateBoardRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	MemberIDs   []uint `json:"member_ids"`
}

func (h *BoardHandler) GetAll(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	log.Printf("[GetAll] userID=%d, role=%s, isSuperadmin=%v", userID, role, isSuperadmin)

	var boards []models.Board

	if isSuperadmin || role == "superadmin" {
		h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).Find(&boards)
	} else {
		var boardIDs []uint
		h.db.Model(&models.Board{}).
			Select("boards.id").
			Joins("LEFT JOIN board_members ON board_members.board_id = boards.id").
			Where("board_members.user_id = ? OR boards.created_by = ?", userID, userID).
			Group("boards.id").
			Pluck("boards.id", &boardIDs)

		log.Printf("[GetAll] Found %d unique board IDs: %v", len(boardIDs), boardIDs)

		if len(boardIDs) > 0 {
			h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
				return db.Order("\"order\" ASC")
			}).Where("boards.id IN ?", boardIDs).Find(&boards)
		}
	}

	log.Printf("[GetAll] Found %d boards", len(boards))
	for _, b := range boards {
		log.Printf("[GetAll]   Board: ID=%d, Name=%s, CreatedBy=%d", b.ID, b.Name, b.CreatedBy)
	}

	c.JSON(http.StatusOK, boards)
}

func (h *BoardHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	var board models.Board
	if err := h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC")
	}).First(&board, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
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
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	c.JSON(http.StatusOK, board)
}

func (h *BoardHandler) Create(c *gin.Context) {
	var req CreateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	log.Printf("[CreateBoard] userID=%d, req=%+v", userID, req)

	board := models.Board{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		CreatedBy:   userID,
	}

	if board.Color == "" {
		board.Color = "#3b82f6"
	}

	if err := h.db.Create(&board).Error; err != nil {
		log.Printf("Error creating board: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create board"})
		return
	}
	log.Printf("[CreateBoard] Board created with ID=%d", board.ID)

	if len(req.MemberIDs) > 0 {
		var members []models.User
		h.db.Find(&members, req.MemberIDs)
		h.db.Model(&board).Association("Members").Append(members)
	}

	// Add creator as a member automatically using BoardMember
	boardMember := models.BoardMember{
		BoardID: board.ID,
		UserID:  userID,
	}
	h.db.Create(&boardMember)
	log.Printf("[CreateBoard] Added creator as member: boardID=%d, userID=%d", board.ID, userID)

	// Add phases - use provided phases or defaults
	phasesToCreate := req.Phases
	if len(phasesToCreate) == 0 {
		phasesToCreate = []CreatePhaseInput{
			{Name: "Por hacer", Color: "#6b7280"},
			{Name: "En proceso", Color: "#3b82f6"},
			{Name: "Finalizado", Color: "#22c55e"},
		}
	}

	statusNames := []string{"por_hacer", "en_proceso", "finalizado", "", "", ""} // Default statuses for first 3

	for i, p := range phasesToCreate {
		color := p.Color
		if color == "" {
			color = "#6b7280"
		}
		status := ""
		if i < len(statusNames) {
			status = statusNames[i]
		}
		phase := models.Phase{
			Name:   p.Name,
			Color:  color,
			Status: status,
			Order:  i,
		}
		h.db.Create(&phase)
		boardPhase := models.BoardPhase{
			BoardID: board.ID,
			PhaseID: phase.ID,
		}
		h.db.Create(&boardPhase)
	}
	log.Printf("[CreateBoard] Added %d phases to board: boardID=%d", len(phasesToCreate), board.ID)

	// Verify members
	var members []models.User
	h.db.Model(&board).Association("Members").Find(&members)
	log.Printf("[CreateBoard] Board members count: %d", len(members))
	for _, m := range members {
		log.Printf("[CreateBoard]   - Member: ID=%d, Name=%s", m.ID, m.Name)
	}

	h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC")
	}).First(&board, board.ID)

	c.JSON(http.StatusCreated, board)
}

func (h *BoardHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	var board models.Board
	if err := h.db.First(&board, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	var req UpdateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}

	if len(updates) > 0 {
		h.db.Model(&board).Updates(updates)
	}

	if len(req.MemberIDs) > 0 {
		var members []models.User
		h.db.Find(&members, req.MemberIDs)
		h.db.Model(&board).Association("Members").Replace(members)
	}

	h.db.Preload("Members").Preload("Creator").First(&board, id)

	c.JSON(http.StatusOK, board)
}

func (h *BoardHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	userID := middleware.GetUserID(c)

	var board models.Board
	if err := h.db.First(&board, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	if board.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo el creador puede eliminar el tablero"})
		return
	}

	// Delete associated phases first
	h.db.Unscoped().Where("board_id = ?", id).Delete(&models.Phase{})

	// Delete board members
	h.db.Unscoped().Where("board_id = ?", id).Delete(&models.BoardMember{})

	// Get task IDs for this board to delete associated records
	var taskIDs []uint
	h.db.Unscoped().Model(&models.Task{}).Where("board_id = ?", id).Pluck("id", &taskIDs)

	// Delete associated comments and attachments for each task
	if len(taskIDs) > 0 {
		h.db.Unscoped().Where("task_id IN ?", taskIDs).Delete(&models.Comment{})
		h.db.Unscoped().Where("task_id IN ?", taskIDs).Delete(&models.TaskAttachment{})
		// Delete task-user associations (many2many)
		h.db.Unscoped().Where("task_id IN ?", taskIDs).Delete(&models.TaskUser{})
	}

	// Delete associated tasks
	h.db.Unscoped().Where("board_id = ?", id).Delete(&models.Task{})

	// Delete the board
	if err := h.db.Delete(&board).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete board"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Board deleted successfully"})
}

type AddPhaseRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

func (h *BoardHandler) AddPhase(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	var req AddPhaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var board models.Board
	if err := h.db.First(&board, boardID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	// Get current max order
	var maxOrder int
	h.db.Model(&models.Phase{}).
		Joins("JOIN board_phases ON board_phases.phase_id = phases.id").
		Where("board_phases.board_id = ?", boardID).
		Select("COALESCE(MAX(phases.\"order\"), -1)").Scan(&maxOrder)

	color := req.Color
	if color == "" {
		color = "#6b7280"
	}

	phase := models.Phase{
		Name:  req.Name,
		Color: color,
		Order: maxOrder + 1,
	}

	if err := h.db.Create(&phase).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create phase"})
		return
	}

	boardPhase := models.BoardPhase{
		BoardID: board.ID,
		PhaseID: phase.ID,
	}
	h.db.Create(&boardPhase)

	h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC")
	}).First(&board, board.ID)

	c.JSON(http.StatusCreated, board)
}

func (h *BoardHandler) RemovePhase(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	phaseID, err := strconv.ParseUint(c.Param("phaseId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid phase ID"})
		return
	}

	var board models.Board
	if err := h.db.First(&board, boardID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	// Check if phase exists on this board
	var count int64
	h.db.Model(&models.BoardPhase{}).Where("board_id = ? AND phase_id = ?", boardID, phaseID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Phase not found on this board"})
		return
	}

	// Remove the board-phase association
	h.db.Where("board_id = ? AND phase_id = ?", boardID, phaseID).Delete(&models.BoardPhase{})

	// Check if phase is used by any tasks on this board
	var taskCount int64
	h.db.Model(&models.Task{}).Where("board_id = ? AND status = ?", boardID, getPhaseStatusName(uint(phaseID))).Count(&taskCount)
	if taskCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove phase with tasks. Move or delete tasks first."})
		return
	}

	// Optionally delete the phase if not used by other boards
	var otherBoards int64
	h.db.Model(&models.BoardPhase{}).Where("phase_id = ? AND board_id != ?", phaseID, boardID).Count(&otherBoards)
	if otherBoards == 0 {
		h.db.Delete(&models.Phase{}, phaseID)
	}

	h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC")
	}).First(&board, board.ID)

	c.JSON(http.StatusOK, board)
}

type ReorderPhasesRequest struct {
	PhaseIDs []uint `json:"phase_ids" binding:"required"`
}

func (h *BoardHandler) ReorderPhases(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	var req ReorderPhasesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var board models.Board
	if err := h.db.First(&board, boardID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	// Update the order of each phase
	for i, phaseID := range req.PhaseIDs {
		h.db.Model(&models.Phase{}).Where("id = ?", phaseID).Update("order", i)
	}

	h.db.Preload("Members").Preload("Creator").Preload("Phases", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC")
	}).First(&board, board.ID)

	c.JSON(http.StatusOK, board)
}

func getPhaseStatusName(phaseID uint) string {
	// This is a simple mapping - in a real app you'd have a proper relationship
	names := map[uint]string{
		1: "por_hacer",
		2: "en_proceso",
		3: "finalizado",
	}
	return names[phaseID]
}
