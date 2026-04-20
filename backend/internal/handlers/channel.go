package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/websocket"
)

type ChannelHandler struct {
	svc  service.ChannelService
	hub  *websocket.ChannelHub
}

func NewChannelHandler(svc service.ChannelService, hub *websocket.ChannelHub) *ChannelHandler {
	return &ChannelHandler{svc: svc, hub: hub}
}

func (h *ChannelHandler) GetChannels(c *gin.Context) {
	userID := middleware.GetUserID(c)

	channels, err := h.svc.GetChannels(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch channels"})
		return
	}

	c.JSON(http.StatusOK, channels)
}

func (h *ChannelHandler) CreateChannel(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Type        string `json:"type" binding:"required"`
		MemberIDs   []uint `json:"member_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel, err := h.svc.Create(userID, req.Name, req.Description, req.Type, req.MemberIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, channel)
}

func (h *ChannelHandler) GetChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	c.JSON(http.StatusOK, channel)
}

func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel, err := h.svc.Update(uint(id), userID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}

func (h *ChannelHandler) DeleteChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	if err := h.svc.Delete(uint(id), userID, isSuperadmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Channel deleted"})
}

func (h *ChannelHandler) GetMembers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	members, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	c.JSON(http.StatusOK, members.Members)
}

func (h *ChannelHandler) AddMember(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.AddMember(uint(id), userID, req.UserID); err != nil {
		if err.Error() == "unauthorized" || err.Error() == "professionals cannot add superadmins" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else if err.Error() == "user is already a member" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else if err.Error() == "channel not found" || err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member added"})
}

func (h *ChannelHandler) RemoveMember(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.RemoveMember(uint(id), userID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}

func (h *ChannelHandler) JoinChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	if err := h.svc.Join(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined channel"})
}

func (h *ChannelHandler) LeaveChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	if err := h.svc.Leave(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Left channel"})
}

func (h *ChannelHandler) GetMessages(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	messages, err := h.svc.GetMessages(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) SendMessage(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Content    string `json:"content"`
		Attachment string `json:"attachment"`
		FileName   string `json:"file_name"`
		FileSize   int64  `json:"file_size"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message, _, err := h.svc.SendMessage(uint(id), userID, req.Content, req.Attachment, req.FileName, req.FileSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "message",
		ChannelID: uint(id),
		Content:   req.Content,
		UserID:    userID,
		Data:      message,
	})

	c.JSON(http.StatusCreated, message)
}

func (h *ChannelHandler) EditMessage(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message, err := h.svc.EditMessage(uint(id), uint(messageID), userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "message_edited",
		ChannelID: uint(id),
		Data:      message,
	})

	c.JSON(http.StatusOK, message)
}

func (h *ChannelHandler) DeleteMessage(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	if err := h.svc.DeleteMessage(uint(id), uint(messageID), userID, isSuperadmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "message_deleted",
		ChannelID: uint(id),
		Data:      map[string]interface{}{"id": messageID, "channel_id": id},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

func (h *ChannelHandler) AddReaction(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reaction, err := h.svc.AddReaction(uint(id), uint(messageID), userID, req.Emoji)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "reaction_added",
		ChannelID: uint(id),
		Data:      map[string]interface{}{"message_id": messageID, "reaction": reaction},
	})

	c.JSON(http.StatusOK, reaction)
}

func (h *ChannelHandler) RemoveReaction(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.RemoveReaction(uint(id), uint(messageID), userID, req.Emoji); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "reaction_removed",
		ChannelID: uint(id),
		Data:      map[string]interface{}{"message_id": messageID, "user_id": userID, "emoji": req.Emoji},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Reaction removed"})
}

func (h *ChannelHandler) GetReactions(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)

	reactions, err := h.svc.GetReactions(uint(messageID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reactions)
}

func (h *ChannelHandler) PinMessage(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	message, err := h.svc.PinMessage(uint(id), uint(messageID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "message_pinned",
		ChannelID: uint(id),
		Data:      message,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Message pinned"})
}

func (h *ChannelHandler) UnpinMessage(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	message, err := h.svc.UnpinMessage(uint(id), uint(messageID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "message_unpinned",
		ChannelID: uint(id),
		Data:      message,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Message unpinned"})
}

func (h *ChannelHandler) GetPinnedMessages(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	messages, err := h.svc.GetPinnedMessages(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) GetThreadReplies(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	replies, err := h.svc.GetThreadReplies(uint(id), uint(messageID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, replies)
}

func (h *ChannelHandler) SendThreadReply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message, err := h.svc.SendThreadReply(uint(id), uint(messageID), userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "thread_reply",
		ChannelID: uint(id),
		Data:      message,
	})

	c.JSON(http.StatusCreated, message)
}

func (h *ChannelHandler) StarMessage(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	if err := h.svc.StarMessage(uint(messageID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message starred"})
}

func (h *ChannelHandler) UnstarMessage(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	if err := h.svc.UnstarMessage(uint(messageID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message unstarred"})
}

func (h *ChannelHandler) GetStarredMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)

	messages, err := h.svc.GetStarredMessages(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) UpdateStatus(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := h.svc.UpdateStatus(userID, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

func (h *ChannelHandler) GetStatuses(c *gin.Context) {
	userIDsParam := c.Query("user_ids")
	if userIDsParam == "" {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	var userIDs []uint
	for _, s := range splitString(userIDsParam, ",") {
		if id, err := strconv.ParseUint(s, 10, 32); err == nil {
			userIDs = append(userIDs, uint(id))
		}
	}

	statuses, err := h.svc.GetStatuses(userIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statuses)
}

func (h *ChannelHandler) SearchMessages(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	messages, err := h.svc.SearchMessages(uint(id), userID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) CreateDirectMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		RecipientID uint `json:"recipient_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dm, err := h.svc.CreateDirectMessage(userID, req.RecipientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dm)
}

func (h *ChannelHandler) MarkAsRead(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	if err := h.svc.MarkAsRead(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}

func (h *ChannelHandler) HandleWebSocket(c *gin.Context) {
	userID := middleware.GetUserID(c)
	h.hub.HandleConnection(c.Writer, c.Request, userID)
}

func (h *ChannelHandler) GetAllUsers(c *gin.Context) {
	users, err := h.svc.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *ChannelHandler) GetTotalUnreadCount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	count, err := h.svc.GetTotalUnreadCount(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total_unread": count})
}

// Helper function

func splitString(s, sep string) []string {
	var result []string
	parts := strings.Split(s, sep)
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}
