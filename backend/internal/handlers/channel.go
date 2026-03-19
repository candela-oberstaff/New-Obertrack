package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

type ChannelHandler struct {
	db  *gorm.DB
	hub *ChannelHub
}

type ChannelHub struct {
	clients    map[*websocket.Conn]uint
	broadcast  chan ChannelWSMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	db         *gorm.DB
}

type ChannelWSMessage struct {
	Type      string      `json:"type"`
	ChannelID uint        `json:"channel_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	UserID    uint        `json:"user_id,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

var channelUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewChannelHub(db *gorm.DB) *ChannelHub {
	return &ChannelHub{
		clients:    make(map[*websocket.Conn]uint),
		broadcast:  make(chan ChannelWSMessage),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		db:         db,
	}
}

func (h *ChannelHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = 0
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				err := client.WriteJSON(message)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
		}
	}
}

func NewChannelHandler(db *gorm.DB) *ChannelHandler {
	hub := NewChannelHub(db)
	go hub.Run()
	return &ChannelHandler{db: db, hub: hub}
}

func (h *ChannelHandler) GetChannels(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var channels []models.Channel
	if err := h.db.Where("is_active = ?", true).
		Joins("JOIN channel_members ON channel_members.channel_id = channels.id").
		Where("channel_members.user_id = ?", userID).
		Order("channels.created_at DESC").
		Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch channels"})
		return
	}

	type ChannelResponse struct {
		ID          uint               `json:"id"`
		Name        string             `json:"name"`
		Description string             `json:"description"`
		Type        models.ChannelType `json:"type"`
		CreatedBy   uint               `json:"created_by"`
		IsActive    bool               `json:"is_active"`
		CreatedAt   time.Time          `json:"created_at"`
		UnreadCount int64              `json:"unread_count"`
		Members     []models.User      `json:"members,omitempty"`
	}

	var response []ChannelResponse
	for _, ch := range channels {
		var memberCount int64
		h.db.Model(&models.ChannelMember{}).Where("channel_id = ?", ch.ID).Count(&memberCount)

		var unreadCount int64
		h.db.Model(&models.ChannelMessage{}).
			Where("channel_id = ?", ch.ID).
			Count(&unreadCount)

		response = append(response, ChannelResponse{
			ID:          ch.ID,
			Name:        ch.Name,
			Description: ch.Description,
			Type:        ch.Type,
			CreatedBy:   ch.CreatedBy,
			IsActive:    ch.IsActive,
			CreatedAt:   ch.CreatedAt,
			UnreadCount: unreadCount,
		})
	}

	c.JSON(http.StatusOK, response)
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

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel name is required"})
		return
	}

	channelType := models.ChannelTypePublic
	if req.Type == "private" {
		channelType = models.ChannelTypePrivate
	}

	channel := models.Channel{
		Name:        req.Name,
		Description: req.Description,
		Type:        channelType,
		CreatedBy:   userID,
		IsActive:    true,
	}

	if err := h.db.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create channel"})
		return
	}

	h.db.Model(&channel).Association("Members").Append(&models.User{ID: userID})

	if len(req.MemberIDs) > 0 {
		var members []models.User
		h.db.Find(&members, req.MemberIDs)
		h.db.Model(&channel).Association("Members").Append(&members)
	}

	c.JSON(http.StatusCreated, channel)
}

func (h *ChannelHandler) GetChannel(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var channel models.Channel
	if err := h.db.Preload("Members").First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	c.JSON(http.StatusOK, channel)
}

func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only channel creator can update"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if len(updates) > 0 {
		h.db.Model(&channel).Updates(updates)
	}

	h.db.Preload("Members").First(&channel, channelID)
	c.JSON(http.StatusOK, channel)
}

func (h *ChannelHandler) DeleteChannel(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.CreatedBy != userID && !isSuperadmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	h.db.Model(&channel).Update("is_active", false)
	c.JSON(http.StatusOK, gin.H{"message": "Channel deleted"})
}

func (h *ChannelHandler) AddMember(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.Type == models.ChannelTypePrivate && channel.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only channel creator can add members to private channels"})
		return
	}

	var member models.ChannelMember
	result := h.db.Where("channel_id = ? AND user_id = ?", channelID, req.UserID).First(&member)
	if result.Error == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already a member"})
		return
	}

	newMember := models.ChannelMember{
		ChannelID: uint(channelID),
		UserID:    req.UserID,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	}

	if err := h.db.Create(&newMember).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member added"})
}

