package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type RBACHandler struct {
	svc service.RBACService
}

func NewRBACHandler(svc service.RBACService) *RBACHandler {
	return &RBACHandler{svc: svc}
}

// RequireRBACManager limita el módulo a superadmins y cuentas empresa.
func RequireRBACManager() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeEmployer) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo empresas o superadmins pueden gestionar roles y grupos"})
		c.Abort()
	}
}

// RequirePermission protege una ruta con el permiso efectivo del usuario para
// un módulo. Semántica v1: un usuario SIN roles asignados no se restringe
// (comportamiento histórico de su tipo de cuenta); superadmins y cuentas
// empresa nunca se restringen (gestionan los roles, no pueden auto-bloquearse).
func RequirePermission(svc service.RBACService, module, level string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeEmployer) {
			c.Next()
			return
		}
		perms, hasRoles, err := svc.EffectivePermissions(middleware.GetUserID(c), middleware.GetTenantID(c))
		if err != nil {
			// Fail-closed: si no podemos verificar los permisos, no abrimos el
			// endpoint (un fallo transitorio no debe conceder acceso de escritura).
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron verificar los permisos"})
			c.Abort()
			return
		}
		if !hasRoles {
			// Sin roles asignados: comportamiento histórico de su tipo de cuenta,
			// no se restringe.
			c.Next()
			return
		}
		granted := perms[module]
		if granted == models.PermissionEdit || (level == models.PermissionView && granted == models.PermissionView) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Tu rol no tiene permisos suficientes en este módulo"})
		c.Abort()
	}
}

// resolveTenant determina el tenant a operar: el superadmin debe indicar
// ?company_id=, una cuenta empresa siempre usa la suya.
func (h *RBACHandler) resolveTenant(c *gin.Context) (uint, bool) {
	if middleware.IsSuperadmin(c) {
		companyID := superadminCompanyFilter(c, true)
		if companyID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Selecciona una empresa (company_id) para gestionar sus roles y grupos"})
			return 0, false
		}
		return companyID, true
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tu cuenta no está asociada a una empresa"})
		return 0, false
	}
	return tenantID, true
}

func rbacError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRBACNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrRBACCrossTenant):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func paramID(c *gin.Context, name string) uint {
	id, _ := strconv.ParseUint(c.Param(name), 10, 32)
	return uint(id)
}

// ── Roles ────────────────────────────────────────────────────────────────────

func (h *RBACHandler) ListRoles(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	roles, err := h.svc.ListRoles(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los roles"})
		return
	}
	if roles == nil {
		roles = []models.Role{}
	}
	c.JSON(http.StatusOK, gin.H{"data": roles})
}

type roleRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Permissions *string `json:"permissions"`
}

func (h *RBACHandler) CreateRole(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name, description, permissions := "", "", ""
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		description = *req.Description
	}
	if req.Permissions != nil {
		permissions = *req.Permissions
	}

	role, err := h.svc.CreateRole(tenantID, middleware.GetUserID(c), name, description, permissions)
	if err != nil {
		rbacError(c, err)
		return
	}
	c.JSON(http.StatusCreated, role)
}

func (h *RBACHandler) UpdateRole(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role, err := h.svc.UpdateRole(tenantID, paramID(c, "id"), req.Name, req.Description, req.Permissions)
	if err != nil {
		rbacError(c, err)
		return
	}
	c.JSON(http.StatusOK, role)
}

