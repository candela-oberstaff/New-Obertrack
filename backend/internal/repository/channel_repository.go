package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type ChannelRepository interface {
	// Channels
	GetChannelsByUser(userID uint) ([]models.Channel, error)
	GetChannelsByCompany(companyID uint) ([]models.Channel, error)
	GetChannel(id uint) (*models.Channel, error)
	GetChannelByNameAndType(name string, channelType models.ChannelType, tenantID uint) (*models.Channel, error)
	IsExplicitMember(channelID, userID uint) (bool, error)
	CreateChannel(channel *models.Channel) error
	UpdateChannel(channel *models.Channel, updates map[string]interface{}) error
	DeleteChannel(id uint) error

	// Members
	GetMembers(channelID uint) ([]models.User, error)
	GetMember(channelID, userID uint) (*models.ChannelMember, error)
	AddMember(member *models.ChannelMember) error
	RemoveMember(channelID, userID uint) error
	IsMember(channelID, userID uint) (bool, error)

	// Messages
	GetMessages(channelID uint, limit int, since *time.Time, beforeID uint) ([]models.ChannelMessage, error)
	GetMessage(id uint) (*models.ChannelMessage, error)
	CreateMessage(message *models.ChannelMessage) error
	UpdateMessage(message *models.ChannelMessage, updates map[string]interface{}) error
	DeleteMessage(id uint) error
	GetThreadReplies(parentID uint) ([]models.ChannelMessage, error)

	// Reactions
	GetReactions(messageID uint) ([]models.MessageReaction, error)
	AddReaction(reaction *models.MessageReaction) error
	RemoveReaction(messageID, userID uint, emoji string) error
	GetReaction(messageID, userID uint, emoji string) (*models.MessageReaction, error)

	// Pins
	GetPinnedMessages(channelID uint, since *time.Time) ([]models.ChannelMessage, error)
	PinMessage(messageID uint) error
	UnpinMessage(messageID uint) error

	// Starred
	StarMessage(starred *models.StarredMessage) error
	UnstarMessage(userID, messageID uint) error
	GetStarredMessages(userID uint) ([]models.StarredMessage, error)

	// Status
	GetUserStatus(userID uint) (*models.UserStatus, error)
	UpsertUserStatus(status *models.UserStatus) error
	GetUserStatuses(userIDs []uint) ([]models.UserStatus, error)

	// Unread
	GetUnreadCount(channelID, userID uint) (int64, error)
	MarkAsRead(channelID, userID uint) error
	GetTotalUnreadCount(userID uint) (int64, error)

	// Mentions
	CreateMention(mention *models.Mention) error

	// Users
	GetActiveUsers(tenantID uint, isSuperadmin bool) ([]models.User, error)
	FindUserByNamePrefix(name string, tenantID uint) (*models.User, error)

	// Custom
	SearchMessages(channelID uint, query string, limit int, since *time.Time) ([]models.ChannelMessage, error)
	FindManyMessagesByIDs(ids []uint) ([]models.ChannelMessage, error)
	CreateDMChannel(channel *models.Channel, memberIDs []uint) error

	// Support tickets
	CreateSupportTicket(ticket *models.SupportTicket) error
	GetSupportTicketByChannel(channelID uint) (*models.SupportTicket, error)
	GetSupportTicketsByChannelIDs(channelIDs []uint) ([]models.SupportTicket, error)
	GetPendingSupportTickets() ([]models.SupportTicket, error)
	GetAllSupportTickets() ([]models.SupportTicket, error)
	UpdateSupportTicket(ticket *models.SupportTicket, updates map[string]interface{}) error
}

type channelRepository struct {
	db *gorm.DB
}

func NewChannelRepository(db *gorm.DB) ChannelRepository {
	return &channelRepository{db: db}
}

func (r *channelRepository) GetDB() *gorm.DB {
	return r.db
}

// Channels

func (r *channelRepository) GetChannelsByUser(userID uint) ([]models.Channel, error) {
	var user models.User
	if err := r.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	tenantID := models.TenantForUser(&user)

	var channels []models.Channel
	err := r.db.Table("channels").
		Select("DISTINCT channels.*").
		Joins("LEFT JOIN channel_members ON channel_members.channel_id = channels.id").
		Where("channels.is_active = ?", true).
		Where(
			r.db.Where("channel_members.user_id = ?", userID).
				Or("channels.type = ? AND channels.tenant_id = ?", models.ChannelTypePublic, tenantID),
		).
		Order("channels.created_at DESC").
		Find(&channels).Error
	return channels, err
}

