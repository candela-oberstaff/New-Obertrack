package routes

import "github.com/gin-gonic/gin"

// registerMessagingRoutes wires real-time messaging: 1:1 chat, channels and
// in-app notifications.
func registerMessagingRoutes(api *gin.RouterGroup, d *deps) {
	chat := api.Group("/chat")
	{
		chat.GET("/messages", d.chat.GetMessages)
		chat.POST("/messages", d.chat.SendMessage)
	}

	notifications := api.Group("/notifications")
	{
		notifications.GET("", d.notification.GetNotifications)
		notifications.GET("/unread-count", d.notification.GetUnreadCount)
		notifications.POST("/:id/read", d.notification.MarkAsRead)
		notifications.POST("/read-all", d.notification.MarkAllAsRead)
	}

	channels := api.Group("/channels")
	{
		channels.GET("/unread/total", d.channel.GetTotalUnreadCount)
		channels.GET("", d.channel.GetChannels)
		channels.POST("", d.channel.CreateChannel)
		channels.GET("/all-users", d.channel.GetAllUsers)
		channels.GET("/:id", d.channel.GetChannel)
		channels.PUT("/:id", d.channel.UpdateChannel)
		channels.DELETE("/:id", d.channel.DeleteChannel)
		channels.GET("/:id/messages", d.channel.GetMessages)
		channels.POST("/:id/messages", d.channel.SendMessage)
		channels.PUT("/:id/messages/:messageId", d.channel.EditMessage)
		channels.DELETE("/:id/messages/:messageId", d.channel.DeleteMessage)
		channels.GET("/:id/messages/:messageId/reactions", d.channel.GetReactions)
		channels.POST("/:id/messages/:messageId/reactions", d.channel.AddReaction)
		channels.DELETE("/:id/messages/:messageId/reactions", d.channel.RemoveReaction)
		channels.GET("/:id/messages/:messageId/replies", d.channel.GetThreadReplies)
		channels.POST("/:id/messages/:messageId/replies", d.channel.SendThreadReply)
		channels.GET("/:id/members", d.channel.GetMembers)
		channels.POST("/:id/members", d.channel.AddMember)
		channels.DELETE("/:id/members", d.channel.RemoveMember)
		channels.POST("/:id/join", d.channel.JoinChannel)
		channels.POST("/:id/leave", d.channel.LeaveChannel)
		channels.POST("/:id/read", d.channel.MarkAsRead)
		channels.POST("/:id/pin/:messageId", d.channel.PinMessage)
		channels.POST("/:id/unpin/:messageId", d.channel.UnpinMessage)
		channels.GET("/:id/pinned", d.channel.GetPinnedMessages)
		channels.GET("/:id/search", d.channel.SearchMessages)
		channels.POST("/star/:messageId", d.channel.StarMessage)
		channels.DELETE("/star/:messageId", d.channel.UnstarMessage)
		channels.GET("/starred", d.channel.GetStarredMessages)
		channels.POST("/status", d.channel.UpdateStatus)
		channels.GET("/statuses", d.channel.GetStatuses)
		channels.POST("/dm", d.channel.CreateDirectMessage)
	}
}
