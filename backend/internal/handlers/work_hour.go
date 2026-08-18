package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type WorkHourHandler struct {
	svc service.WorkHourService
}

func NewWorkHourHandler(svc service.WorkHourService) *WorkHourHandler {
	return &WorkHourHandler{svc: svc}
}

// superadminCompanyFilter reads the ?company_id= scope from the request. It only
// applies to superadmins; tenant-scoped users are always bound to their own tenant.
// readsAcrossCompanies indica si quien pide LEE a nivel plataforma: elige la
// empresa con ?company_id= en vez de quedar atado a la suya.
//
// Son el superadmin y customer success. CS ya atiende a todas las cuentas —el
// panel de empresas, el expediente y la bandeja de soporte lo tratan igual que
// al superadmin (ver requireAdminPanel)—, pero en Reportes caía en la rama de
// "usuario normal" y solo veía sus propias horas: una pantalla vacía, porque un
// CS no carga jornadas.
//
// Es un permiso de LECTURA. No sustituye a IsSuperadmin en ninguna escritura:
// esas siguen decidiéndose por separado.
func readsAcrossCompanies(c *gin.Context) bool {
	return middleware.IsSuperadmin(c) ||
		middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess)
}

// companyFilterFor lee ?company_id= para quien puede elegir empresa. Para el
// resto devuelve 0 y el servicio los acota a su propio tenant.
func companyFilterFor(c *gin.Context, crossCompany bool) uint {
	if !crossCompany {
		return 0
	}
	if v, err := strconv.ParseUint(c.Query("company_id"), 10, 32); err == nil {
		return uint(v)
	}
	return 0
}

// superadminCompanyFilter es lo mismo acotado al superadmin. Lo usan el resto de
// los módulos (tableros, canales, RBAC), donde la lectura de customer success no
// está contemplada y no se cambia aquí.
func superadminCompanyFilter(c *gin.Context, isSuperadmin bool) uint {
	return companyFilterFor(c, isSuperadmin)
}

func (h *WorkHourHandler) GetAll(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)
	// Quien lee a nivel plataforma (superadmin y CS) elige empresa; el resto
	// queda acotado a la suya.
	crossCompany := readsAcrossCompanies(c)

	userIDFilter := c.Query("user_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	companyFilter := companyFilterFor(c, crossCompany)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	// El frontend pide el mes completo (limit alto) para que sus cálculos de
	// semana/recuperación/calendario no se trunquen; acotamos a 1000 para evitar
	// que un cliente pida una página ilimitada.
	if limit <= 0 {
		limit = 10
	} else if limit > 1000 {
		limit = 1000
	}
	offset := (page - 1) * limit

	workHours, total, err := h.svc.GetAll(userID, role, crossCompany, isManager, tenantID, companyFilter, userIDFilter, startDate, endDate, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch work hours"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  workHours,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *WorkHourHandler) Create(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	workHour, err := h.svc.Create(userID, req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrInvalidDateFormat), errors.Is(err, service.ErrFutureWorkDate):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrDuplicateWorkDay), errors.Is(err, service.ErrDuplicateRecover):
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, workHour)
}

func (h *WorkHourHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid work hour ID"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isManager := middleware.IsManager(c)

	workHour, err := h.svc.Update(uint(id), tenantID, userID, role, isManager, isSuperadmin, req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Work hour not found" {
			status = http.StatusNotFound
		} else if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, workHour)
}

func (h *WorkHourHandler) Approve(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)

	approved, skipped, err := h.svc.Approve(req.IDs, userID, role, isSuperadmin, isManager, tenantID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "No work hours found" {
			status = http.StatusBadRequest
		} else if err.Error() == "No tienes permiso para aprobar estas horas." {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// approved/skipped: el frontend informa cuántas entraron y cuántas se
	// omitieron (las propias del manager, u otras fuera de su equipo).
	c.JSON(http.StatusOK, gin.H{"message": "Work hours approved", "approved": approved, "skipped": skipped})
}

func (h *WorkHourHandler) Reject(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids" binding:"required"`
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)

	rejected, skipped, err := h.svc.Reject(req.IDs, userID, role, isSuperadmin, isManager, tenantID, req.Reason)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "No work hours found" || err.Error() == "Rejection reason is required" {
			status = http.StatusBadRequest
		} else if err.Error() == "No tienes permiso para rechazar estas horas." {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Work hours rejected", "rejected": rejected, "skipped": skipped})
}

func (h *WorkHourHandler) GetSummary(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)

	companyFilter := superadminCompanyFilter(c, isSuperadmin)
	userIDFilter := c.Query("user_id")

	summary, err := h.svc.GetSummary(userID, role, isSuperadmin, isManager, tenantID, companyFilter, userIDFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *WorkHourHandler) GetPending(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)

	companyFilter := superadminCompanyFilter(c, isSuperadmin)
	userIDFilter := c.Query("user_id")

	pending, err := h.svc.GetPending(tenantID, userID, role, isSuperadmin, isManager, companyFilter, userIDFilter)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "Only employers can access this resource" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pending)
}

func (h *WorkHourHandler) SendReport(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)

	if role != "empleador" && !isSuperadmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo las empresas o superadmins pueden solicitar el envío del reporte por correo"})
		return
	}

	var req struct {
		Month int `json:"month" binding:"required"`
		Year  int `json:"year" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyFilter := superadminCompanyFilter(c, isSuperadmin)
	err := h.svc.SendReportEmail(userID, role, isSuperadmin, isManager, tenantID, req.Month, req.Year, companyFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al enviar el reporte: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reporte enviado con éxito"})
}

func (h *WorkHourHandler) DownloadPDF(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)

	if role != "empleador" && !isSuperadmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo las empresas o superadmins pueden descargar reportes"})
		return
	}

	monthStr := c.Query("month")
	yearStr := c.Query("year")
	month, _ := strconv.Atoi(monthStr)
	year, _ := strconv.Atoi(yearStr)

	if month < 1 || month > 12 || year < 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetros de mes o año inválidos"})
		return
	}

	companyFilter := superadminCompanyFilter(c, isSuperadmin)
	pdfBytes, monthName, err := h.svc.GetPDFReportBytes(userID, role, isSuperadmin, isManager, tenantID, month, year, companyFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar PDF: " + err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=reporte_jornadas_%s_%d.pdf", monthName, year))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *WorkHourHandler) DownloadExcel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	isManager := middleware.IsManager(c)
	tenantID := middleware.GetTenantID(c)

	if role != "empleador" && !isSuperadmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo las empresas o superadmins pueden descargar reportes"})
		return
	}

	monthStr := c.Query("month")
	yearStr := c.Query("year")
	month, _ := strconv.Atoi(monthStr)
	year, _ := strconv.Atoi(yearStr)

	if month < 1 || month > 12 || year < 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetros de mes o año inválidos"})
		return
	}

	companyFilter := superadminCompanyFilter(c, isSuperadmin)
	excelBytes, monthName, err := h.svc.GetExcelReportBytes(userID, role, isSuperadmin, isManager, tenantID, month, year, companyFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar Excel: " + err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=reporte_jornadas_%s_%d.xlsx", monthName, year))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", excelBytes)
}
