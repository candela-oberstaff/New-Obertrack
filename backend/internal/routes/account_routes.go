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
// requireAdminPanel protege el panel de administración.
//
// Customer Success opera con el mismo alcance que un superadmin: gestiona
// usuarios, empresas, expedientes e incidentes. Antes solo podía consultar (GET),
// que es lo que lo dejaba fuera de su propio trabajo diario.
//
// Las cuatro funciones que NO le corresponden —Papelera, Auditoría,
// Configuración y Novedades— NO se controlan aquí: cada una lleva su propia
// guarda en su ruta, para que abrir este panel no las arrastre de rebote.
func requireAdminPanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) {
			c.Next()
			return
		}
		if middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin o customer success"})
		c.Abort()
	}
}

// requireSupportWrite: escribir en el EXPEDIENTE de una empresa (notas y
// contactos) lo pueden hacer también los customer success, no solo el
// superadmin.
//
// Es la excepción deliberada a requireAdminPanel, y la justifica el propósito
// del expediente: es material de seguimiento del área de soporte. Si la única
// gente que atiende a los clientes no puede anotar lo que habla con ellos, el
// expediente se queda a medias y el seguimiento acaba en un canal aparte.
//
// Sigue sin abrirles nada que cambie la cuenta: suspender, editar, crear o
// tocar profesionales continúa siendo exclusivo del superadmin.
func requireSupportWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeCustomerSuccess) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere superadmin o customer success"})
		c.Abort()
	}
}

// requireManageUsers: las acciones de gestión de usuarios (activar/desactivar,
// promover/quitar manager, asignar/reasignar equipo) solo las hace el dueño de
// la empresa (empleador) o un superadmin. Defensa en profundidad a nivel de
// ruta sobre la validación del servicio (authorizeAdminAction). El perfil propio
// (PUT /users/:id) y el cambio de contraseña quedan fuera (son self-service).
func requireManageUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeEmployer) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere empresa o superadmin"})
		c.Abort()
	}
}