// GetChannelsByCompany returns every active channel (public, private and direct
// messages) that belongs to a given tenant. Used by superadmins to scope the chat
// to a single company so channels/DMs from different tenants never get mixed.
func (r *channelRepository) GetChannelsByCompany(companyID uint) ([]models.Channel, error) {
	var channels []models.Channel
	err := r.db.Table("channels").
		Where("channels.is_active = ?", true).
		Where("channels.tenant_id = ?", companyID).
		Order("channels.created_at DESC").
		Find(&channels).Error
	return channels, err
}

func (r *channelRepository) GetChannel(id uint) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.Preload("Members").First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepository) GetChannelByNameAndType(name string, channelType models.ChannelType, tenantID uint) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.Where("name = ? AND type = ? AND tenant_id = ?", name, channelType, tenantID).First(&channel).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepository) IsExplicitMember(channelID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ChannelMember{}).Where("channel_id = ? AND user_id = ?", channelID, userID).Count(&count).Error
	return count > 0, err
}

func (r *channelRepository) CreateChannel(channel *models.Channel) error {
	return r.db.Create(channel).Error
}

func (r *channelRepository) UpdateChannel(channel *models.Channel, updates map[string]interface{}) error {
	return r.db.Model(channel).Updates(updates).Error
}

func (r *channelRepository) DeleteChannel(id uint) error {
	return r.db.Model(&models.Channel{}).Where("id = ?", id).Update("is_active", false).Error
}

// Members

func (r *channelRepository) GetMembers(channelID uint) ([]models.User, error) {
	var members []models.User
	err := r.db.Joins("JOIN channel_members ON channel_members.user_id = users.id").
		Where("channel_members.channel_id = ?", channelID).
		Find(&members).Error
	return members, err
}

func (r *channelRepository) GetMember(channelID, userID uint) (*models.ChannelMember, error) {
	var member models.ChannelMember
	err := r.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *channelRepository) AddMember(member *models.ChannelMember) error {
	return r.db.Create(member).Error
}

func (r *channelRepository) RemoveMember(channelID, userID uint) error {
	return r.db.Where("channel_id = ? AND user_id = ?", channelID, userID).Delete(&models.ChannelMember{}).Error
}

func (r *channelRepository) IsMember(channelID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ChannelMember{}).Where("channel_id = ? AND user_id = ?", channelID, userID).Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	var channel models.Channel
	if err := r.db.First(&channel, channelID).Error; err != nil {
		return false, err
	}
	if channel.Type == models.ChannelTypePublic {
		var user models.User
		if err := r.db.First(&user, userID).Error; err != nil {
			return false, err
		}
		if models.TenantForUser(&user) == channel.TenantID {
			return true, nil
		}
	}

	return false, nil
}

// Messages

func (r *channelRepository) GetMessages(channelID uint, limit int, since *time.Time, beforeID uint) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	q := r.db.Where("channel_id = ? AND is_deleted = ? AND parent_id IS NULL", channelID, false)
	// Private channels: members only see history from the moment they joined.
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	// Cursor for paging into older history.
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	// Newest page first, then reversed to chronological order for the client.
	err := q.
		Preload("User").
		Preload("Reactions").
		Preload("Reactions.User").
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Reply counts in a single grouped query instead of one COUNT per message.
	if len(messages) > 0 {
		ids := make([]uint, len(messages))
		for i := range messages {
			ids[i] = messages[i].ID
		}

		type replyCountRow struct {
			ParentID uint
			Count    int64
		}
		var rows []replyCountRow
		r.db.Model(&models.ChannelMessage{}).
			Select("parent_id, COUNT(*) AS count").
			Where("parent_id IN ? AND is_deleted = ?", ids, false).
			Group("parent_id").
			Scan(&rows)

		counts := make(map[uint]int64, len(rows))
		for _, row := range rows {
			counts[row.ParentID] = row.Count
		}
		for i := range messages {
			messages[i].ReplyCount = int(counts[messages[i].ID])
		}
	}

	return messages, nil
}

func (r *channelRepository) GetMessage(id uint) (*models.ChannelMessage, error) {
	var message models.ChannelMessage
	err := r.db.Preload("User").Preload("Reactions").Preload("Reactions.User").First(&message, id).Error
	if err != nil {
		return nil, err
	}
	var count int64
	r.db.Model(&models.ChannelMessage{}).Where("parent_id = ? AND is_deleted = ?", message.ID, false).Count(&count)
	message.ReplyCount = int(count)
	return &message, nil
}

func (r *channelRepository) CreateMessage(message *models.ChannelMessage) error {
	return r.db.Create(message).Error
}