func (h *RBACHandler) DeleteRole(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteRole(tenantID, paramID(c, "id")); err != nil {
		rbacError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Rol eliminado"})
}

func (h *RBACHandler) GetRoleUsers(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	users, err := h.svc.GetRoleUsers(tenantID, paramID(c, "id"))
	if err != nil {
		rbacError(c, err)
		return
	}
	if users == nil {
		users = []models.User{}
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

type memberRequest struct {
	UserID  uint   `json:"user_id"`
	UserIDs []uint `json:"user_ids"`
}

// ids normaliza el request: acepta un solo user_id o un lote user_ids.
func (r memberRequest) ids() []uint {
	if len(r.UserIDs) > 0 {
		return r.UserIDs
	}
	if r.UserID != 0 {
		return []uint{r.UserID}
	}
	return nil
}

// applyToMembers ejecuta la operación por cada usuario del request y responde
// con el conteo aplicado; si ninguno se pudo aplicar, devuelve el último error.
func applyToMembers(c *gin.Context, req memberRequest, op func(userID uint) error, okMessage string) {
	ids := req.ids()
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Indica user_id o user_ids"})
		return
	}
	applied := 0
	var lastErr error
	for _, id := range ids {
		if err := op(id); err != nil {
			lastErr = err
			continue
		}
		applied++
	}
	if applied == 0 && lastErr != nil {
		rbacError(c, lastErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": okMessage, "applied": applied})
}

func (h *RBACHandler) AssignRole(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req memberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	roleID := paramID(c, "id")
	applyToMembers(c, req, func(userID uint) error {
		return h.svc.AssignRole(tenantID, roleID, userID)
	}, "Rol asignado")
}

func (h *RBACHandler) UnassignRole(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req memberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	roleID := paramID(c, "id")
	applyToMembers(c, req, func(userID uint) error {
		return h.svc.UnassignRole(tenantID, roleID, userID)
	}, "Rol removido")
}

// GetUserRBAC devuelve los roles y grupos de un usuario del tenant.
func (h *RBACHandler) GetUserRBAC(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	roles, groups, err := h.svc.UserRBAC(tenantID, paramID(c, "userId"))
	if err != nil {
		rbacError(c, err)
		return
	}
	if roles == nil {
		roles = []models.Role{}
	}
	if groups == nil {
		groups = []models.Group{}
	}
	c.JSON(http.StatusOK, gin.H{"roles": roles, "groups": groups})
}

// ── Groups ───────────────────────────────────────────────────────────────────

func (h *RBACHandler) ListGroups(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	groups, err := h.svc.ListGroups(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los grupos"})
		return
	}
	if groups == nil {
		groups = []models.Group{}
	}
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

type groupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *RBACHandler) CreateGroup(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req groupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name, description := "", ""
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		description = *req.Description
	}

	group, err := h.svc.CreateGroup(tenantID, middleware.GetUserID(c), name, description)
	if err != nil {
		rbacError(c, err)
		return
	}
	c.JSON(http.StatusCreated, group)
}

func (h *RBACHandler) UpdateGroup(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req groupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	group, err := h.svc.UpdateGroup(tenantID, paramID(c, "id"), req.Name, req.Description)
	if err != nil {
		rbacError(c, err)
		return
	}
	c.JSON(http.StatusOK, group)
}

func (h *RBACHandler) DeleteGroup(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteGroup(tenantID, paramID(c, "id")); err != nil {
		rbacError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Grupo eliminado"})
}

func (h *RBACHandler) GetGroupMembers(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	members, err := h.svc.GetGroupMembers(tenantID, paramID(c, "id"))
	if err != nil {
		rbacError(c, err)
		return
	}
	if members == nil {
		members = []models.User{}
	}
	c.JSON(http.StatusOK, gin.H{"data": members})
}

func (h *RBACHandler) AddGroupMember(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req memberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	groupID := paramID(c, "id")
	applyToMembers(c, req, func(userID uint) error {
		return h.svc.AddGroupMember(tenantID, groupID, userID)
	}, "Miembros agregados")
}

func (h *RBACHandler) RemoveGroupMember(c *gin.Context) {
	tenantID, ok := h.resolveTenant(c)
	if !ok {
		return
	}
	var req memberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	groupID := paramID(c, "id")
	applyToMembers(c, req, func(userID uint) error {
		return h.svc.RemoveGroupMember(tenantID, groupID, userID)
	}, "Miembro removido")
}
