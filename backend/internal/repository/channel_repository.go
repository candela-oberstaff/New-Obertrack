package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	GetUserStatuses(userIDs []uint, tenantID uint, isSuperadmin bool) ([]models.UserStatus, error)

	// Unread
	GetUnreadCount(channelID, userID uint) (int64, error)
	GetUnreadCounts(userID uint) ([]UnreadCount, error)
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
	CreateWithMembers(channel *models.Channel, members []models.ChannelMember) error

	// Support tickets
	CreateSupportTicket(ticket *models.SupportTicket) error
	GetSupportTicketByChannel(channelID uint) (*models.SupportTicket, error)
	GetSupportTicketsByChannelIDs(channelIDs []uint) ([]models.SupportTicket, error)
	GetPendingSupportTickets(companyID uint) ([]models.SupportTicket, error)
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
//
// PRODUCT DECISION (intentional, do NOT "fix"): superadmins can see and read EVERY
// channel of the tenant — including private channels and direct messages they are
// not a member of — for supervision purposes. There is no membership filter here on
// purpose. This mirrors the service-level gate (channel_messages.go s.isSuperadmin),
// which lets superadmins past the membership check. Removing this would break
// superadmin oversight; keep it.
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

// GetChannelByNameAndType intentionally does NOT filter on is_active: it backs the
// find-or-create flows (Create/CreateDirectMessage/ContactSupport), which must be
// able to find a previously soft-deleted channel with the same name/type/tenant so
// it can be REACTIVATED in place (B-7) instead of inserting a duplicate row that
// would violate the uniqueIndex idx_channel_name_type_tenant.
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
	// Idempotente ante carrera: el auto-join del cliente y el de channelAccessAllowed
	// pueden insertar el mismo (channel_id, user_id) casi a la vez; ON CONFLICT DO
	// NOTHING evita la violación de la PK compuesta (que se traducía en un HTTP 500
	// y, en el cliente, en "No se pudo unir al canal").
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(member).Error
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
	// Only load non-deleted messages: editing/pinning/reacting to an already
	// deleted message must fail cleanly with "record not found" instead of
	// mutating a tombstoned row.
	err := r.db.Preload("User").Preload("Reactions").Preload("Reactions.User").
		Where("id = ? AND is_deleted = ?", id, false).First(&message).Error
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
	// Idempotent under races (double-click): the unique index on
	// (message_id, user_id, emoji) makes the duplicate a no-op instead of an error.
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(reaction).Error
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
	// Idempotent under races (double-click): the unique index on
	// (user_id, message_id) makes the duplicate a no-op instead of an error.
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(starred).Error
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
	// Atomic upsert: a plain First-then-Create/Updates races under concurrent
	// requests for the same user (both pass First as "not found", both Create) and
	// violates the uniqueIndex on user_id → 500. OnConflict on user_id makes the
	// insert-or-update a single statement, matching AddReaction/StarMessage. We list
	// updated_at explicitly in DoUpdates because GORM does NOT auto-touch it on the
	// conflict path (autoUpdateTime only fires on Save/Updates, not OnConflict).
	status.UpdatedAt = time.Now()
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "last_seen", "updated_at"}),
	}).Create(status).Error
}

func (r *channelRepository) GetUserStatuses(userIDs []uint, tenantID uint, isSuperadmin bool) ([]models.UserStatus, error) {
	var statuses []models.UserStatus
	if len(userIDs) == 0 {
		return statuses, nil
	}
	// Superadmins can probe presence of anyone. For everyone else, scope to the
	// requester's tenant by joining users so presence of users in other companies
	// is never leaked: the user must be the employer (id = tenantID) or one of its
	// professionals (empleador_id = tenantID).
	q := r.db.Model(&models.UserStatus{}).
		Joins("JOIN users ON users.id = user_statuses.user_id").
		Where("user_statuses.user_id IN ?", userIDs)
	if !isSuperadmin {
		if tenantID == 0 {
			// No tenant context: leak nothing.
			return statuses, nil
		}
		q = q.Where("users.id = ? OR users.empleador_id = ?", tenantID, tenantID)
	}
	err := q.Find(&statuses).Error
	return statuses, err
}

// Unread

