package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
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

// registerAccountRoutes wires identity & administration: the current user,
// user management and the superadmin panel.
func registerAccountRoutes(api *gin.RouterGroup, d *deps) {
	api.GET("/auth/me", d.auth.Me)

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
		admin.GET("/stats", d.admin.GetStats)
		admin.GET("/users", d.admin.GetAllUsers)
		admin.POST("/users", d.admin.CreateUser)
		admin.PUT("/users/:id", d.admin.UpdateUser)
		admin.DELETE("/users/:id", d.admin.DeleteUser)
		admin.POST("/users/:id/reset-password", d.admin.ResetPassword)

		admin.GET("/tenants", d.admin.GetTenants)
		admin.POST("/tenants", d.admin.CreateTenant)
		admin.GET("/tenants/:id", d.admin.GetTenant)
		admin.GET("/tenants/:id/employees", d.admin.GetTenantEmployees)
		admin.GET("/tenants/:id/activity", d.admin.GetTenantActivity)
		admin.POST("/tenants/:id/suspend", d.admin.SuspendTenant)
		admin.POST("/tenants/:id/activate", d.admin.ActivateTenant)
		admin.GET("/employees/:id/tracking", d.admin.GetEmployeeTracking)
	}

	// Bitácora de gestión CS: fuera del grupo /admin porque los customer
	// success deben poder ESCRIBIR seguimientos (allí solo tienen GET).
	followUps := api.Group("/follow-ups")
	followUps.Use(requireSupportInboxAccess())
	{
		followUps.GET("", d.admin.GetFollowUps)
		followUps.POST("", d.admin.CreateFollowUp)
	}

	// Diagnóstico técnico de plataforma: superadmins y analistas de IT.
	api.GET("/admin/audit-logs", middleware.RequirePlatformTech(), d.audit.GetLogs)
	api.GET("/metrics", middleware.RequirePlatformTech(), d.metrics.GetGlobalMetrics)
}