func (r *channelRepository) UpdateMessage(message *models.ChannelMessage, updates map[string]interface{}) error {
	return r.db.Model(message).Updates(updates).Error
}

func (r *channelRepository) DeleteMessage(id uint) error {
	return r.db.Model(&models.ChannelMessage{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (r *channelRepository) GetThreadReplies(parentID uint) ([]models.ChannelMessage, error) {
	var replies []models.ChannelMessage
	err := r.db.Where("parent_id = ? AND is_deleted = ?", parentID, false).
		Preload("User").
		Preload("Reactions").
		Preload("Reactions.User").
		Order("created_at ASC").
		Find(&replies).Error
	return replies, err
}

// Reactions

func (r *channelRepository) GetReactions(messageID uint) ([]models.MessageReaction, error) {
	var reactions []models.MessageReaction
	err := r.db.Where("message_id = ?", messageID).
		Preload("User").
		Find(&reactions).Error
	return reactions, err
}

func (r *channelRepository) AddReaction(reaction *models.MessageReaction) error {
	return r.db.Create(reaction).Error
}

func (r *channelRepository) RemoveReaction(messageID, userID uint, emoji string) error {
	return r.db.Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, emoji).
		Delete(&models.MessageReaction{}).Error
}

func (r *channelRepository) GetReaction(messageID, userID uint, emoji string) (*models.MessageReaction, error) {
	var reaction models.MessageReaction
	err := r.db.Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, emoji).
		First(&reaction).Error
	if err != nil {
		return nil, err
	}
	return &reaction, nil
}

// Pins

func (r *channelRepository) GetPinnedMessages(channelID uint, since *time.Time) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	q := r.db.Where("channel_id = ? AND is_pinned = ? AND is_deleted = ?", channelID, true, false)
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	err := q.
		Preload("User").
		Order("created_at DESC").
		Find(&messages).Error
	return messages, err
}

func (r *channelRepository) PinMessage(messageID uint) error {
	return r.db.Model(&models.ChannelMessage{}).Where("id = ?", messageID).Update("is_pinned", true).Error
}

func (r *channelRepository) UnpinMessage(messageID uint) error {
	return r.db.Model(&models.ChannelMessage{}).Where("id = ?", messageID).Update("is_pinned", false).Error
}

// Starred

func (r *channelRepository) StarMessage(starred *models.StarredMessage) error {
	return r.db.Create(starred).Error
}

func (r *channelRepository) UnstarMessage(userID, messageID uint) error {
	return r.db.Where("user_id = ? AND message_id = ?", userID, messageID).
		Delete(&models.StarredMessage{}).Error
}

func (r *channelRepository) GetStarredMessages(userID uint) ([]models.StarredMessage, error) {
	var starred []models.StarredMessage
	err := r.db.Where("user_id = ?", userID).Find(&starred).Error
	return starred, err
}

// Status

func (r *channelRepository) GetUserStatus(userID uint) (*models.UserStatus, error) {
	var status models.UserStatus
	err := r.db.Where("user_id = ?", userID).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *channelRepository) UpsertUserStatus(status *models.UserStatus) error {
	var existing models.UserStatus
	err := r.db.Where("user_id = ?", status.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(status).Error
	}
	return r.db.Model(&existing).Updates(map[string]interface{}{
		"status":    status.Status,
		"last_seen": status.LastSeen,
	}).Error
}

func (r *channelRepository) GetUserStatuses(userIDs []uint) ([]models.UserStatus, error) {
	var statuses []models.UserStatus
	err := r.db.Where("user_id IN ?", userIDs).Find(&statuses).Error
	return statuses, err
}

// Unread

func (r *channelRepository) GetUnreadCount(channelID, userID uint) (int64, error) {
	var member models.ChannelMember
	if err := r.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		return 0, err
	}

	query := r.db.Model(&models.ChannelMessage{}).Where("channel_id = ? AND user_id != ?", channelID, userID)
	// Unread starts at whichever is later: last read or the moment the user joined,
	// so being added to a channel with history doesn't inflate the badge.
	cutoff := member.JoinedAt
	if member.LastReadAt != nil && member.LastReadAt.After(cutoff) {
		cutoff = *member.LastReadAt
	}
	if !cutoff.IsZero() {
		query = query.Where("created_at > ?", cutoff)
	}

	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (r *channelRepository) MarkAsRead(channelID, userID uint) error {
	// Use the exact current time. A previous +1s buffer caused messages that
	// arrived within that second to never count as unread.
	return r.db.Model(&models.ChannelMember{}).
		Where("channel_id = ? AND user_id = ?", channelID, userID).
		Update("last_read_at", time.Now()).Error
}