func (h *ChannelHandler) RemoveMember(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.CreatedBy != userID && req.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if req.UserID == channel.CreatedBy {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove channel creator"})
		return
	}

	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, req.UserID).Delete(&models.ChannelMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}

func (h *ChannelHandler) GetMessages(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	var messages []models.ChannelMessage
	if err := h.db.Where("channel_id = ? AND is_deleted = ?", channelID, false).
		Preload("User").
		Order("created_at ASC").
		Limit(100).
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) processMentions(messageID uint, content string, channelID uint) []uint {
	var mentionedUserIDs []uint

	// Find all @name patterns
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "@") {
			name := strings.TrimPrefix(word, "@")
			// Remove trailing punctuation
			name = strings.TrimRight(name, ".,!?;:")
			if name != "" {
				var user models.User
				if err := h.db.Where("name LIKE ?", name+"%").First(&user).Error; err == nil {
					// Check if user is a member of the channel
					var member models.ChannelMember
					if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, user.ID).First(&member).Error; err == nil {
						mentionedUserIDs = append(mentionedUserIDs, user.ID)
						mention := models.Mention{
							MessageID: messageID,
							UserID:    user.ID,
							Notified:  true,
						}
						h.db.Create(&mention)
					}
				}
			}
		}
	}

	return mentionedUserIDs
}

func (h *ChannelHandler) SendMessage(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
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

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	message := models.ChannelMessage{
		ChannelID:  uint(channelID),
		UserID:     userID,
		Content:    req.Content,
		Attachment: req.Attachment,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
	}

	if err := h.db.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	h.db.Preload("User").First(&message, message.ID)

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "message",
		ChannelID: uint(channelID),
		Content:   req.Content,
		UserID:    userID,
		Timestamp: message.CreatedAt,
		Data:      message,
	}

	// Process @mentions
	mentionedUsers := h.processMentions(message.ID, req.Content, uint(channelID))

	// Send notifications to mentioned users
	for _, mentionedUserID := range mentionedUsers {
		notification := models.Notification{
			UserID:  mentionedUserID,
			Type:    "mention",
			Title:   "Te mencionaron en un canal",
			Message: req.Content,
			Data:    fmt.Sprintf(`{"channel_id": %d, "message_id": %d}`, channelID, message.ID),
		}
		h.db.Create(&notification)
	}

	c.JSON(http.StatusCreated, message)
}

func (h *ChannelHandler) HandleWebSocket(c *gin.Context) {
	userID := middleware.GetUserID(c)

	conn, err := channelUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.hub.register <- conn
	h.hub.clients[conn] = userID

	go func() {
		defer func() {
			h.hub.unregister <- conn
			conn.Close()
		}()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var msg ChannelWSMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}

			msg.UserID = userID
			h.hub.broadcast <- msg
		}
	}()
}

func (h *ChannelHandler) GetMembers(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var members []models.User
	if err := h.db.Joins("JOIN channel_members ON channel_members.user_id = users.id").
		Where("channel_members.channel_id = ?", channelID).
		Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

func (h *ChannelHandler) JoinChannel(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.Type == models.ChannelTypePrivate {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot join private channel directly"})
		return
	}

	var existing models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Already a member"})
		return
	}

	member := models.ChannelMember{
		ChannelID: uint(channelID),
		UserID:    userID,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	}

	if err := h.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined channel"})
}

func (h *ChannelHandler) LeaveChannel(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.CreatedBy == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel creator cannot leave"})
		return
	}

	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).Delete(&models.ChannelMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Left channel"})
}

func (h *ChannelHandler) PinMessage(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	var message models.ChannelMessage
	if err := h.db.Where("id = ? AND channel_id = ?", messageID, channelID).First(&message).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if err := h.db.Model(&message).Update("is_pinned", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pin message"})
		return
	}

	h.db.Preload("User").First(&message, message.ID)

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "message_pinned",
		ChannelID: uint(channelID),
		Data:      message,
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message pinned"})
}

func (h *ChannelHandler) UnpinMessage(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	var message models.ChannelMessage
	if err := h.db.Where("id = ? AND channel_id = ?", messageID, channelID).First(&message).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if err := h.db.Model(&message).Update("is_pinned", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpin message"})
		return
	}

	h.db.Preload("User").First(&message, message.ID)

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "message_unpinned",
		ChannelID: uint(channelID),
		Data:      message,
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message unpinned"})
}