// requireManageTeam: las acciones sobre el EQUIPO (promover/quitar manager,
// asignar manager, reasignar equipo) las hace el empleador, un superadmin o un
// supervisor.
//
// Esta guarda solo decide quién puede INTENTARLO; hasta dónde llega el
// supervisor lo decide el servicio (authorizeTeamAction), que exige que el
// objetivo esté dentro de su árbol. Se separa de requireManageUsers a propósito:
// activar/desactivar una cuenta sigue siendo cosa de la empresa, porque deja a
// alguien fuera de la aplicación y eso no es gestionar un equipo.
func requireManageTeam() gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsSuperadmin(c) || middleware.GetUserRole(c) == string(models.UserTypeEmployer) {
			c.Next()
			return
		}
		if service.SupervisorScopeEnabled() && middleware.IsSupervisor(c) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Requiere empresa, supervisor o superadmin"})
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

	// Organigrama de una empresa. Lo consultan el superadmin (con ?company_id=),
	// el empleador sobre la suya y el supervisor sobre su rama; el recorte por rol
	// lo aplica el servicio, así que no lleva guarda de panel.
	api.GET("/org-chart", d.admin.GetOrgChart)

	// Feature flags para el front (p.ej. mostrar el modo multi-manager).
	api.GET("/features", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"multi_manager_reads": service.MultiManagerReadsEnabled(),
			"supervisor_scope":    service.SupervisorScopeEnabled(),
		})
	})

	// Expediente propio (FASE 3): el profesional consulta su CV vivo y el
	// detalle de cada empleo (solo ve lo que cada empresa decidió compartir).
	api.GET("/me/cv", d.auth.MyCV)
	api.GET("/me/cv/pdf", d.auth.MyCVPDF)
	api.GET("/me/employments", d.auth.MyEmployments)
	// Mi Wallet: cada usuario consulta SOLO sus propios pagos (self-service, solo
	// lectura). El filtrado por email vive en el servicio; la empresa no ve las
	// ganancias individuales.
	api.GET("/me/wallet", d.wallet.MyWallet)
	// Google Calendar: vínculo PERSONAL con la cuenta de Google del usuario
	// (self-service, como Wallet). Todas operan sobre la sesión y no reciben un
	// user_id, así que nadie puede tocar el vínculo de otro. El callback del
	// consentimiento vive en public_routes: lo invoca Google, no el frontend.
	google := api.Group("/me/integrations/google")
	{
		google.GET("/status", d.googleCal.Status)
		google.POST("/connect", d.googleCal.Connect)
		google.DELETE("", d.googleCal.Disconnect)
	}

	api.GET("/me/employments/:empId/expediente", d.auth.MyExpediente)
	api.GET("/me/employments/:empId/documents/:docId/download", d.upload.DownloadMyExpedienteDoc)

	users := api.Group("/users")
	{
		users.GET("", d.user.GetAll)
		users.GET("/employees", d.user.GetEmployees)
		users.GET("/my-team", d.user.GetMyTeam)
		users.GET("/:id", d.user.GetByID)
		users.PUT("/:id", d.user.Update)
		users.POST("/:id/toggle-status", requireManageUsers(), d.user.ToggleStatus)
		users.POST("/:id/promote-manager", requireManageTeam(), d.user.PromoteToManager)
		users.POST("/:id/assign-manager", requireManageTeam(), d.user.AssignToManager)
		users.POST("/:id/reassign-team", requireManageTeam(), d.user.ReassignTeam)
		users.POST("/:id/change-password", d.user.ChangePassword)
	}

	profile := api.Group("/profile")
	{
		profile.POST("/change-requests", d.profileChange.Create)
		profile.GET("/change-request", d.profileChange.GetMine)
	}
	profileReview := api.Group("/admin/profile-change-requests")
	profileReview.Use(requireSupportInboxAccess())
	{
		profileReview.GET("/user/:id", d.profileChange.GetForUser)
		profileReview.POST("/:reqId/apply", d.profileChange.Apply)
		profileReview.POST("/:reqId/reject", d.profileChange.Reject)
	}

	admin := api.Group("/admin")
	admin.Use(requireAdminPanel())
	{
		admin.GET("/users/:id/reports", d.admin.GetManagerReports)
		admin.POST("/bulk-assign-manager", d.admin.BulkAssignManager)
		// Borrar cuentas es solo de superadmin: CS gestiona y edita, pero no
		// elimina (misma regla que la Papelera).
		admin.POST("/bulk-delete-users", middleware.RequireSuperadmin(), d.admin.BulkDeleteUsers)
		// Papelera: excluida de Customer Success. Va con su propia guarda porque
		// cuelga del grupo admin, que sí abre a CS — sin esto entraría de rebote.
		admin.GET("/trash", middleware.RequireSuperadmin(), d.trash.List)
		admin.POST("/trash/restore", middleware.RequireSuperadmin(), d.trash.Restore)
		admin.POST("/trash/purge", middleware.RequireSuperadmin(), d.trash.Purge)
		admin.GET("/professionals/locations", d.admin.GetProfessionalLocations)
		admin.POST("/professionals/bulk-email", d.admin.BulkEmailProfessionals)
		// Entrega de acceso a la plataforma (enlace de alta o clave temporal).
		admin.POST("/users/send-access", d.admin.SendAccessEmails)
		admin.GET("/import/template", d.admin.DownloadImportTemplate)
		admin.POST("/import/preview", d.admin.ImportPreview)
		admin.POST("/import/execute", d.admin.ImportExecute)
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
		admin.DELETE("/users/:id", middleware.RequireSuperadmin(), d.admin.DeleteUser)
		admin.POST("/users/:id/reset-password", d.admin.ResetPassword)

		// Configuración de la app: SOLO superadmin, ni siquiera lectura para CS
		// (requireAdminPanel dejaría pasar sus GET, RequireSuperadmin los corta).
		settings := admin.Group("/settings")
		settings.Use(middleware.RequireSuperadmin())
		{
			settings.GET("/report-schedule", d.reportSched.Get)
			settings.PUT("/report-schedule", d.reportSched.Update)
			settings.POST("/report-schedule/run-now", d.reportSched.RunNow)
			settings.GET("/report-schedule/runs", d.reportSched.ListRuns)

			// Correos del sistema: catálogo, interruptor por tipo y envío de
			// una muestra para revisar el formato.
			settings.GET("/emails", d.emailSettings.List)
			// Sin endpoint de "apagar todo": un solo clic capaz de tumbar todos
			// los correos (incluidos los de contraseña) se consideró demasiado
			// peligroso. El apagado se hace tipo por tipo.
			settings.PUT("/emails/:key", d.emailSettings.Update)
			settings.POST("/emails/:key/test", d.emailSettings.SendTest)
		}

		// Membresías (multi-empresa + expediente). GET lo puede consultar CS;
		// las mutaciones son solo superadmin (requireAdminPanel lo aplica).
		admin.GET("/users/:id/employments", d.admin.ListUserEmployments)
		admin.POST("/users/:id/employments", d.admin.AddUserEmployment)
		admin.POST("/users/:id/employments/:empId/end", d.admin.EndUserEmployment)
		admin.PUT("/users/:id/employments/:empId/manager", d.admin.UpdateEmploymentManager)
		// Corrección de la fecha de ingreso: el alta la sella con el día en que
		// se cargó, que casi nunca es el día en que la persona empezó.
		admin.PUT("/users/:id/employments/:empId/start", d.admin.UpdateEmploymentStart)

		// Conjunto de managers por empleo (multi-manager). El single de arriba
		// gestiona el principal; estos gestionan el CONJUNTO (adicionales +
		// promover principal).
		admin.GET("/users/:id/employments/:empId/managers", d.admin.GetEmploymentManagers)
		admin.POST("/users/:id/employments/:empId/managers", d.admin.AddEmploymentManager)
		admin.DELETE("/users/:id/employments/:empId/managers/:managerId", d.admin.RemoveEmploymentManager)
		admin.PUT("/users/:id/employments/:empId/managers/:managerId/primary", d.admin.SetPrimaryEmploymentManager)

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
		admin.GET("/tenants/:id/tickets", d.admin.GetTenantTickets)
		admin.GET("/tenants/:id/activity", d.admin.GetTenantActivity)
		admin.GET("/tenants/:id/activity/people", d.admin.GetTenantActivityPeople)
		admin.GET("/tenants/:id/notes/pinned", d.admin.GetTenantPinnedNotes)
		admin.GET("/tenants/:id/archived", d.admin.GetTenantArchived)
		// Pestaña Actividad de la ficha de empresa: el mismo semáforo de
		// inactividad y reporte de ausencias del panel, acotados a la empresa.
		admin.GET("/tenants/:id/inactive-users", d.admin.GetTenantInactiveUsers)
		admin.GET("/tenants/:id/absence-report", d.admin.GetTenantAbsenceReport)
		admin.POST("/tenants/:id/suspend", d.admin.SuspendTenant)
		admin.POST("/tenants/:id/activate", d.admin.ActivateTenant)
		admin.GET("/employees/:id/tracking", d.admin.GetEmployeeTracking)
		// Soporte de UNA persona: lo que se le abrió y lo que le alcanzó. Van
		// bajo /employees y no bajo /tenants porque son de la persona, no de su
		// empresa (que puede haber cambiado desde que se abrió el ticket).
		admin.GET("/employees/:id/tickets", d.admin.GetEmployeeTickets)
		admin.GET("/employees/:id/incidents", d.incident.ListForUser)

		admin.GET("/incidents", d.incident.List)
		admin.POST("/incidents", d.incident.Create)
		admin.GET("/incidents/:id", d.incident.Get)
		admin.PUT("/incidents/:id/close", d.incident.Close)
		admin.POST("/incidents/:id/broadcast", d.incident.Broadcast)
		admin.PUT("/incidents/:id/responses/:userId", d.incident.UpsertResponse)

		admin.GET("/emergency-templates", d.emergencyTpl.List)
		admin.POST("/emergency-templates", d.emergencyTpl.Create)
		admin.PUT("/emergency-templates/:id", d.emergencyTpl.Update)
		admin.DELETE("/emergency-templates/:id", d.emergencyTpl.Delete)
	}

	// Escritura en el expediente de empresa. Va en un grupo aparte del panel de
	// admin porque es la única mutación que también hace customer success: son
	// quienes atienden a los clientes y quienes tienen algo que anotar.
	support := api.Group("/admin")
	support.Use(requireSupportWrite())
	{
		support.POST("/tenants/:id/notes", d.admin.AddTenantNote)
		support.PUT("/tenants/:id/notes/:noteId", d.admin.UpdateTenantNote)
		support.PUT("/tenants/:id/notes/:noteId/pin", d.admin.SetTenantNotePinned)
		support.DELETE("/tenants/:id/notes/:noteId", d.admin.DeleteTenantNote)
		// Contactos con la empresa: los de correo y WhatsApp los registra la
		// plataforma al enviar; los de llamada y reunión se anotan a mano.
		support.POST("/tenants/:id/contacts", d.admin.AddTenantContact)

		// Hilo de cada entrada del expediente: comentarios y archivos. El
		// binario sube por /api/uploads; aquí solo se registra a qué entrada
		// pertenece y quién lo colgó.
		support.POST("/tenants/:id/events/:eventId/comments", d.companyThread.AddComment)
		support.PUT("/tenants/:id/comments/:commentId", d.companyThread.UpdateComment)
		support.DELETE("/tenants/:id/comments/:commentId", d.companyThread.DeleteComment)
		support.POST("/tenants/:id/events/:eventId/attachments", d.companyThread.AddAttachment)
		support.DELETE("/tenants/:id/attachments/:attId", d.companyThread.DeleteAttachment)
		// La descarga también vive aquí (y no en el grupo de solo lectura) para
		// que la vea el mismo público que puede escribir: superadmin y CS.
		support.GET("/tenants/:id/attachments/:attId/download", d.companyThread.DownloadAttachment)
	}

	// Gestión del expediente por el EMPLEADOR (solo su empresa). Reusa los
	// handlers del expediente; el acceso se acota con requireExpedienteOwnership.
	employer := api.Group("/employer")
	employer.Use(requireExpedienteOwnership(d.employmentSvc))
	{
		// Alta/baja de profesionales por el EMPLEADOR (auto-acotado a su empresa).
		// /employees es estático nuevo; DELETE /users/:id es nuevo a esa profundidad.
		employer.POST("/employees", d.admin.CreateEmployee)
		employer.GET("/import/template", d.admin.DownloadEmployerImportTemplate)
		employer.POST("/import/preview", d.admin.EmployerImportPreview)
		employer.POST("/import/execute", d.admin.EmployerImportExecute)
		employer.PUT("/users/:id", d.admin.UpdateEmployee)
		employer.POST("/users/:id/reset-password", d.admin.ResetEmployeePassword)
		employer.DELETE("/users/:id", d.admin.DeleteEmployee)

		// Acciones masivas del EMPLEADOR (auto-acotadas a su empresa). Rutas
		// estáticas sin :id -> requireExpedienteOwnership hace c.Next() y el
		// handler se acota por tenant.
		employer.POST("/bulk-assign-manager", d.admin.EmployerBulkAssignManager)
		employer.POST("/bulk-delete-users", d.admin.EmployerBulkDeleteUsers)

		employer.GET("/users/:id/employment", d.admin.GetMyCompanyEmployment)
		// Equipo a cargo de un manager (para el bloqueo "reasigna primero" al
		// degradar). Solo :id -> el gate hace c.Next() (auto-acotado por el handler).
		employer.GET("/users/:id/reports", d.admin.GetManagerReports)

		// Conjunto de managers por empleo (multi-manager) para el EMPLEADOR.
		// Reusa los MISMOS handlers admin (parsean :id + :empId); el acceso lo
		// acota requireExpedienteOwnership por :empId -> empresa dueña.
		employer.GET("/users/:id/employments/:empId/managers", d.admin.GetEmploymentManagers)
		employer.POST("/users/:id/employments/:empId/managers", d.admin.AddEmploymentManager)
		employer.DELETE("/users/:id/employments/:empId/managers/:managerId", d.admin.RemoveEmploymentManager)
		employer.PUT("/users/:id/employments/:empId/managers/:managerId/primary", d.admin.SetPrimaryEmploymentManager)

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
