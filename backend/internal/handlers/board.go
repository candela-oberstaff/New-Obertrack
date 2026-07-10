package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
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
}

func boardMembershipStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrBoardNotFound), errors.Is(err, service.ErrInvitationNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrBoardAccessDenied):
		return http.StatusForbidden
	case errors.Is(err, service.ErrAlreadyBoardMember),
		errors.Is(err, service.ErrAlreadyPending),
		errors.Is(err, service.ErrInvitationResolved):
		return http.StatusConflict
	case errors.Is(err, service.ErrCreatorCannotLeave), errors.Is(err, service.ErrNotBoardMember):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func boardCtx(c *gin.Context) (userID, tenantID uint, role string, isManager, isSuperadmin bool) {
	return middleware.GetUserID(c), middleware.GetTenantID(c), middleware.GetUserRole(c),
		middleware.IsManager(c), middleware.IsSuperadmin(c)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid " + name})
		return 0, false
	}
	return uint(v), true
}

func (h *BoardHandler) GetAll(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	var companyID uint
	if isSuperadmin {
		// Superadmin must scope explicitly to a company via ?company_id=.
		// Without it, no boards are returned to avoid mixing tenants.
		if v, err := strconv.ParseUint(c.Query("company_id"), 10, 32); err == nil {
			companyID = uint(v)
		}
	} else {
		companyID = middleware.GetTenantID(c)
	}

	boards, err := h.service.GetAll(userID, role, isSuperadmin, companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get boards"})
		return
	}

	c.JSON(http.StatusOK, boards)
}

func (h *BoardHandler) GetPublicBoards(c *gin.Context) {
	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	var companyID uint
	if isSuperadmin {
		if v, err := strconv.ParseUint(c.Query("company_id"), 10, 32); err == nil {
			companyID = uint(v)
		}
	} else {
		companyID = middleware.GetTenantID(c)
	}

	boards, err := h.service.GetPublicBoards(userID, companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get public boards"})
		return
	}

	c.JSON(http.StatusOK, boards)
}

type InviteMembersRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required"`
}

func (h *BoardHandler) InviteMembers(c *gin.Context) {
	boardID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req InviteMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, tenantID, role, isManager, isSuperadmin := boardCtx(c)
	invs, err := h.service.InviteMembers(boardID, tenantID, userID, role, isManager, isSuperadmin, req.UserIDs)
	if err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"invited": len(invs), "invitations": invs})
}

func (h *BoardHandler) RequestJoin(c *gin.Context) {
	boardID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	inv, err := h.service.RequestJoin(middleware.GetUserID(c), boardID)
	if err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, inv)
}

func (h *BoardHandler) MyInvitations(c *gin.Context) {
	invs, err := h.service.ListMyInvitations(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar las invitaciones"})
		return
	}
	c.JSON(http.StatusOK, invs)
}

func (h *BoardHandler) BoardRequests(c *gin.Context) {
	h.listPending(c, models.BoardInviteKindRequest)
}

func (h *BoardHandler) BoardInvitations(c *gin.Context) {
	h.listPending(c, models.BoardInviteKindInvitation)
}

func (h *BoardHandler) listPending(c *gin.Context, kind string) {
	boardID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	userID, tenantID, role, isManager, isSuperadmin := boardCtx(c)
	invs, err := h.service.ListBoardPending(boardID, tenantID, userID, role, isManager, isSuperadmin, kind)
	if err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, invs)
}

func (h *BoardHandler) AcceptInvitation(c *gin.Context) { h.resolve(c, true) }
func (h *BoardHandler) RejectInvitation(c *gin.Context) { h.resolve(c, false) }

func (h *BoardHandler) resolve(c *gin.Context, accept bool) {
	invID, ok := parseUintParam(c, "invId")
	if !ok {
		return
	}
	userID, _, role, isManager, isSuperadmin := boardCtx(c)
	inv, err := h.service.ResolveInvitation(invID, userID, role, isManager, isSuperadmin, accept)
	if err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h *BoardHandler) CancelInvitation(c *gin.Context) {
	invID, ok := parseUintParam(c, "invId")
	if !ok {
		return
	}
	userID, _, role, isManager, isSuperadmin := boardCtx(c)
	if err := h.service.CancelInvitation(invID, userID, role, isManager, isSuperadmin); err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Invitación cancelada"})
}

func (h *BoardHandler) RemoveMember(c *gin.Context) {
	boardID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	targetID, ok := parseUintParam(c, "userId")
	if !ok {
		return
	}
	userID, tenantID, role, isManager, isSuperadmin := boardCtx(c)
	board, err := h.service.RemoveMember(boardID, targetID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, board)
}

func (h *BoardHandler) LeaveBoard(c *gin.Context) {
	boardID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.LeaveBoard(middleware.GetUserID(c), boardID); err != nil {
		c.JSON(boardMembershipStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Saliste del tablero"})
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

	var companyID uint
	if !isSuperadmin {
		companyID = middleware.GetTenantID(c)
	}

	board, err := h.service.GetByID(userID, role, isSuperadmin, companyID, uint(id))
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

	companyFilter := superadminCompanyFilter(c, middleware.IsSuperadmin(c))
	board, err := h.service.Create(userID, req.Name, req.Description, req.Color, req.MemberIDs, phases, companyFilter)
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

	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	board, err := h.service.Update(uint(id), tenantID, userID, role, isManager, isSuperadmin, updates)
	if err != nil {
		status := http.StatusNotFound
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
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
	tenantID := middleware.GetTenantID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	if err := h.service.Delete(userID, uint(id), tenantID, role, isManager, isSuperadmin); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Board not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" || err.Error() == "No tienes permisos para eliminar este tablero" {
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

	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	board, err := h.service.AddPhase(uint(id), tenantID, userID, role, isManager, isSuperadmin, req.Name, req.Color)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Access denied" || err.Error() == "Board not found" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
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

	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	board, err := h.service.RemovePhase(uint(boardID), uint(phaseID), tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "Phase not found on this board" || err.Error() == "Board not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" {
			status = http.StatusForbidden
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

	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	board, err := h.service.ReorderPhases(uint(boardID), tenantID, userID, role, isManager, isSuperadmin, req.PhaseIDs)
	if err != nil {
		status := http.StatusNotFound
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, board)
}
