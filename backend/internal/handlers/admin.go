package handlers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// tempPasswordAlphabet excluye caracteres ambiguos (0/O, 1/l/I) para que la
// contraseña temporal sea fácil de dictar/transcribir.
const tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// generateTempPassword genera una contraseña temporal alfanumérica de n
// caracteres usando crypto/rand (sin caracteres ambiguos).
func generateTempPassword(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = tempPasswordAlphabet[idx.Int64()]
	}
	return string(b), nil
}

type AdminHandler struct {
	service       service.AdminService
	rbacSvc       service.RBACService
	employmentSvc service.EmploymentService
}

func NewAdminHandler(s service.AdminService, rbacSvc service.RBACService, employmentSvc service.EmploymentService) *AdminHandler {
	return &AdminHandler{service: s, rbacSvc: rbacSvc, employmentSvc: employmentSvc}
}

// seedTenantRoles siembra los roles preconfigurados de una empresa recién
// creada (best-effort: un fallo no debe impedir el alta).
func (h *AdminHandler) seedTenantRoles(c *gin.Context, tenantID uint) {
	if err := h.rbacSvc.SeedDefaultRoles(tenantID, middleware.GetUserID(c)); err != nil {
		log.Printf("[admin] no se pudieron sembrar los roles preconfigurados del tenant %d: %v", tenantID, err)
	}
}

func (h *AdminHandler) GetDashboard(c *gin.Context) {
	metrics, err := h.service.GetDashboardMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard metrics"})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func (h *AdminHandler) GetCompanies(c *gin.Context) {
	companies, err := h.service.GetCompanies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}
	c.JSON(http.StatusOK, companies)
}

func (h *AdminHandler) GetInactiveUsers(c *gin.Context) {
	days := c.DefaultQuery("days", "7")
	daysInt, _ := strconv.Atoi(days)

	users, err := h.service.GetInactiveUsers(daysInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inactive users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *AdminHandler) GetRecentActivity(c *gin.Context) {
	activities, err := h.service.GetRecentActivities()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent activities"})
		return
	}
	c.JSON(http.StatusOK, activities)
}

func (h *AdminHandler) GetAbsenceReport(c *gin.Context) {
	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))

	report, err := h.service.GetAbsenceReport(month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch absence report"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *AdminHandler) GetAllUsers(c *gin.Context) {
	userType := c.Query("user_type")
	isManager := c.Query("is_manager")
	isActive := c.Query("is_active")
	search := c.Query("q")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	users, total, err := h.service.GetAllUsers(userType, isManager, isActive, search, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *AdminHandler) GetProfessionalLocations(c *gin.Context) {
	if !middleware.IsSuperadmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin"})
		return
	}

	professionals, err := h.service.GetProfessionalLocations(c.Query("country"), c.Query("state"), c.Query("active"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch professional locations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"professionals": professionals})
}