func (h *ChannelHandler) GetPinnedMessages(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var channel models.Channel
	if err := h.db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	var messages []models.ChannelMessage
	if err := h.db.Where("channel_id = ? AND is_pinned = ? AND is_deleted = ?", channelID, true, false).
		Preload("User").
		Order("created_at DESC").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pinned messages"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) GetAllUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Where("is_active = ?", true).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *ChannelHandler) EditMessage(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var message models.ChannelMessage
	if err := h.db.Where("id = ? AND channel_id = ?", messageID, channelID).First(&message).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if message.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only edit your own messages"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.Model(&message).Updates(map[string]interface{}{
		"content":   req.Content,
		"is_edited": true,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to edit message"})
		return
	}

	h.db.Preload("User").First(&message, message.ID)

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "message_edited",
		ChannelID: uint(channelID),
		Data:      message,
	}

	c.JSON(http.StatusOK, message)
}

func (h *ChannelHandler) DeleteMessage(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)

	var message models.ChannelMessage
	if err := h.db.Where("id = ? AND channel_id = ?", messageID, channelID).First(&message).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if message.UserID != userID && !isSuperadmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own messages"})
		return
	}

	if err := h.db.Model(&message).Update("is_deleted", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "message_deleted",
		ChannelID: uint(channelID),
		Data:      map[string]interface{}{"id": messageID, "channel_id": channelID},
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

func (h *ChannelHandler) AddReaction(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var message models.ChannelMessage
	if err := h.db.Where("id = ? AND channel_id = ?", messageID, channelID).First(&message).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing models.MessageReaction
	if err := h.db.Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, req.Emoji).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reaction already exists"})
		return
	}

	reaction := models.MessageReaction{
		MessageID: uint(messageID),
		UserID:    userID,
		Emoji:     req.Emoji,
	}

	if err := h.db.Create(&reaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reaction"})
		return
	}

	h.db.Preload("User").First(&reaction, reaction.ID)

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "reaction_added",
		ChannelID: uint(channelID),
		Data:      map[string]interface{}{"message_id": messageID, "reaction": reaction},
	}

	c.JSON(http.StatusOK, reaction)
}

func (h *ChannelHandler) RemoveReaction(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, req.Emoji).Delete(&models.MessageReaction{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove reaction"})
		return
	}

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "reaction_removed",
		ChannelID: uint(channelID),
		Data:      map[string]interface{}{"message_id": messageID, "user_id": userID, "emoji": req.Emoji},
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction removed"})
}

func (h *ChannelHandler) GetReactions(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)

	var reactions []models.MessageReaction
	if err := h.db.Where("message_id = ?", messageID).
		Preload("User").
		Find(&reactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reactions"})
		return
	}

	c.JSON(http.StatusOK, reactions)
}

func (h *ChannelHandler) GetThreadReplies(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	var replies []models.ChannelMessage
	if err := h.db.Where("parent_id = ? AND is_deleted = ?", messageID, false).
		Preload("User").
		Preload("Reactions").
		Order("created_at ASC").
		Find(&replies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch replies"})
		return
	}

	c.JSON(http.StatusOK, replies)
}

func (h *ChannelHandler) SendThreadReply(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var parentMessage models.ChannelMessage
	if err := h.db.First(&parentMessage, messageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent message not found"})
		return
	}

	if parentMessage.ParentID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot reply to a thread reply"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message := models.ChannelMessage{
		ChannelID: uint(channelID),
		UserID:    userID,
		Content:   req.Content,
		ParentID:  &parentMessage.ID,
	}

	if err := h.db.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send reply"})
		return
	}

	h.db.Preload("User").First(&message, message.ID)

	h.hub.broadcast <- ChannelWSMessage{
		Type:      "thread_reply",
		ChannelID: uint(channelID),
		Data:      message,
	}

	c.JSON(http.StatusCreated, message)
}

func (h *ChannelHandler) StarMessage(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	var existing models.StarredMessage
	if err := h.db.Where("message_id = ? AND user_id = ?", messageID, userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message already starred"})
		return
	}

	starred := models.StarredMessage{
		UserID:    userID,
		MessageID: uint(messageID),
	}

	if err := h.db.Create(&starred).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to star message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message starred"})
}

