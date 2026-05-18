package service

import (
	"fmt"
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

var (
	ErrUnauthorized      = fmt.Errorf("unauthorized")
	ErrAlreadyMember     = fmt.Errorf("user is already a member")
	ErrSuperadminBlocked = fmt.Errorf("professionals cannot add superadmins")
	ErrChannelNotFound   = fmt.Errorf("channel not found")
	ErrUserNotFound      = fmt.Errorf("user not found")
)

type ChannelService interface {
	GetChannels(userID uint) ([]ChannelWithUnread, error)
	GetChannel(id uint) (*models.Channel, error)
	Create(userID uint, name, description, channelType string, memberIDs []uint) (*models.Channel, error)
	Update(id, userID uint, name, description string) (*models.Channel, error)
	Delete(id, userID uint, isSuperadmin bool) error
	AddMember(channelID, userID, memberToAdd uint) error
	RemoveMember(channelID, userID, memberToRemove uint) error
	Join(channelID, userID uint) error
	Leave(channelID, userID uint) error

	GetMessages(channelID, userID uint) ([]models.ChannelMessage, error)
	SendMessage(channelID, userID uint, content, attachment, fileName string, fileSize int64) (*models.ChannelMessage, []uint, error)
	EditMessage(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error)
	DeleteMessage(channelID, messageID, userID uint, isSuperadmin bool) error
	AddReaction(channelID, messageID, userID uint, emoji string) (*models.MessageReaction, error)
	RemoveReaction(channelID, messageID, userID uint, emoji string) error
	PinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error)
	UnpinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error)
	GetPinnedMessages(channelID, userID uint) ([]models.ChannelMessage, error)
	GetReactions(messageID uint) ([]models.MessageReaction, error)
	GetThreadReplies(channelID, messageID, userID uint) ([]models.ChannelMessage, error)
	SendThreadReply(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error)
	StarMessage(messageID, userID uint) error
	UnstarMessage(messageID, userID uint) error
	GetStarredMessages(userID uint) ([]models.ChannelMessage, error)
	SearchMessages(channelID, userID uint, query string) ([]models.ChannelMessage, error)
	CreateDirectMessage(userID, recipientID uint) (*DirectMessageResponse, error)
	UpdateStatus(userID uint, status string) (*models.UserStatus, error)
	GetStatuses(userIDs []uint) ([]models.UserStatus, error)
	GetTotalUnreadCount(userID uint) (int64, error)
	MarkAsRead(channelID, userID uint) error
	GetAllUsers() ([]models.User, error)
}

