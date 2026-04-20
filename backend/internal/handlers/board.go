package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type BoardHandler struct {
	service service.BoardService
}

func NewBoardHandler(s service.BoardService) *BoardHandler {
	return &BoardHandler{service: s}
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

	boards, err := h.service.GetAll(userID, role, isSuperadmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get boards"})
		return
	}

	c.JSON(http.StatusOK, boards)
}

func (h *BoardHandler) GetPublicBoards(c *gin.Context) {
	userID := middleware.GetUserID(c)

	boards, err := h.service.GetPublicBoards(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get public boards"})
		return
	}

	c.JSON(http.StatusOK, boards)
}

func (h *BoardHandler) JoinBoard(c *gin.Context) {
	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	userID := middleware.GetUserID(c)

	board, err := h.service.JoinBoard(userID, uint(boardID))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Board not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Ya eres miembro de este tablero" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, board)
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

	board, err := h.service.GetByID(userID, role, isSuperadmin, uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Board not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
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

	var phases []struct {
		Name  string
		Color string
	}
	for _, p := range req.Phases {
		phases = append(phases, struct{ Name, Color string }{p.Name, p.Color})
	}

	board, err := h.service.Create(userID, req.Name, req.Description, req.Color, req.MemberIDs, phases)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create board"})
		return
	}

	c.JSON(http.StatusCreated, board)
}

func (h *BoardHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
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

	board, err := h.service.Update(uint(id), updates, req.MemberIDs)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, board)
}

func (h *BoardHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	userID := middleware.GetUserID(c)

	if err := h.service.Delete(userID, uint(id)); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Board not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Solo el creador puede eliminar el tablero" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Board deleted successfully"})
}

type AddPhaseRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

func (h *BoardHandler) AddPhase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board ID"})
		return
	}

	var req AddPhaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	board, err := h.service.AddPhase(uint(id), req.Name, req.Color)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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

	board, err := h.service.RemovePhase(uint(boardID), uint(phaseID))
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "Phase not found on this board" || err.Error() == "Board not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

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

	board, err := h.service.ReorderPhases(uint(boardID), req.PhaseIDs)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, board)
}