func (h *ChannelHandler) UnstarMessage(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("messageId"), 10, 32)
	userID := middleware.GetUserID(c)

	if err := h.db.Where("message_id = ? AND user_id = ?", messageID, userID).Delete(&models.StarredMessage{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unstar message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message unstarred"})
}

func (h *ChannelHandler) GetStarredMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var starred []models.StarredMessage
	if err := h.db.Where("user_id = ?", userID).Find(&starred).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch starred messages"})
		return
	}

	var messageIDs []uint
	for _, s := range starred {
		messageIDs = append(messageIDs, s.MessageID)
	}

	var messages []models.ChannelMessage
	if len(messageIDs) > 0 {
		if err := h.db.Where("id IN ? AND is_deleted = ?", messageIDs, false).
			Preload("User").
			Preload("Reactions").
			Find(&messages).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
			return
		}
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChannelHandler) UpdateStatus(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		Status string `json:"status" binding:"required"` // online, away, offline
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Status != "online" && req.Status != "away" && req.Status != "offline" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	var status models.UserStatus
	if err := h.db.Where("user_id = ?", userID).First(&status).Error; err != nil {
		status = models.UserStatus{
			UserID:   userID,
			Status:   req.Status,
			LastSeen: time.Now(),
		}
		h.db.Create(&status)
	} else {
		h.db.Model(&status).Updates(map[string]interface{}{
			"status":    req.Status,
			"last_seen": time.Now(),
		})
	}

	c.JSON(http.StatusOK, status)
}

func (h *ChannelHandler) GetStatuses(c *gin.Context) {
	userIDsParam := c.Query("user_ids")
	if userIDsParam == "" {
		c.JSON(http.StatusOK, []models.UserStatus{})
		return
	}

	var userIDs []uint
	for _, s := range strings.Split(userIDsParam, ",") {
		if id, err := strconv.ParseUint(s, 10, 32); err == nil {
			userIDs = append(userIDs, uint(id))
		}
	}

	var statuses []models.UserStatus
	if err := h.db.Where("user_id IN ?", userIDs).Find(&statuses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch statuses"})
		return
	}

	c.JSON(http.StatusOK, statuses)
}

func (h *ChannelHandler) SearchMessages(c *gin.Context) {
	channelID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	var member models.ChannelMember
	if err := h.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this channel"})
		return
	}

	var messages []models.ChannelMessage
	if err := h.db.Where("channel_id = ? AND is_deleted = ? AND content ILIKE ?", channelID, false, "%"+query+"%").
		Preload("User").
		Preload("Reactions").
		Order("created_at DESC").
		Limit(50).
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search messages"})
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

	if req.RecipientID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create DM with yourself"})
		return
	}

	var recipient models.User
	if err := h.db.First(&recipient, req.RecipientID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recipient not found"})
		return
	}

	var dmChannel models.Channel
	dmName := fmt.Sprintf("DM-%d-%d", userID, req.RecipientID)
	if userID > req.RecipientID {
		dmName = fmt.Sprintf("DM-%d-%d", req.RecipientID, userID)
	}

	if err := h.db.Where("type = ? AND name = ?", models.ChannelTypeDirect, dmName).First(&dmChannel).Error; err != nil {
		dmChannel = models.Channel{
			Name:      dmName,
			Type:      models.ChannelTypeDirect,
			CreatedBy: userID,
			IsActive:  true,
		}
		if err := h.db.Create(&dmChannel).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create DM channel"})
			return
		}

		h.db.Model(&dmChannel).Association("Members").Append(&models.User{ID: userID})
		h.db.Model(&dmChannel).Association("Members").Append(&models.User{ID: req.RecipientID})
	}

	h.db.Preload("Members").First(&dmChannel, dmChannel.ID)

	type DMChannelResponse struct {
		ID          uint               `json:"id"`
		Name        string             `json:"name"`
		Type        models.ChannelType `json:"type"`
		Recipient   models.User        `json:"recipient"`
		UnreadCount int64              `json:"unread_count"`
	}

	response := DMChannelResponse{
		ID:          dmChannel.ID,
		Name:        dmChannel.Name,
		Type:        dmChannel.Type,
		UnreadCount: 0,
	}

	for _, m := range dmChannel.Members {
		if m.ID == req.RecipientID {
			response.Recipient = m
			break
		}
	}

	c.JSON(http.StatusOK, response)
}
