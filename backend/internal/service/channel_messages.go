package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// Messages

func (s *channelService) GetMessages(channelID, userID uint) ([]models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.GetMessages(channelID, 100)
}

func (s *channelService) SendMessage(channelID, userID uint, content, attachment, fileName string, fileSize int64) (*models.ChannelMessage, []uint, error) {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return nil, nil, err
	}

	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, nil, fmt.Errorf("you are not a member of this channel")
	}

	message := &models.ChannelMessage{
		ChannelID:  channelID,
		TenantID:   channel.TenantID,
		UserID:     userID,
		Content:    content,
		Attachment: attachment,
		FileName:   fileName,
		FileSize:   fileSize,
	}

	if err := s.repo.CreateMessage(message); err != nil {
		return nil, nil, err
	}

	preloadedMessage, _ := s.repo.GetMessage(message.ID)
	if preloadedMessage != nil {
		message = preloadedMessage
	}

	// Process mentions
	mentionedUserIDs := s.processMentions(message.ID, content, channelID)

	// Send notifications
	for _, mentionedUserID := range mentionedUserIDs {
		s.notifSvc.CreateNotification(mentionedUserID, "mention", "Te mencionaron en un canal", content, map[string]interface{}{
			"channel_id":  channelID,
			"message_id": message.ID,
		})
	}

	return message, mentionedUserIDs, nil
}

func (s *channelService) processMentions(messageID uint, content string, channelID uint) []uint {
	var mentionedUserIDs []uint

	// Resolve channel tenant for scoped user lookup
	var tenantID uint
	if ch, err := s.repo.GetChannel(channelID); err == nil {
		tenantID = ch.TenantID
	}

	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "@") {
			name := strings.TrimPrefix(word, "@")
			name = strings.TrimRight(name, ".,!?;:")
			if name == "" {
				continue
			}

			user, err := s.repo.FindUserByNamePrefix(name, tenantID)
			if err == nil && user != nil {
				if isMember, _ := s.repo.IsMember(channelID, user.ID); isMember {
					mentionedUserIDs = append(mentionedUserIDs, user.ID)
					s.repo.CreateMention(&models.Mention{
						MessageID: messageID,
						UserID:    user.ID,
						Notified:  true,
					})
				}
			}
		}
	}

	return mentionedUserIDs
}

func (s *channelService) EditMessage(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error) {
	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return nil, err
	}

	if message.UserID != userID {
		return nil, fmt.Errorf("you can only edit your own messages")
	}

	if err := s.repo.UpdateMessage(message, map[string]interface{}{
		"content":   content,
		"is_edited": true,
	}); err != nil {
		return nil, err
	}

	return s.repo.GetMessage(messageID)
}

func (s *channelService) DeleteMessage(channelID, messageID, userID uint, isSuperadmin bool) error {
	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return err
	}

	if message.UserID != userID && !isSuperadmin {
		return fmt.Errorf("you can only delete your own messages")
	}

	return s.repo.DeleteMessage(messageID)
}

func (s *channelService) AddReaction(channelID, messageID, userID uint, emoji string) (*models.MessageReaction, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	if _, err := s.repo.GetMessage(messageID); err != nil {
		return nil, err
	}

	if existing, _ := s.repo.GetReaction(messageID, userID, emoji); existing != nil {
		return nil, fmt.Errorf("reaction already exists")
	}

	reaction := &models.MessageReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	}

	if err := s.repo.AddReaction(reaction); err != nil {
		return nil, err
	}

	return reaction, nil
}

func (s *channelService) RemoveReaction(channelID, messageID, userID uint, emoji string) error {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.RemoveReaction(messageID, userID, emoji)
}

func (s *channelService) PinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	if _, err := s.repo.GetMessage(messageID); err != nil {
		return nil, err
	}

	if err := s.repo.PinMessage(messageID); err != nil {
		return nil, err
	}

	return s.repo.GetMessage(messageID)
}

func (s *channelService) UnpinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	if _, err := s.repo.GetMessage(messageID); err != nil {
		return nil, err
	}

	if err := s.repo.UnpinMessage(messageID); err != nil {
		return nil, err
	}

	return s.repo.GetMessage(messageID)
}

func (s *channelService) GetPinnedMessages(channelID, userID uint) ([]models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.GetPinnedMessages(channelID)
}

func (s *channelService) GetReactions(messageID, userID uint) ([]models.MessageReaction, error) {
	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return nil, err
	}

	if isMember, _ := s.repo.IsMember(message.ChannelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	return s.repo.GetReactions(messageID)
}

func (s *channelService) GetThreadReplies(channelID, messageID, userID uint) ([]models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.GetThreadReplies(messageID)
}