func (h *AdminHandler) BulkEmailProfessionals(c *gin.Context) {
	if !middleware.IsSuperadmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin"})
		return
	}

	var req struct {
		UserIDs []uint `json:"user_ids" binding:"required"`
		Subject string `json:"subject" binding:"required"`
		Body    string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay destinatarios seleccionados"})
		return
	}

	result := h.service.BulkEmailProfessionals(req.UserIDs, req.Subject, req.Body)
	c.JSON(http.StatusOK, gin.H{"sent": result.Sent, "failed": result.Failed})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Email       string `json:"email" binding:"required,email"`
		Password    string `json:"password" binding:"required"`
		UserType    string `json:"user_type" binding:"required"`
		CompanyName string `json:"company_name"`
		JobTitle    string `json:"job_title"`
		EmpleadorID *uint  `json:"empleador_id"`
		ManagerID   *uint  `json:"manager_id"`
		IsManager   bool   `json:"is_manager"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		State       string `json:"state"`
		City        string `json:"city"`
		Location    string `json:"location"`
		Industry    string `json:"industry"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := map[string]interface{}{
		"name":         req.Name,
		"email":        req.Email,
		"password":     req.Password,
		"user_type":    req.UserType,
		"company_name": req.CompanyName,
		"job_title":    req.JobTitle,
		"is_manager":   req.IsManager,
		"phone_number": req.PhoneNumber,
		"country":      req.Country,
		"state":        req.State,
		"city":         req.City,
		"location":     req.Location,
		"industry":     req.Industry,
	}
	// Solo profesionales y customer success pueden quedar vinculados a una empresa.
	if req.EmpleadorID != nil && (req.UserType == "profesional" || req.UserType == "customer_success") {
		payload["empleador_id"] = *req.EmpleadorID
	}
	if req.ManagerID != nil {
		payload["manager_id"] = *req.ManagerID
	}

	user, err := h.service.CreateUser(payload)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Invalid user type" || strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// Toda empresa nueva nace con los roles preconfigurados.
	if user.UserType == models.UserTypeEmployer {
		h.seedTenantRoles(c, user.ID)
	}
	// Dual-write de la membresía (fase 0). Best-effort.
	if err := h.employmentSvc.SyncActiveForUser(user); err != nil {
		log.Printf("[admin] no se pudo sincronizar la membresía del usuario %d: %v", user.ID, err)
	}

	c.JSON(http.StatusCreated, user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		JobTitle    string `json:"job_title"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		State       string `json:"state"`
		City        string `json:"city"`
		Location    string `json:"location"`
		Address     string `json:"address"`
		Industry    string `json:"industry"`
		CompanyName string `json:"company_name"`
		IsActive    *bool  `json:"is_active"`
		IsManager   *bool  `json:"is_manager"`
		UserType    string `json:"user_type"`
		EmpleadorID *uint  `json:"empleador_id"`
		ManagerID   *uint  `json:"manager_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Guard: an admin must not be able to deactivate their own account. Doing so
	// would lock them out the moment their session ends (login blocks inactive
	// users), with no way to recover if they are the last active superadmin.
	if req.IsActive != nil && !*req.IsActive && uint(id) == middleware.GetUserID(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No puedes desactivar tu propia cuenta."})
		return
	}

	demoting := req.UserType != "" && req.UserType != string(models.UserTypeSuperadmin)
	deactivating := req.IsActive != nil && !*req.IsActive
	if demoting || deactivating {
		last, lerr := h.service.IsLastActiveSuperadmin(uint(id))
		if lerr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo verificar los superadmins activos"})
			return
		}
		if last {
			c.JSON(http.StatusConflict, gin.H{"error": service.ErrLastSuperadmin.Error()})
			return
		}
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.JobTitle != "" {
		updates["job_title"] = req.JobTitle
	}
	if req.PhoneNumber != "" {
		updates["phone_number"] = req.PhoneNumber
	}
	if req.Country != "" {
		updates["country"] = req.Country
	}
	if req.State != "" {
		updates["state"] = req.State
	}
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.Industry != "" {
		updates["industry"] = req.Industry
	}
	if req.CompanyName != "" {
		updates["company_name"] = req.CompanyName
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsManager != nil {
		updates["is_manager"] = *req.IsManager
	}
	if req.UserType != "" {
		updates["user_type"] = req.UserType
	}
	if req.EmpleadorID != nil {
		updates["empleador_id"] = *req.EmpleadorID
	}
	if req.ManagerID != nil {
		updates["manager_id"] = *req.ManagerID
	}

	user, err := h.service.UpdateUser(uint(id), updates)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "a su cargo") {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// Dual-write de la membresía (fase 0): mantiene employments al día cuando
	// cambia la empresa/cargo/manager del usuario. Best-effort.
	if err := h.employmentSvc.SyncActiveForUser(user); err != nil {
		log.Printf("[admin] no se pudo sincronizar la membresía del usuario %d: %v", user.ID, err)
	}

	c.JSON(http.StatusOK, user)
}

// GetManagerReports lista los profesionales a cargo de un manager (para mostrar
// el equipo que hay que reasignar antes de degradar/eliminar al manager).
func (h *AdminHandler) GetManagerReports(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	reports, err := h.service.GetManagerReports(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}

// BulkAssignManager asigna un manager a varios profesionales a la vez.
func (h *AdminHandler) BulkAssignManager(c *gin.Context) {
	var req struct {
		ProfessionalIDs []uint `json:"professional_ids"`
		ManagerID       *uint  `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.ProfessionalIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay profesionales seleccionados"})
		return
	}
	assigned, skipped, err := h.service.BulkAssignManager(req.ProfessionalIDs, req.ManagerID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"assigned": assigned, "skipped": skipped})
}

func (h *AdminHandler) BulkDeleteUsers(c *gin.Context) {
	var req struct {
		UserIDs []uint `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay usuarios seleccionados"})
		return
	}
	deleted, skipped, err := h.service.BulkDeleteUsers(req.UserIDs, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped})
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if uint(id) == middleware.GetUserID(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No puedes eliminar tu propia cuenta."})
		return
	}

	if err := h.service.DeleteUser(uint(id)); err != nil {
		if errors.Is(err, service.ErrLastSuperadmin) || strings.Contains(err.Error(), "a su cargo") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// CreateEmployee da de alta un profesional en la empresa del EMPLEADOR
// solicitante. Genera una contraseña temporal (que se muestra una sola vez) y
// sincroniza la membresía. El profesional queda vinculado al tenant del
// solicitante (no se acepta empresa por el body).
func (h *AdminHandler) CreateEmployee(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Email       string `json:"email" binding:"required,email"`
		JobTitle    string `json:"job_title"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		State       string `json:"state"`
		City        string `json:"city"`
		Location    string `json:"location"`
		ManagerID   *uint  `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tu cuenta no está asociada a una empresa"})
		return
	}

	temp, err := generateTempPassword(12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar la contraseña temporal"})
		return
	}

	payload := map[string]interface{}{
		"name":         req.Name,
		"email":        req.Email,
		"password":     temp,
		"user_type":    "profesional",
		"empleador_id": uint(tenantID),
		"job_title":    req.JobTitle,
		"phone_number": req.PhoneNumber,
		"country":      req.Country,
		"state":        req.State,
		"city":         req.City,
		"location":     req.Location,
	}
	if req.ManagerID != nil && *req.ManagerID > 0 {
		payload["manager_id"] = *req.ManagerID
	}

	user, err := h.service.CreateUser(payload)
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		switch {
		case strings.Contains(strings.ToLower(msg), "unique") ||
			strings.Contains(strings.ToLower(msg), "duplicate") ||
			strings.Contains(msg, "already registered"):
			status = http.StatusConflict
			msg = "Ya existe un usuario con ese correo"
		case msg == "Invalid user type" ||
			strings.Contains(msg, "Manager inválido") ||
			strings.Contains(msg, "no es válida"):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}

	// Dual-write de la membresía (best-effort, igual que en CreateUser admin).
	if err := h.employmentSvc.SyncActiveForUser(user); err != nil {
		log.Printf("[employer] no se pudo sincronizar la membresía del profesional %d: %v", user.ID, err)
	}

	// La contraseña temporal en claro se devuelve UNA sola vez.
	c.JSON(http.StatusCreated, gin.H{"user": user, "temp_password": temp})
}

func (h *AdminHandler) UpdateEmployee(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		JobTitle    string `json:"job_title"`
		PhoneNumber string `json:"phone_number"`
		Country     string `json:"country"`
		State       string `json:"state"`
		City        string `json:"city"`
		Location    string `json:"location"`
		IsActive    *bool  `json:"is_active"`
		IsManager   *bool  `json:"is_manager"`
		ManagerID   *uint  `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tu cuenta no está asociada a una empresa"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.JobTitle != "" {
		updates["job_title"] = req.JobTitle
	}
	if req.PhoneNumber != "" {
		updates["phone_number"] = req.PhoneNumber
	}
	if req.Country != "" {
		updates["country"] = req.Country
	}
	if req.State != "" {
		updates["state"] = req.State
	}
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsManager != nil {
		updates["is_manager"] = *req.IsManager
	}
	if req.ManagerID != nil {
		updates["manager_id"] = *req.ManagerID
	}

	user, err := h.service.UpdateUserScoped(uint(id), updates, uint(tenantID))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case err.Error() == "User not found":
			status = http.StatusNotFound
		case err.Error() == "Access denied":
			status = http.StatusForbidden
		case strings.Contains(err.Error(), "a su cargo"):
			status = http.StatusConflict
		case strings.Contains(err.Error(), "Manager inválido") ||
			strings.Contains(err.Error(), "no se puede cambiar"):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if err := h.employmentSvc.SyncActiveForUser(user); err != nil {
		log.Printf("[employer] no se pudo sincronizar la membresía del profesional %d: %v", user.ID, err)
	}

	c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) ResetEmployeePassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tu cuenta no está asociada a una empresa"})
		return
	}

	temp, err := generateTempPassword(12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar la contraseña temporal"})
		return
	}

	if err := h.service.ResetPasswordScoped(uint(id), temp, uint(tenantID)); err != nil {
		status := http.StatusInternalServerError
		switch err.Error() {
		case "User not found":
			status = http.StatusNotFound
		case "Access denied":
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"temp_password": temp})
}

// DeleteEmployee elimina (soft delete) un profesional. El superadmin borra sin
// restricción de tenant; el empleador solo puede borrar usuarios de SU empresa.
// Reusa el guard de orphans (manager con equipo -> 409).
func (h *AdminHandler) DeleteEmployee(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if middleware.IsSuperadmin(c) {
		err = h.service.DeleteUser(uint(id))
	} else {
		err = h.service.DeleteUserScoped(uint(id), middleware.GetTenantID(c))
	}
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		switch {
		case msg == "User not found":
			status = http.StatusNotFound
		case msg == "Access denied":
			status = http.StatusForbidden
		case strings.Contains(msg, "a su cargo"):
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profesional eliminado"})
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Resetting password for user ID: %d", id)

	if err := h.service.ResetPassword(uint(id), req.NewPassword); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *AdminHandler) GetTenants(c *gin.Context) {
	tenants, err := h.service.GetTenants()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenants"})
		return
	}
	c.JSON(http.StatusOK, tenants)
}

func (h *AdminHandler) GetTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	tenant, err := h.service.GetTenant(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

func (h *AdminHandler) GetTenantEmployees(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	employees, err := h.service.GetTenantEmployees(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant employees"})
		return
	}
	c.JSON(http.StatusOK, employees)
}

func (h *AdminHandler) GetEmployeeTracking(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid employee ID"})
		return
	}

	tracking, err := h.service.GetEmployeeTracking(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Employee not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tracking)
}

func (h *AdminHandler) GetTenantActivity(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	activities, err := h.service.GetTenantActivities(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant activity"})
		return
	}
	c.JSON(http.StatusOK, activities)
}

func (h *AdminHandler) SetTenantStatus(c *gin.Context, active bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	tenant, err := h.service.SetTenantStatus(uint(id), active, middleware.GetUserID(c))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Tenant not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

func (h *AdminHandler) SuspendTenant(c *gin.Context) {
	h.SetTenantStatus(c, false)
}

func (h *AdminHandler) ActivateTenant(c *gin.Context) {
	h.SetTenantStatus(c, true)
}

func (h *AdminHandler) CreateTenant(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		CompanyName string `json:"company_name" binding:"required"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		UserID      *uint  `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID != nil {
		tenant, err := h.service.AssignTenant(*req.UserID, req.CompanyName)
		if err != nil {
			status := http.StatusBadRequest
			if err.Error() == "Usuario no encontrado" {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		h.seedTenantRoles(c, tenant.ID)
		c.JSON(http.StatusCreated, tenant)
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, email y password son obligatorios para crear una cuenta nueva"})
		return
	}

	tenant, err := h.service.CreateTenant(req.Name, req.CompanyName, req.Email, req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Email already registered" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.seedTenantRoles(c, tenant.ID)
	c.JSON(http.StatusCreated, tenant)
}

// GetFollowUps devuelve el estado vigente de gestión por profesional para un
// tipo de seguimiento (?kind=inactivity|absence).
func (h *AdminHandler) GetFollowUps(c *gin.Context) {
	items, err := h.service.GetLatestFollowUps(c.DefaultQuery("kind", models.FollowUpKindInactivity))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []repository.FollowUpInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CreateFollowUp registra una entrada en la bitácora de gestión.
func (h *AdminHandler) CreateFollowUp(c *gin.Context) {
	var req struct {
		UserID uint   `json:"user_id" binding:"required"`
		Kind   string `json:"kind" binding:"required"`
		Status string `json:"status" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	followUp, err := h.service.CreateFollowUp(req.UserID, middleware.GetUserID(c), req.Kind, req.Status, req.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, followUp)
}

// ── Membresías (employments): multi-empresa + expediente ────────────────────

// ListUserEmployments lista las membresías (activas y terminadas) de un usuario.
func (h *AdminHandler) ListUserEmployments(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	views, err := h.employmentSvc.ListForUser(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar las membresías"})
		return
	}
	if views == nil {
		views = []service.EmploymentView{}
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

// AddUserEmployment vincula al usuario con una empresa adicional.
func (h *AdminHandler) AddUserEmployment(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		CompanyID   uint   `json:"company_id" binding:"required"`
		JobTitle    string `json:"job_title"`
		StartReason string `json:"start_reason"`
		ManagerID   *uint  `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	employment, err := h.employmentSvc.AddEmployment(uint(userID), req.CompanyID, req.JobTitle, req.StartReason, req.ManagerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, employment)
}

// EndUserEmployment finaliza una membresía (el profesional deja esa empresa).
func (h *AdminHandler) EndUserEmployment(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	var req struct {
		EndReason string `json:"end_reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.employmentSvc.EndEmployment(uint(userID), uint(empID), req.EndReason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Membresía finalizada"})
}

// UpdateEmploymentManager fija el manager de un empleo concreto del usuario
// (o lo desasigna si manager_id es null).
func (h *AdminHandler) UpdateEmploymentManager(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	var req struct {
		ManagerID *uint `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.employmentSvc.UpdateEmploymentManager(uint(userID), uint(empID), req.ManagerID); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "Manager inválido") {
			status = http.StatusBadRequest
		} else if err.Error() == "Membresía no encontrada" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Manager actualizado"})
}

// employmentManagerError mapea errores del conjunto de managers a HTTP:
// "Manager inválido" -> 400, "Membresía no encontrada" -> 404, else 500.
func employmentManagerStatus(err error) int {
	if strings.Contains(err.Error(), "Manager inválido") {
		return http.StatusBadRequest
	}
	if err.Error() == "Membresía no encontrada" {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// GetEmploymentManagers devuelve el CONJUNTO de managers de un empleo (principal
// primero, luego por nombre).
func (h *AdminHandler) GetEmploymentManagers(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	views, err := h.employmentSvc.ListEmploymentManagers(uint(userID), uint(empID))
	if err != nil {
		c.JSON(employmentManagerStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

// AddEmploymentManager agrega un manager ADICIONAL (no principal) al empleo.
func (h *AdminHandler) AddEmploymentManager(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	var req struct {
		ManagerID uint `json:"manager_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.employmentSvc.AddEmploymentManager(uint(userID), uint(empID), req.ManagerID); err != nil {
		c.JSON(employmentManagerStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Manager agregado"})
}

// RemoveEmploymentManager quita un manager del conjunto del empleo.
func (h *AdminHandler) RemoveEmploymentManager(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	managerID, _ := strconv.ParseUint(c.Param("managerId"), 10, 32)
	if err := h.employmentSvc.RemoveEmploymentManager(uint(userID), uint(empID), uint(managerID)); err != nil {
		c.JSON(employmentManagerStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Manager quitado"})
}

// SetPrimaryEmploymentManager marca un manager ya asignado como principal.
func (h *AdminHandler) SetPrimaryEmploymentManager(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	managerID, _ := strconv.ParseUint(c.Param("managerId"), 10, 32)
	if err := h.employmentSvc.SetPrimaryEmploymentManager(uint(userID), uint(empID), uint(managerID)); err != nil {
		c.JSON(employmentManagerStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Manager principal actualizado"})
}

// --- Expediente (FASE 3): vista de la empresa (RR.HH.) ---

// GetMyCompanyEmployment resuelve el empleo de un profesional en la empresa del
// solicitante (para que el empleador abra su expediente por user_id).
func (h *AdminHandler) GetMyCompanyEmployment(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	view, err := h.employmentSvc.EmploymentForUserInCompany(uint(userID), middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

// GetUserExpediente devuelve el expediente completo de un empleo (resumen,
// notas y documentos) para la audiencia empresa.
func (h *AdminHandler) GetUserExpediente(c *gin.Context) {
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	exp, err := h.employmentSvc.GetExpediente(uint(empID), service.AudienceCompany)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, exp)
}

// AddEmploymentNote registra una evaluación o anotación en el expediente.
func (h *AdminHandler) AddEmploymentNote(c *gin.Context) {
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	var req struct {
		Kind       string `json:"kind"`
		Rating     *int   `json:"rating"`
		Content    string `json:"content" binding:"required"`
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	note, err := h.employmentSvc.AddNote(uint(empID), middleware.GetUserID(c), req.Kind, req.Rating, req.Content, req.Visibility)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, note)
}

// DeleteEmploymentNote elimina una nota del expediente.
func (h *AdminHandler) DeleteEmploymentNote(c *gin.Context) {
	noteID, _ := strconv.ParseUint(c.Param("noteId"), 10, 32)
	if err := h.employmentSvc.DeleteNote(uint(noteID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Nota eliminada"})
}

// parseDatePtr interpreta una fecha "YYYY-MM-DD"; vacío o inválido => nil.
func parseDatePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t
	}
	return nil
}

// AddEmploymentDocument adjunta un documento (ya subido por /uploads) al
// expediente del empleo.
func (h *AdminHandler) AddEmploymentDocument(c *gin.Context) {
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	var req struct {
		Title      string `json:"title"`
		FileName   string `json:"file_name" binding:"required"`
		FileURL    string `json:"file_url" binding:"required"`
		FileSize   int64  `json:"file_size"`
		MimeType   string `json:"mime_type"`
		Visibility string `json:"visibility"`
		ExpiresAt  string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	doc, err := h.employmentSvc.AddDocument(uint(empID), middleware.GetUserID(c), req.Title, req.FileName, req.FileURL, req.FileSize, req.MimeType, req.Visibility, parseDatePtr(req.ExpiresAt))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, doc)
}

// UpdateEmploymentNote edita una evaluación/nota del expediente.
func (h *AdminHandler) UpdateEmploymentNote(c *gin.Context) {
	noteID, _ := strconv.ParseUint(c.Param("noteId"), 10, 32)
	var req struct {
		Kind       string `json:"kind"`
		Rating     *int   `json:"rating"`
		Content    string `json:"content" binding:"required"`
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	note, err := h.employmentSvc.UpdateNote(uint(noteID), req.Kind, req.Rating, req.Content, req.Visibility)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, note)
}

// UpdateEmploymentDocument edita los metadatos de un documento (título,
// visibilidad, vencimiento); no cambia el archivo.
func (h *AdminHandler) UpdateEmploymentDocument(c *gin.Context) {
	docID, _ := strconv.ParseUint(c.Param("docId"), 10, 32)
	var req struct {
		Title      string `json:"title"`
		Visibility string `json:"visibility"`
		ExpiresAt  string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	doc, err := h.employmentSvc.UpdateDocument(uint(docID), req.Title, req.Visibility, parseDatePtr(req.ExpiresAt))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, doc)
}

// DeleteEmploymentDocument elimina un documento del expediente.
func (h *AdminHandler) DeleteEmploymentDocument(c *gin.Context) {
	docID, _ := strconv.ParseUint(c.Param("docId"), 10, 32)
	if err := h.employmentSvc.DeleteDocument(uint(docID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Documento eliminado"})
}

// DownloadExpedientePDF descarga el expediente completo de un empleo en PDF.
func (h *AdminHandler) DownloadExpedientePDF(c *gin.Context) {
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	bytes, name, err := h.employmentSvc.GetExpedientePDF(uint(empID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=expediente_%s.pdf", slugify(name)))
	c.Data(http.StatusOK, "application/pdf", bytes)
}

// LogUserContact registra un intento de contacto (email/WhatsApp/chat) a un
// profesional, para que quede en el historial de su expediente.
func (h *AdminHandler) LogUserContact(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Channel string `json:"channel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.employmentSvc.LogContact(uint(userID), middleware.GetUserID(c), req.Channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Contacto registrado"})
}

// GetArchived lista profesionales archivados (bajas + desactivados) a nivel
// global (todas las empresas).
func (h *AdminHandler) GetArchived(c *gin.Context) {
	h.respondArchived(c, 0)
}

// GetTenantArchived lista los archivados de una empresa específica.
func (h *AdminHandler) GetTenantArchived(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	h.respondArchived(c, uint(id))
}

func (h *AdminHandler) respondArchived(c *gin.Context, tenantID uint) {
	entries, err := h.service.GetArchived(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los archivados"})
		return
	}
	if entries == nil {
		entries = []repository.ArchivedEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// ReactivateUserEmployment revierte la baja de un empleo (vuelve a estar activo).
func (h *AdminHandler) ReactivateUserEmployment(c *gin.Context) {
	empID, _ := strconv.ParseUint(c.Param("empId"), 10, 32)
	if err := h.employmentSvc.ReactivateEmployment(uint(empID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Empleo reactivado"})
}

// GetSeniorityRanking lista los profesionales por antigüedad (métricas CS).
func (h *AdminHandler) GetSeniorityRanking(c *gin.Context) {
	items, err := h.service.GetSeniorityRanking()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar el ranking de antigüedad"})
		return
	}
	if items == nil {
		items = []repository.SeniorityItem{}
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) CreateSuperAdmin(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.CreateSuperAdmin(req.Name, req.Email, req.Password, false)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Superadmin already exists. Use /api/seed/reset-superadmin to recreate." {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Superadmin created successfully",
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"user_type": user.UserType,
		},
	})
}

func (h *AdminHandler) ResetSuperAdmin(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.ResetSuperAdmin(req.Name, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create superadmin: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Superadmin reset successfully",
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"user_type": user.UserType,
		},
	})
}

func (h *AdminHandler) MakeSuperAdmin(c *gin.Context) {
	email := c.Param("email")
	user, err := h.service.MakeSuperAdmin(email)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "User not found with email" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User is now a superadmin",
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"user_type":     user.UserType,
			"is_superadmin": user.IsSuperadmin,
		},
	})
}

func (h *AdminHandler) CreateSuperAdminForced(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.CreateSuperAdmin(req.Name, req.Email, req.Password, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create superadmin: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Superadmin created successfully",
		"user": gin.H{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"user_type": user.UserType,
		},
	})
}
