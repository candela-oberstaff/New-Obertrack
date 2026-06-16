package routes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// requireAdminPanel: el superadmin gestiona todo; los customer success
// (manager y analista) tienen acceso de consulta (solo GET) al panel de
// administración y empresas para dar soporte — nunca mutaciones.
func requireAdminPanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) {
			c.Next()
			return
		}
		if middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess) && c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin (customer success solo puede consultar)"})
		c.Abort()
	}
}

// requireExpedienteOwnership acota la gestión del expediente al empleador: el
// superadmin pasa libre; el empleador solo puede tocar empleos/notas/documentos
// de SU empresa (resuelve la empresa dueña del recurso más específico del path).
// Otros roles quedan fuera.
func requireExpedienteOwnership(svc service.EmploymentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) {
			c.Next()
			return
		}
		if middleware.GetUserRole(c) != string(models.UserTypeEmployer) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Requiere cuenta empresa"})
			c.Abort()
			return
		}
		tenant := middleware.GetTenantID(c)
		var owner uint
		var err error
		if v := c.Param("noteId"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 32)
			owner, err = svc.NoteCompanyID(uint(id))
		} else if v := c.Param("docId"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 32)
			owner, err = svc.DocCompanyID(uint(id))
		} else if v := c.Param("empId"); v != "" {
			id, _ := strconv.ParseUint(v, 10, 32)
			owner, err = svc.EmploymentCompanyID(uint(id))
		} else {
			// Sin recurso específico (resolver por user_id): el handler se
			// auto-acota al tenant del solicitante.
			c.Next()
			return
		}
		if err != nil || owner == 0 || owner != tenant {
			c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado sobre este expediente"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// registerAccountRoutes wires identity & administration: the current user,
// user management and the superadmin panel.
func registerAccountRoutes(api *gin.RouterGroup, d *deps) {
	api.GET("/auth/me", d.auth.Me)
	api.POST("/auth/switch-company", d.auth.SwitchCompany)

	// Expediente propio (FASE 3): el profesional consulta su CV vivo y el
	// detalle de cada empleo (solo ve lo que cada empresa decidió compartir).
	api.GET("/me/cv", d.auth.MyCV)
	api.GET("/me/cv/pdf", d.auth.MyCVPDF)
	api.GET("/me/employments", d.auth.MyEmployments)
	api.GET("/me/employments/:empId/expediente", d.auth.MyExpediente)
	api.GET("/me/employments/:empId/documents/:docId/download", d.upload.DownloadMyExpedienteDoc)

	users := api.Group("/users")
	{
		users.GET("", d.user.GetAll)
		users.GET("/employees", d.user.GetEmployees)
		users.GET("/my-team", d.user.GetMyTeam)
		users.GET("/:id", d.user.GetByID)
		users.PUT("/:id", d.user.Update)
		users.POST("/:id/toggle-status", d.user.ToggleStatus)
		users.POST("/:id/promote-manager", d.user.PromoteToManager)
		users.POST("/:id/assign-manager", d.user.AssignToManager)
		users.POST("/:id/change-password", d.user.ChangePassword)
	}

	admin := api.Group("/admin")
	admin.Use(requireAdminPanel())
	{
		admin.GET("/dashboard", d.admin.GetDashboard)
		admin.GET("/companies", d.admin.GetCompanies)
		admin.GET("/inactive-users", d.admin.GetInactiveUsers)
		admin.GET("/recent-activity", d.admin.GetRecentActivity)
		admin.GET("/absence-report", d.admin.GetAbsenceReport)
		admin.GET("/seniority", d.admin.GetSeniorityRanking)
		// Archivados (bajas + cuentas desactivadas). Global y por empresa; la
		// reactivación de un empleo revierte la baja.
		admin.GET("/archived", d.admin.GetArchived)
		admin.POST("/users/:id/employments/:empId/reactivate", d.admin.ReactivateUserEmployment)
		admin.GET("/stats", d.admin.GetStats)
		admin.GET("/users", d.admin.GetAllUsers)
		admin.POST("/users", d.admin.CreateUser)
		admin.PUT("/users/:id", d.admin.UpdateUser)
		admin.DELETE("/users/:id", d.admin.DeleteUser)
		admin.POST("/users/:id/reset-password", d.admin.ResetPassword)

		// Membresías (multi-empresa + expediente). GET lo puede consultar CS;
		// las mutaciones son solo superadmin (requireAdminPanel lo aplica).
		admin.GET("/users/:id/employments", d.admin.ListUserEmployments)
		admin.POST("/users/:id/employments", d.admin.AddUserEmployment)
		admin.POST("/users/:id/employments/:empId/end", d.admin.EndUserEmployment)

		// Expediente laboral (FASE 3): resumen + evaluaciones/notas + documentos.
		admin.GET("/users/:id/employments/:empId/expediente", d.admin.GetUserExpediente)
		admin.GET("/users/:id/employments/:empId/expediente/pdf", d.admin.DownloadExpedientePDF)
		admin.POST("/users/:id/employments/:empId/notes", d.admin.AddEmploymentNote)
		admin.PUT("/users/:id/employments/:empId/notes/:noteId", d.admin.UpdateEmploymentNote)
		admin.DELETE("/users/:id/employments/:empId/notes/:noteId", d.admin.DeleteEmploymentNote)
		admin.POST("/users/:id/employments/:empId/documents", d.admin.AddEmploymentDocument)
		admin.PUT("/users/:id/employments/:empId/documents/:docId", d.admin.UpdateEmploymentDocument)
		admin.DELETE("/users/:id/employments/:empId/documents/:docId", d.admin.DeleteEmploymentDocument)
		admin.GET("/users/:id/employments/:empId/documents/:docId/download", d.upload.DownloadExpedienteDoc)

		admin.GET("/tenants", d.admin.GetTenants)
		admin.POST("/tenants", d.admin.CreateTenant)
		admin.GET("/tenants/:id", d.admin.GetTenant)
		admin.GET("/tenants/:id/employees", d.admin.GetTenantEmployees)
		admin.GET("/tenants/:id/activity", d.admin.GetTenantActivity)
		admin.GET("/tenants/:id/archived", d.admin.GetTenantArchived)
		admin.POST("/tenants/:id/suspend", d.admin.SuspendTenant)
		admin.POST("/tenants/:id/activate", d.admin.ActivateTenant)
		admin.GET("/employees/:id/tracking", d.admin.GetEmployeeTracking)
	}

	// Gestión del expediente por el EMPLEADOR (solo su empresa). Reusa los
	// handlers del expediente; el acceso se acota con requireExpedienteOwnership.
	employer := api.Group("/employer")
	employer.Use(requireExpedienteOwnership(d.employmentSvc))
	{
		employer.GET("/users/:id/employment", d.admin.GetMyCompanyEmployment)
		employer.GET("/employments/:empId/expediente", d.admin.GetUserExpediente)
		employer.GET("/employments/:empId/expediente/pdf", d.admin.DownloadExpedientePDF)
		employer.POST("/employments/:empId/notes", d.admin.AddEmploymentNote)
		employer.PUT("/employments/:empId/notes/:noteId", d.admin.UpdateEmploymentNote)
		employer.DELETE("/employments/:empId/notes/:noteId", d.admin.DeleteEmploymentNote)
		employer.POST("/employments/:empId/documents", d.admin.AddEmploymentDocument)
		employer.PUT("/employments/:empId/documents/:docId", d.admin.UpdateEmploymentDocument)
		employer.DELETE("/employments/:empId/documents/:docId", d.admin.DeleteEmploymentDocument)
		employer.GET("/employments/:empId/documents/:docId/download", d.upload.DownloadExpedienteDoc)
	}

	// Bitácora de gestión CS: fuera del grupo /admin porque los customer
	// success deben poder ESCRIBIR seguimientos (allí solo tienen GET).
	followUps := api.Group("/follow-ups")
	followUps.Use(requireSupportInboxAccess())
	{
		followUps.GET("", d.admin.GetFollowUps)
		followUps.POST("", d.admin.CreateFollowUp)
	}

	// Registro de contactos (email/WhatsApp/chat) al profesional: misma bitácora
	// de gestión de CS, por eso comparte el acceso (CS puede escribir).
	contacts := api.Group("/users")
	contacts.Use(requireSupportInboxAccess())
	{
		contacts.POST("/:id/contacts", d.admin.LogUserContact)
	}

	// Diagnóstico técnico de plataforma: superadmins y analistas de IT.
	api.GET("/admin/audit-logs", middleware.RequirePlatformTech(), d.audit.GetLogs)
	api.GET("/metrics", middleware.RequirePlatformTech(), d.metrics.GetGlobalMetrics)
}