// unreadJoinCondition is the single source of truth for the "countable unread
// message" predicate shared by GetUnreadCounts and GetTotalUnreadCount: a message
// counts as unread for a member when it is not the member's own message, is not
// deleted, and was created strictly after the later of the member's joined_at and
// last_read_at (GREATEST/COALESCE mirrors cutoff = max(joined_at, last_read_at)).
// It is written as a JOIN condition on channel_messages ⋈ channel_members so both
// the per-channel grouped query and the global total stay byte-for-byte identical
// and can never diverge. The single ? placeholder binds the is_deleted = false arg.
const unreadJoinCondition = "channel_messages.channel_id = channel_members.channel_id" +
	" AND channel_messages.user_id != channel_members.user_id" +
	" AND channel_messages.is_deleted = ?" +
	" AND channel_messages.created_at > GREATEST(channel_members.joined_at, COALESCE(channel_members.last_read_at, channel_members.joined_at))"

func (r *channelRepository) GetUnreadCount(channelID, userID uint) (int64, error) {
	var member models.ChannelMember
	if err := r.db.Where("channel_id = ? AND user_id = ?", channelID, userID).First(&member).Error; err != nil {
		return 0, err
	}

	// Mirror GetTotalUnreadCount's "countable message" criteria so the per-channel
	// badges and the global total always add up: exclude deleted messages.
	query := r.db.Model(&models.ChannelMessage{}).
		Where("channel_id = ? AND user_id != ? AND is_deleted = ?", channelID, userID, false)
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

// UnreadCount is the per-channel unread total for a user, returned by
// GetUnreadCounts as a single grouped query.
type UnreadCount struct {
	ChannelID uint
	Count     int64
}

// GetUnreadCounts computes unread counts for ALL of the user's channels in one
// grouped query, using the EXACT same predicate as GetUnreadCount so the
// per-channel badges stay in sync: messages with user_id != me, is_deleted =
// false, and created_at strictly greater than the later of the user's joined_at
// and last_read_at on that channel's member row. Channels with no member row are
// simply absent from the result (their unread is 0). GREATEST/COALESCE mirror
// the Go-side cutoff = max(joined_at, last_read_at); a zero joined_at compares as
// the minimum timestamp, matching GetUnreadCount's "skip filter when cutoff is
// zero" branch (created_at is always greater than the zero time).
func (r *channelRepository) GetUnreadCounts(userID uint) ([]UnreadCount, error) {
	var rows []UnreadCount
	err := r.db.Table("channel_members").
		Select("channel_members.channel_id AS channel_id, COUNT(channel_messages.id) AS count").
		Joins("JOIN channel_messages ON "+unreadJoinCondition, false).
		Where("channel_members.user_id = ?", userID).
		Group("channel_members.channel_id").
		Scan(&rows).Error
	return rows, err
}

func (r *channelRepository) MarkAsRead(channelID, userID uint) error {
	// Use the exact current time. A previous +1s buffer caused messages that
	// arrived within that second to never count as unread.
	return r.db.Model(&models.ChannelMember{}).
		Where("channel_id = ? AND user_id = ?", channelID, userID).
		Update("last_read_at", time.Now()).Error
}

// GetTotalUnreadCount is the global unread badge. It MUST equal the sum of the
// per-channel counts from GetUnreadCounts, so it uses the exact same predicate
// (unreadJoinCondition) over channel_members ⋈ channel_messages: COUNT here ==
// SUM of the grouped COUNTs there because both count the identical rows.
func (r *channelRepository) GetTotalUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Table("channel_members").
		Joins("JOIN channel_messages ON "+unreadJoinCondition, false).
		Where("channel_members.user_id = ?", userID).
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

func (r *channelRepository) GetPendingSupportTickets(companyID uint) ([]models.SupportTicket, error) {
	var tickets []models.SupportTicket
	q := r.db.Preload("Requester").
		Where("status = ? AND assigned_to IS NULL", models.SupportStatusOpen)
	if companyID != 0 {
		q = q.Where("tenant_id = ?", companyID)
	}
	err := q.Order("created_at ASC").Find(&tickets).Error
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

// CreateWithMembers inserts the channel and all its members atomically. The
// members slice is created in a single batch insert (instead of N inserts) and
// the whole operation rolls back if anything fails. The caller must leave each
// member's ChannelID zero; it is filled in here once the channel has an ID.
func (r *channelRepository) CreateWithMembers(channel *models.Channel, members []models.ChannelMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if len(members) == 0 {
			return nil
		}
		for i := range members {
			members[i].ChannelID = channel.ID
		}
		if err := tx.Create(&members).Error; err != nil {
			return err
		}
		return nil
	})
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
