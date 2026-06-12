package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/models"
)

// registerWorkRoutes wires the productivity domain: boards, tasks and work hours.
// Cada grupo exige al menos "view" del módulo a nivel grupo; las mutaciones
// agregan "edit". Usuarios sin roles asignados no se restringen.
func registerWorkRoutes(api *gin.RouterGroup, d *deps) {
	tasksView := handlers.RequirePermission(d.rbacSvc, "tasks", models.PermissionView)
	tasksEdit := handlers.RequirePermission(d.rbacSvc, "tasks", models.PermissionEdit)

	boards := api.Group("/boards")
	boards.Use(tasksView)
	{
		boards.GET("", d.board.GetAll)
		boards.POST("", tasksEdit, d.board.Create)
		boards.GET("/public", d.board.GetPublicBoards)
		boards.POST("/:id/join", tasksEdit, d.board.JoinBoard)
		boards.GET("/:id", d.board.GetByID)
		boards.PUT("/:id", tasksEdit, d.board.Update)
		boards.DELETE("/:id", tasksEdit, d.board.Delete)
		boards.POST("/:id/phases", tasksEdit, d.board.AddPhase)
		boards.DELETE("/:id/phases/:phaseId", tasksEdit, d.board.RemovePhase)
		boards.PUT("/:id/phases/reorder", tasksEdit, d.board.ReorderPhases)
	}

	tasks := api.Group("/tasks")
	tasks.Use(tasksView)
	{
		tasks.GET("", d.task.GetAll)
		tasks.GET("/status-counts", d.task.GetBoardStatusCounts)
		tasks.POST("", tasksEdit, d.task.Create)
		tasks.GET("/:id", d.task.GetByID)
		tasks.PUT("/:id", tasksEdit, d.task.Update)
		tasks.DELETE("/:id", tasksEdit, d.task.Delete)
		tasks.POST("/:id/toggle-completion", tasksEdit, d.task.ToggleCompletion)
		tasks.POST("/:id/comments", tasksEdit, d.task.AddComment)
		tasks.POST("/:id/attachments", tasksEdit, d.task.AddAttachment)
		tasks.DELETE("/:id/attachments/:attachmentId", tasksEdit, d.task.DeleteAttachment)
	}

	// Permisos por rol (módulo "hours"): lecturas exigen "view", escrituras
	// exigen "edit". Usuarios sin roles asignados no se restringen.
	hoursView := handlers.RequirePermission(d.rbacSvc, "hours", models.PermissionView)
	hoursEdit := handlers.RequirePermission(d.rbacSvc, "hours", models.PermissionEdit)

	workHours := api.Group("/work-hours")
	{
		workHours.GET("", hoursView, d.workHour.GetAll)
		workHours.POST("", hoursEdit, d.workHour.Create)
		workHours.PUT("/:id", hoursEdit, d.workHour.Update)
		workHours.POST("/approve", hoursEdit, d.workHour.Approve)
		workHours.POST("/reject", hoursEdit, d.workHour.Reject)
		workHours.GET("/summary", hoursView, d.workHour.GetSummary)
		workHours.GET("/pending", hoursView, d.workHour.GetPending)
		workHours.POST("/send-report", hoursEdit, d.workHour.SendReport)
		workHours.GET("/report/pdf", hoursView, d.workHour.DownloadPDF)
		workHours.GET("/report/excel", hoursView, d.workHour.DownloadExcel)
	}
}
