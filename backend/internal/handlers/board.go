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
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Color       string `json:"color"`
	MemberIDs   []uint `json:"member_ids"`
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

	var boards []models.Board
	query := h.db.Model(&models.Board{})

	if isSuperadmin || role == "superadmin" {
		query.Preload("Members").Preload("Creator").Find(&boards)
	} else {
		query.Joins("JOIN board_members ON board_members.board_id = boards.id").
			Where("board_members.user_id = ?", userID).
			Or("boards.created_by = ?", userID).
			Preload("Members").
			Preload("Creator").
			Find(&boards)
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
	if err := h.db.Preload("Members").Preload("Creator").First(&board, id).Error; err != nil {
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

	if len(req.MemberIDs) > 0 {
		var members []models.User
		h.db.Find(&members, req.MemberIDs)
		h.db.Model(&board).Association("Members").Append(members)
	}

	h.db.Preload("Members").Preload("Creator").First(&board, board.ID)

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

	var board models.Board
	if err := h.db.First(&board, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	if err := h.db.Delete(&board).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete board"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Board deleted successfully"})
}
