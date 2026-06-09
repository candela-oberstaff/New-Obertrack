package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
)

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
	admin.Use(middleware.RequireSuperadmin())
	{
		admin.GET("/dashboard", d.admin.GetDashboard)
		admin.GET("/companies", d.admin.GetCompanies)
		admin.GET("/inactive-users", d.admin.GetInactiveUsers)
		admin.GET("/recent-activity", d.admin.GetRecentActivity)
		admin.GET("/audit-logs", d.audit.GetLogs)
		admin.GET("/absence-report", d.admin.GetAbsenceReport)
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

	// Global platform metrics (superadmin only).
	api.GET("/metrics", middleware.RequireSuperadmin(), d.metrics.GetGlobalMetrics)
}