func (s *channelService) SendThreadReply(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	parentMessage, err := s.repo.GetMessage(messageID)
	if err != nil {
		return nil, err
	}

	if parentMessage.ParentID != nil {
		return nil, fmt.Errorf("cannot reply to a thread reply")
	}

	message := &models.ChannelMessage{
		ChannelID: channelID,
		TenantID:  parentMessage.TenantID,
		UserID:    userID,
		Content:   content,
		ParentID:  &parentMessage.ID,
	}

	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}

	return s.repo.GetMessage(message.ID)
}

func (s *channelService) StarMessage(messageID, userID uint) error {
	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return err
	}

	// Verify membership: user must belong to the message's channel
	if isMember, _ := s.repo.IsMember(message.ChannelID, userID); !isMember {
		return fmt.Errorf("you are not a member of this channel")
	}

	return s.repo.StarMessage(&models.StarredMessage{
		UserID:    userID,
		MessageID: messageID,
	})
}

func (s *channelService) UnstarMessage(messageID, userID uint) error {
	return s.repo.UnstarMessage(userID, messageID)
}

func (s *channelService) GetStarredMessages(userID uint) ([]models.ChannelMessage, error) {
	starred, err := s.repo.GetStarredMessages(userID)
	if err != nil {
		return nil, err
	}

	var messageIDs []uint
	for _, star := range starred {
		messageIDs = append(messageIDs, star.MessageID)
	}

	if len(messageIDs) == 0 {
		return []models.ChannelMessage{}, nil
	}

	messages, err := s.repo.FindManyMessagesByIDs(messageIDs)
	if err != nil {
		return nil, err
	}

	// Filter by tenant: only return messages belonging to the user's tenant
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	tenantID := models.TenantForUser(user)

	var filtered []models.ChannelMessage
	for _, msg := range messages {
		if msg.TenantID == tenantID || isSuperadminUser(user) {
			filtered = append(filtered, msg)
		}
	}
	return filtered, nil
}

func (s *channelService) SearchMessages(channelID, userID uint, query string) ([]models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	return s.repo.SearchMessages(channelID, query, 50)
}

func (s *channelService) CreateDirectMessage(userID, recipientID uint) (*DirectMessageResponse, error) {
	if userID == recipientID {
		return nil, fmt.Errorf("cannot create DM with yourself")
	}

	creator, _ := s.userRepo.GetByID(userID)
	recipient, err := s.userRepo.GetByID(recipientID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if !isSuperadminUser(creator) && !isSuperadminUser(recipient) && models.TenantForUser(creator) != models.TenantForUser(recipient) {
		return nil, ErrCrossTenant
	}

	dmName := fmt.Sprintf("DM-%d-%d", userID, recipientID)
	if userID > recipientID {
		dmName = fmt.Sprintf("DM-%d-%d", recipientID, userID)
	}

	dmChannel, err := s.repo.GetChannelByNameAndType(dmName, models.ChannelTypeDirect, models.TenantForUser(creator))
	if err == nil && dmChannel != nil {
		return s.buildDMResponse(dmChannel, recipientID)
	}

	newChannel := &models.Channel{
		Name:      dmName,
		Type:      models.ChannelTypeDirect,
		CreatedBy: userID,
		TenantID:  models.TenantForUser(creator),
		IsActive:  true,
	}

	if err := s.repo.CreateDMChannel(newChannel, []uint{userID, recipientID}); err != nil {
		return nil, err
	}

	dmChannel, _ = s.repo.GetChannel(newChannel.ID)
	return s.buildDMResponse(dmChannel, recipientID)
}

func (s *channelService) buildDMResponse(channel *models.Channel, recipientID uint) (*DirectMessageResponse, error) {
	members, _ := s.repo.GetMembers(channel.ID)
	var recipient models.User
	for _, m := range members {
		if m.ID == recipientID {
			recipient = m
			break
		}
	}

	return &DirectMessageResponse{
		ID:        channel.ID,
		Name:      channel.Name,
		Type:      channel.Type,
		Recipient: recipient,
	}, nil
}

func (s *channelService) UpdateStatus(userID uint, status string) (*models.UserStatus, error) {
	if status != "online" && status != "away" && status != "offline" {
		return nil, fmt.Errorf("invalid status")
	}

	userStatus := &models.UserStatus{
		UserID:   userID,
		Status:   status,
		LastSeen: time.Now(),
	}

	if err := s.repo.UpsertUserStatus(userStatus); err != nil {
		return nil, err
	}

	return s.repo.GetUserStatus(userID)
}

func (s *channelService) GetStatuses(userIDs []uint) ([]models.UserStatus, error) {
	return s.repo.GetUserStatuses(userIDs)
}

func (s *channelService) GetTotalUnreadCount(userID uint) (int64, error) {
	return s.repo.GetTotalUnreadCount(userID)
}

func (s *channelService) MarkAsRead(channelID, userID uint) error {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember {
		return fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.MarkAsRead(channelID, userID)
}

func (s *channelService) GetAllUsers(tenantID uint, isSuperadmin bool) ([]models.User, error) {
	return s.repo.GetActiveUsers(tenantID, isSuperadmin)
}