func (r *channelRepository) GetTotalUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Table("channel_messages").
		Joins("JOIN channel_members ON channel_members.channel_id = channel_messages.channel_id").
		Where("channel_members.user_id = ?", userID).
		Where("channel_messages.user_id != ?", userID).
		Where("channel_messages.is_deleted = ?", false).
		Where("channel_messages.deleted_at IS NULL").
		Where("channel_members.last_read_at IS NULL OR channel_messages.created_at > channel_members.last_read_at").
		Where("channel_messages.created_at > channel_members.joined_at").
		Count(&count).Error
	return count, err
}

// Mentions

func (r *channelRepository) CreateMention(mention *models.Mention) error {
	return r.db.Create(mention).Error
}

// Users

func (r *channelRepository) GetActiveUsers(tenantID uint, isSuperadmin bool) ([]models.User, error) {
	var users []models.User
	q := r.db.Where("is_active = ?", true)

	// Exclude internal roles from the chat user list (superadmin, customer_success)
	if !isSuperadmin {
		q = q.Where("user_type NOT IN ?", []string{"superadmin", "customer_success"})
	}

	if isSuperadmin {
		// Superadmins can see everyone
	} else if tenantID > 0 {
		// Only return users that belong to the same company:
		// - The employer themselves (id = tenantID)
		// - Professionals under that employer (empleador_id = tenantID)
		q = q.Where("id = ? OR empleador_id = ?", tenantID, tenantID)
	} else {
		// No tenant context — only return the user themselves as a safety measure
		// to avoid leaking users from other companies.
		q = q.Where("1 = 0")
	}
	err := q.Find(&users).Error
	return users, err
}

func (r *channelRepository) FindUserByNamePrefix(name string, tenantID uint) (*models.User, error) {
	var user models.User
	q := r.db.Where("name ILIKE ?", name+"%")
	if tenantID > 0 {
		q = q.Where("id = ? OR empleador_id = ?", tenantID, tenantID)
	}
	err := q.First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *channelRepository) SearchMessages(channelID uint, query string, limit int, since *time.Time) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	q := r.db.Where("channel_id = ? AND is_deleted = ? AND content ILIKE ?", channelID, false, "%"+query+"%")
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	err := q.
		Preload("User").
		Preload("Reactions").
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *channelRepository) FindManyMessagesByIDs(ids []uint) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	err := r.db.Where("id IN ?", ids).
		Preload("User").
		Preload("Reactions").
		Order("created_at DESC").
		Find(&messages).Error
	return messages, err
}

// Support tickets

func (r *channelRepository) CreateSupportTicket(ticket *models.SupportTicket) error {
	return r.db.Create(ticket).Error
}

func (r *channelRepository) GetSupportTicketByChannel(channelID uint) (*models.SupportTicket, error) {
	var ticket models.SupportTicket
	err := r.db.Preload("Assignee").Preload("Requester").
		Where("channel_id = ?", channelID).First(&ticket).Error
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *channelRepository) GetPendingSupportTickets() ([]models.SupportTicket, error) {
	var tickets []models.SupportTicket
	err := r.db.Preload("Requester").
		Where("status = ? AND assigned_to IS NULL", models.SupportStatusOpen).
		Order("created_at ASC").Find(&tickets).Error
	return tickets, err
}

func (r *channelRepository) GetAllSupportTickets() ([]models.SupportTicket, error) {
	var tickets []models.SupportTicket
	err := r.db.Preload("Requester").Preload("Assignee").
		Order("updated_at DESC").Find(&tickets).Error
	return tickets, err
}

func (r *channelRepository) GetSupportTicketsByChannelIDs(channelIDs []uint) ([]models.SupportTicket, error) {
	var tickets []models.SupportTicket
	if len(channelIDs) == 0 {
		return tickets, nil
	}
	err := r.db.Preload("Assignee").Preload("Requester").Where("channel_id IN ?", channelIDs).Find(&tickets).Error
	return tickets, err
}

func (r *channelRepository) UpdateSupportTicket(ticket *models.SupportTicket, updates map[string]interface{}) error {
	return r.db.Model(ticket).Updates(updates).Error
}

func (r *channelRepository) CreateDMChannel(channel *models.Channel, memberIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(channel).Error; err != nil {
			return err
		}

		for _, userID := range memberIDs {
			member := &models.ChannelMember{
				ChannelID: channel.ID,
				UserID:    userID,
				Role:      "member",
				JoinedAt:  time.Now(),
			}
			if err := tx.Create(member).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
