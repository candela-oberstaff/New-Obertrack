package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
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

// channelAccessAllowed refleja las mismas reglas que el listado del sidebar
// (GetChannelsByUser): superadmin, miembro explícito, o canal público del
// mismo tenant. Los canales públicos solo registran miembros al crearse, así
// que a los usuarios que entraron a la empresa después se los une aquí en su
// primera interacción — sin esto ven el canal pero reciben 403 al usarlo, y
// además sin la fila de membresía no funcionan los no-leídos ni el last_read.
func (h *ChannelHandler) channelAccessAllowed(c *gin.Context, channel *models.Channel) bool {
	if middleware.IsSuperadmin(c) {
		return true
	}
	userID := middleware.GetUserID(c)
	for _, m := range channel.Members {
		if m.ID == userID {
			return true
		}
	}
	if channel.Type != models.ChannelTypePublic || channel.TenantID == 0 || channel.TenantID != middleware.GetTenantID(c) {
		return false
	}
	// Join revalida tipo de canal y tenant y es idempotente (no-op si ya es miembro).
	if err := h.svc.Join(channel.ID, userID); err != nil {
		return false
	}
	return true
}

func (h *ChannelHandler) GetChannels(c *gin.Context) {
	userID := middleware.GetUserID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	companyFilter := superadminCompanyFilter(c, isSuperadmin)

	channels, err := h.svc.GetChannels(userID, isSuperadmin, companyFilter)
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

	companyFilter := superadminCompanyFilter(c, middleware.IsSuperadmin(c))
	channel, err := h.svc.Create(userID, req.Name, req.Description, req.Type, req.MemberIDs, companyFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, channel)
}

func (h *ChannelHandler) GetChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	// authorize access: superadmin, miembro, o canal público del mismo tenant
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
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

	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, channel.Members)
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

	// Join es idempotente: si ya tiene la fila de membresía es un no-op exitoso
	// (channelAccessAllowed auto-une al ver un canal público), así que pulsar
	// "Unirse al canal" cuando ya es miembro no es un error.
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
	// ensure membership
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Optional cursor: return messages older than this message ID.
	beforeID, _ := strconv.ParseUint(c.Query("before"), 10, 32)

	messages, err := h.svc.GetMessages(uint(id), userID, uint(beforeID))
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
		FileType   string `json:"file_type"`
		TempID     string `json:"temp_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// membership check
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	message, mentioned, err := h.svc.SendMessage(uint(id), userID, req.Content, req.Attachment, req.FileName, req.FileSize, req.FileType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "message",
		ChannelID: uint(id),
		Content:   req.Content,
		UserID:    userID,
		TempID:    req.TempID,
		Data:      message,
	})

	// Continuidad del soporte: avisa al otro lado (responsable/solicitante) si
	// este canal es un ticket de soporte asignado. No aplica a otros canales.
	h.svc.NotifySupportReply(uint(id), userID, req.Content, mentioned)

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

	// membership check
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
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
	// membership check
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

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

	// membership check
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
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

	// membership check
	channel, err := h.svc.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}
	if !h.channelAccessAllowed(c, channel) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
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
	userID := middleware.GetUserID(c)

	reactions, err := h.svc.GetReactions(uint(messageID), userID)
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
		TempID  string `json:"temp_id"`
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

	// Echo temp_id back like the normal-message handler so the sender's other tabs
	// reconcile the optimistic thread reply by temp_id (not only by server id).
	h.hub.Broadcast(websocket.ChannelWSMessage{
		Type:      "thread_reply",
		ChannelID: uint(id),
		TempID:    req.TempID,
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

	statuses, err := h.svc.GetStatuses(userIDs, middleware.GetTenantID(c), middleware.IsSuperadmin(c))
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

	companyFilter := superadminCompanyFilter(c, middleware.IsSuperadmin(c))
	dm, err := h.svc.CreateDirectMessage(userID, req.RecipientID, companyFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dm)
}

// ContactSupport abre (o reutiliza) el canal de soporte del usuario con
// Customer Success y alerta a los agentes. Pensado para usuarios cliente.
func (h *ChannelHandler) ContactSupport(c *gin.Context) {
	userID := middleware.GetUserID(c)

	channel, err := h.svc.ContactSupport(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}

// ListSupportAgents devuelve los agentes (CS + superadmin) para el selector de reasignación.
func (h *ChannelHandler) ListSupportAgents(c *gin.Context) {
	agents, err := h.svc.ListSupportAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agents)
}

// ListPendingSupport: cola de solicitudes de soporte sin asignar (invitaciones).
func (h *ChannelHandler) ListPendingSupport(c *gin.Context) {
	userID := middleware.GetUserID(c)

	companyFilter := superadminCompanyFilter(c, middleware.IsSuperadmin(c))
	tickets, err := h.svc.ListPendingSupport(userID, companyFilter)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

// ClaimSupport: el agente toma el ticket (se autoasigna).
func (h *ChannelHandler) ClaimSupport(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	ticket, err := h.svc.ClaimSupportTicket(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ticket)
}

// AssignSupport: reasigna el ticket a otro agente.
func (h *ChannelHandler) AssignSupport(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	var req struct {
		AssigneeID uint `json:"assignee_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket, err := h.svc.AssignSupportTicket(uint(id), userID, req.AssigneeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ticket)
}

// ResolveSupport: marca el ticket como resuelto.
func (h *ChannelHandler) ResolveSupport(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	userID := middleware.GetUserID(c)

	ticket, err := h.svc.ResolveSupportTicket(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ticket)
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
	tenantID := middleware.GetTenantID(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	companyFilter := superadminCompanyFilter(c, isSuperadmin)
	users, err := h.svc.GetAllUsers(tenantID, isSuperadmin, companyFilter)
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
