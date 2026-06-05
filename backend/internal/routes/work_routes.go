package routes

import "github.com/gin-gonic/gin"

// registerWorkRoutes wires the productivity domain: boards, tasks and work hours.
func registerWorkRoutes(api *gin.RouterGroup, d *deps) {
	boards := api.Group("/boards")
	{
		boards.GET("", d.board.GetAll)
		boards.POST("", d.board.Create)
		boards.GET("/public", d.board.GetPublicBoards)
		boards.POST("/:id/join", d.board.JoinBoard)
		boards.GET("/:id", d.board.GetByID)
		boards.PUT("/:id", d.board.Update)
		boards.DELETE("/:id", d.board.Delete)
		boards.POST("/:id/phases", d.board.AddPhase)
		boards.DELETE("/:id/phases/:phaseId", d.board.RemovePhase)
		boards.PUT("/:id/phases/reorder", d.board.ReorderPhases)
	}

	tasks := api.Group("/tasks")
	{
		tasks.GET("", d.task.GetAll)
		tasks.POST("", d.task.Create)
		tasks.GET("/:id", d.task.GetByID)
		tasks.PUT("/:id", d.task.Update)
		tasks.DELETE("/:id", d.task.Delete)
		tasks.POST("/:id/toggle-completion", d.task.ToggleCompletion)
		tasks.POST("/:id/comments", d.task.AddComment)
		tasks.POST("/:id/attachments", d.task.AddAttachment)
		tasks.DELETE("/:id/attachments/:attachmentId", d.task.DeleteAttachment)
	}

	workHours := api.Group("/work-hours")
	{
		workHours.GET("", d.workHour.GetAll)
		workHours.POST("", d.workHour.Create)
		workHours.PUT("/:id", d.workHour.Update)
		workHours.POST("/approve", d.workHour.Approve)
		workHours.POST("/reject", d.workHour.Reject)
		workHours.GET("/summary", d.workHour.GetSummary)
		workHours.GET("/pending", d.workHour.GetPending)
		workHours.POST("/send-report", d.workHour.SendReport)
		workHours.GET("/report/pdf", d.workHour.DownloadPDF)
		workHours.GET("/report/excel", d.workHour.DownloadExcel)
	}
}