type ChannelWithUnread struct {
	ID          uint               `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Type        models.ChannelType `json:"type"`
	CreatedBy   uint               `json:"created_by"`
	IsActive    bool               `json:"is_active"`
	CreatedAt   time.Time          `json:"created_at"`
	UnreadCount int64              `json:"unread_count"`
	Recipient   *models.User       `json:"recipient,omitempty"`
}

type DirectMessageResponse struct {
	ID        uint               `json:"id"`
	Name      string             `json:"name"`
	Type      models.ChannelType `json:"type"`
	Recipient models.User        `json:"recipient"`
}

type channelService struct {
	repo        repository.ChannelRepository
	userRepo    repository.UserRepository
	notifSvc    NotificationService
}

func NewChannelService(repo repository.ChannelRepository, userRepo repository.UserRepository, notifSvc NotificationService) ChannelService {
	return &channelService{repo: repo, userRepo: userRepo, notifSvc: notifSvc}
}

func (s *channelService) GetChannels(userID uint) ([]ChannelWithUnread, error) {
	channels, err := s.repo.GetChannelsByUser(userID)
	if err != nil {
		return nil, err
	}

	var result []ChannelWithUnread
	for _, ch := range channels {
		unreadCount, _ := s.repo.GetUnreadCount(ch.ID, userID)
		
		var recipient *models.User
		if ch.Type == models.ChannelTypeDirect {
			members, err := s.repo.GetMembers(ch.ID)
			if err == nil {
				for _, m := range members {
					if m.ID != userID {
						recipient = &m
						log.Printf("[DEBUG] DM Channel %d: found recipient %s (ID %d)", ch.ID, m.Name, m.ID)
						break
					}
				}
			} else {
				log.Printf("[DEBUG] DM Channel %d: error getting members: %v", ch.ID, err)
			}
		}

		result = append(result, ChannelWithUnread{
			ID:          ch.ID,
			Name:        ch.Name,
			Description: ch.Description,
			Type:        ch.Type,
			CreatedBy:   ch.CreatedBy,
			IsActive:    ch.IsActive,
			CreatedAt:   ch.CreatedAt,
			UnreadCount: unreadCount,
			Recipient:   recipient,
		})
	}
	return result, nil
}

func (s *channelService) GetChannel(id uint) (*models.Channel, error) {
	return s.repo.GetChannel(id)
}

func (s *channelService) Create(userID uint, name, description, channelType string, memberIDs []uint) (*models.Channel, error) {
	cType := models.ChannelTypePublic
	if channelType == "private" {
		cType = models.ChannelTypePrivate
	}

	// Check if channel already exists
	if existing, _ := s.repo.GetChannelByNameAndType(name, cType); existing != nil {
		return existing, nil
	}

	channel := &models.Channel{
		Name:        name,
		Description: description,
		Type:        cType,
		CreatedBy:   userID,
		IsActive:    true,
	}

	if err := s.repo.CreateChannel(channel); err != nil {
		return nil, err
	}

	// Add creator as member
	s.repo.AddMember(&models.ChannelMember{
		ChannelID: channel.ID,
		UserID:    userID,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	})

	// Add other members
	if len(memberIDs) > 0 {
		var filteredIDs []uint
		for _, id := range memberIDs {
			if id != userID {
				filteredIDs = append(filteredIDs, id)
			}
		}
		for _, id := range filteredIDs {
			s.repo.AddMember(&models.ChannelMember{
				ChannelID: channel.ID,
				UserID:    id,
				Role:      "member",
				JoinedAt:  time.Now(),
				CreatedAt: time.Now(),
			})
		}
	}

	return s.repo.GetChannel(channel.ID)
}

func (s *channelService) Update(id, userID uint, name, description string) (*models.Channel, error) {
	channel, err := s.repo.GetChannel(id)
	if err != nil {
		return nil, err
	}

	if channel.CreatedBy != userID {
		return nil, fmt.Errorf("only channel creator can update")
	}

	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if description != "" {
		updates["description"] = description
	}

	if err := s.repo.UpdateChannel(channel, updates); err != nil {
		return nil, err
	}

	return s.repo.GetChannel(id)
}

func (s *channelService) Delete(id, userID uint, isSuperadmin bool) error {
	channel, err := s.repo.GetChannel(id)
	if err != nil {
		return err
	}

	if channel.CreatedBy != userID && !isSuperadmin {
		return fmt.Errorf("not authorized")
	}

	return s.repo.DeleteChannel(id)
}

func (s *channelService) AddMember(channelID, userID, memberToAdd uint) error {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return ErrChannelNotFound
	}

	if channel.Type == models.ChannelTypePrivate && channel.CreatedBy != userID {
		return ErrUnauthorized
	}

	// Rule: Professionals cannot add Superadmins
	actor, err := s.userRepo.GetByID(userID)
	if err == nil && actor.UserType == models.UserTypeProfessional {
		target, err := s.userRepo.GetByID(memberToAdd)
		if err == nil && (target.UserType == models.UserTypeSuperadmin || target.IsSuperadmin) {
			return ErrSuperadminBlocked
		}
	}

	if isMember, _ := s.repo.IsMember(channelID, memberToAdd); isMember {
		return ErrAlreadyMember
	}

	return s.repo.AddMember(&models.ChannelMember{
		ChannelID: channelID,
		UserID:    memberToAdd,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	})
}

func (s *channelService) RemoveMember(channelID, userID, memberToRemove uint) error {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return err
	}

	if channel.CreatedBy != userID && memberToRemove != userID {
		return fmt.Errorf("not authorized")
	}

	if memberToRemove == channel.CreatedBy {
		return fmt.Errorf("cannot remove channel creator")
	}

	return s.repo.RemoveMember(channelID, memberToRemove)
}

func (s *channelService) Join(channelID, userID uint) error {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return err
	}

	if channel.Type == models.ChannelTypePrivate {
		return fmt.Errorf("cannot join private channel directly")
	}

	if isMember, _ := s.repo.IsMember(channelID, userID); isMember {
		return fmt.Errorf("already a member")
	}

	return s.repo.AddMember(&models.ChannelMember{
		ChannelID: channelID,
		UserID:    userID,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	})
}

func (s *channelService) Leave(channelID, userID uint) error {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return err
	}

	if channel.CreatedBy == userID {
		return fmt.Errorf("channel creator cannot leave")
	}

	return s.repo.RemoveMember(channelID, userID)
}

