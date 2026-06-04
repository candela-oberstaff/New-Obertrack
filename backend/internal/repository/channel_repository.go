package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type ChannelRepository interface {
	// Channels
	GetChannelsByUser(userID uint) ([]models.Channel, error)
	GetChannel(id uint) (*models.Channel, error)
	GetChannelByNameAndType(name string, channelType models.ChannelType) (*models.Channel, error)
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
	GetMessages(channelID uint, limit int) ([]models.ChannelMessage, error)
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
	GetPinnedMessages(channelID uint) ([]models.ChannelMessage, error)
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
	FindUserByNamePrefix(name string) (*models.User, error)

	// Custom
	SearchMessages(channelID uint, query string, limit int) ([]models.ChannelMessage, error)
	FindManyMessagesByIDs(ids []uint) ([]models.ChannelMessage, error)
	CreateDMChannel(channel *models.Channel, memberIDs []uint) error
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

func (r *channelRepository) GetChannel(id uint) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.Preload("Members").First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepository) GetChannelByNameAndType(name string, channelType models.ChannelType) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.Where("name = ? AND type = ?", name, channelType).First(&channel).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
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

func (r *channelRepository) GetMessages(channelID uint, limit int) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	err := r.db.Where("channel_id = ? AND is_deleted = ? AND parent_id IS NULL", channelID, false).
		Preload("User").
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	for i := range messages {
		var count int64
		r.db.Model(&models.ChannelMessage{}).Where("parent_id = ? AND is_deleted = ?", messages[i].ID, false).Count(&count)
		messages[i].ReplyCount = int(count)
	}

	return messages, nil
}

func (r *channelRepository) GetMessage(id uint) (*models.ChannelMessage, error) {
	var message models.ChannelMessage
	err := r.db.Preload("User").First(&message, id).Error
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

func (r *channelRepository) GetPinnedMessages(channelID uint) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	err := r.db.Where("channel_id = ? AND is_pinned = ? AND is_deleted = ?", channelID, true, false).
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
	if member.LastReadAt != nil {
		query = query.Where("created_at > ?", *member.LastReadAt)
	}

	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (r *channelRepository) MarkAsRead(channelID, userID uint) error {
	// Add a small 1s buffer to ensure precision doesn't keep messages at the same second unread
	return r.db.Model(&models.ChannelMember{}).
		Where("channel_id = ? AND user_id = ?", channelID, userID).
		Update("last_read_at", time.Now().Add(time.Second)).Error
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
	if !isSuperadmin && tenantID > 0 {
		// Only return users that belong to the same company:
		// - The employer themselves (id = tenantID)
		// - Professionals under that employer (empleador_id = tenantID)
		q = q.Where("id = ? OR empleador_id = ?", tenantID, tenantID)
	}
	err := q.Find(&users).Error
	return users, err
}

func (r *channelRepository) FindUserByNamePrefix(name string) (*models.User, error) {
	var user models.User
	err := r.db.Where("name ILIKE ?", name+"%").First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *channelRepository) SearchMessages(channelID uint, query string, limit int) ([]models.ChannelMessage, error) {
	var messages []models.ChannelMessage
	err := r.db.Where("channel_id = ? AND is_deleted = ? AND content ILIKE ?", channelID, false, "%"+query+"%").
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
