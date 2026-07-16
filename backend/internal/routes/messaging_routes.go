package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/models"
)

// registerMessagingRoutes wires real-time messaging: 1:1 chat, channels and
// in-app notifications. El módulo "chat" exige "view" a nivel grupo y "edit"
// para enviar/modificar contenido; acciones de estado personal (leer, unirse,
// destacar) quedan en "view".
func registerMessagingRoutes(api *gin.RouterGroup, d *deps) {
	chatView := handlers.RequirePermission(d.rbacSvc, "chat", models.PermissionView)
	chatEdit := handlers.RequirePermission(d.rbacSvc, "chat", models.PermissionEdit)

	chat := api.Group("/chat")
	chat.Use(chatView)
	{
		chat.GET("/messages", d.chat.GetMessages)
		chat.POST("/messages", chatEdit, d.chat.SendMessage)
	}

	notifications := api.Group("/notifications")
	{
		notifications.GET("", d.notification.GetNotifications)
		notifications.GET("/unread-count", d.notification.GetUnreadCount)
		notifications.POST("/:id/read", d.notification.MarkAsRead)
		notifications.POST("/read-all", d.notification.MarkAllAsRead)
	}

	channels := api.Group("/channels")
	channels.Use(chatView)
	{
		channels.GET("/unread/total", d.channel.GetTotalUnreadCount)
		channels.GET("", d.channel.GetChannels)
		channels.GET("/archived", d.channel.GetArchivedChannels)
		channels.POST("", chatEdit, d.channel.CreateChannel)
		channels.GET("/all-users", d.channel.GetAllUsers)
		channels.GET("/:id", d.channel.GetChannel)
		channels.PUT("/:id", chatEdit, d.channel.UpdateChannel)
		channels.DELETE("/:id", chatEdit, d.channel.DeleteChannel)
		channels.GET("/:id/messages", d.channel.GetMessages)
		channels.POST("/:id/messages", chatEdit, d.channel.SendMessage)
		channels.PUT("/:id/messages/:messageId", chatEdit, d.channel.EditMessage)
		channels.DELETE("/:id/messages/:messageId", chatEdit, d.channel.DeleteMessage)
		channels.GET("/:id/messages/:messageId/reactions", d.channel.GetReactions)
		channels.POST("/:id/messages/:messageId/reactions", chatEdit, d.channel.AddReaction)
		channels.DELETE("/:id/messages/:messageId/reactions", chatEdit, d.channel.RemoveReaction)
		channels.GET("/:id/messages/:messageId/replies", d.channel.GetThreadReplies)
		channels.POST("/:id/messages/:messageId/replies", chatEdit, d.channel.SendThreadReply)
		channels.GET("/:id/members", d.channel.GetMembers)
		channels.POST("/:id/members", chatEdit, d.channel.AddMember)
		channels.DELETE("/:id/members", chatEdit, d.channel.RemoveMember)
		channels.PATCH("/:id/members/:userId/role", chatEdit, d.channel.UpdateMemberRole)
		channels.POST("/:id/join", d.channel.JoinChannel)
		channels.POST("/:id/leave", d.channel.LeaveChannel)
		channels.POST("/:id/hide", d.channel.HideChannel)
		channels.POST("/:id/unhide", d.channel.UnhideChannel)
		channels.POST("/:id/read", d.channel.MarkAsRead)
		channels.POST("/:id/pin/:messageId", chatEdit, d.channel.PinMessage)
		channels.POST("/:id/unpin/:messageId", chatEdit, d.channel.UnpinMessage)
		channels.GET("/:id/pinned", d.channel.GetPinnedMessages)
		channels.GET("/:id/search", d.channel.SearchMessages)
		channels.POST("/star/:messageId", d.channel.StarMessage)
		channels.DELETE("/star/:messageId", d.channel.UnstarMessage)
		channels.GET("/starred", d.channel.GetStarredMessages)
		channels.POST("/status", d.channel.UpdateStatus)
		channels.GET("/statuses", d.channel.GetStatuses)
		channels.POST("/dm", chatEdit, d.channel.CreateDirectMessage)
		channels.POST("/support", chatEdit, d.channel.ContactSupport)
		channels.GET("/support/agents", d.channel.ListSupportAgents)
		channels.GET("/support/pending", d.channel.ListPendingSupport)
		channels.GET("/support/mine", d.channel.ListMySupportTickets)
		channels.POST("/support/tickets/:id/claim", chatEdit, d.channel.ClaimSupport)
		channels.POST("/support/tickets/:id/reopen", chatEdit, d.channel.ReopenSupport)
		channels.POST("/support/tickets/:id/assign", chatEdit, d.channel.AssignSupport)
		channels.POST("/support/tickets/:id/resolve", chatEdit, d.channel.ResolveSupport)
	}
}
