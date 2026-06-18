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
	ErrCrossTenant       = fmt.Errorf("user belongs to a different tenant")
)

func isSuperadminUser(user *models.User) bool {
	return user != nil && (user.IsSuperadmin || user.UserType == models.UserTypeSuperadmin)
}

type ChannelService interface {
	GetChannels(userID uint, isSuperadmin bool, companyFilter uint) ([]ChannelWithUnread, error)
	GetChannel(id uint) (*models.Channel, error)
	Create(userID uint, name, description, channelType string, memberIDs []uint, tenantOverride uint) (*models.Channel, error)
	Update(id, userID uint, name, description string) (*models.Channel, error)
	Delete(id, userID uint, isSuperadmin bool) error
	AddMember(channelID, userID, memberToAdd uint) error
	RemoveMember(channelID, userID, memberToRemove uint) error
	Join(channelID, userID uint) error
	Leave(channelID, userID uint) error

	GetMessages(channelID, userID, beforeID uint) ([]models.ChannelMessage, error)
	SendMessage(channelID, userID uint, content, attachment, fileName string, fileSize int64) (*models.ChannelMessage, []uint, error)
	EditMessage(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error)
	DeleteMessage(channelID, messageID, userID uint, isSuperadmin bool) error
	AddReaction(channelID, messageID, userID uint, emoji string) (*models.MessageReaction, error)
	RemoveReaction(channelID, messageID, userID uint, emoji string) error
	PinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error)
	UnpinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error)
	GetPinnedMessages(channelID, userID uint) ([]models.ChannelMessage, error)
	GetReactions(messageID, userID uint) ([]models.MessageReaction, error)
	GetThreadReplies(channelID, messageID, userID uint) ([]models.ChannelMessage, error)
	SendThreadReply(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error)
	StarMessage(messageID, userID uint) error
	UnstarMessage(messageID, userID uint) error
	GetStarredMessages(userID uint) ([]models.ChannelMessage, error)
	SearchMessages(channelID, userID uint, query string) ([]models.ChannelMessage, error)
	CreateDirectMessage(userID, recipientID uint, tenantOverride uint) (*DirectMessageResponse, error)
	ContactSupport(userID uint) (*ChannelWithUnread, error)
	ListSupportAgents() ([]models.User, error)
	ListPendingSupport(userID uint) ([]models.SupportTicket, error)
	ListSupportTicketsForBoard() ([]models.SupportTicket, error)
	NotifySupportReply(channelID, senderID uint, content string, alreadyNotified []uint)
	ClaimSupportTicket(channelID, userID uint) (*models.SupportTicket, error)
	AssignSupportTicket(channelID, actorID, assigneeID uint) (*models.SupportTicket, error)
	ResolveSupportTicket(channelID, actorID uint) (*models.SupportTicket, error)
	UpdateStatus(userID uint, status string) (*models.UserStatus, error)
	GetStatuses(userIDs []uint) ([]models.UserStatus, error)
	GetTotalUnreadCount(userID uint) (int64, error)
	MarkAsRead(channelID, userID uint) error
	GetAllUsers(tenantID uint, isSuperadmin bool, companyFilter uint) ([]models.User, error)
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
	Support     *SupportInfo       `json:"support,omitempty"`
}

// SupportInfo es el estado del ticket asociado a un canal de soporte, embebido en
// la lista de canales para que el frontend muestre responsable/estado y el panel
// de contexto (datos del solicitante) sin pedir nada más.
type SupportInfo struct {
	Status         string    `json:"status"`
	AssignedTo     *uint     `json:"assigned_to,omitempty"`
	AssigneeName   string    `json:"assignee_name,omitempty"`
	RequesterID    uint      `json:"requester_id"`
	RequesterName  string    `json:"requester_name,omitempty"`
	RequesterEmail string    `json:"requester_email,omitempty"`
	RequesterPhone string    `json:"requester_phone,omitempty"`
	CompanyName    string    `json:"company_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type DirectMessageResponse struct {
	ID        uint               `json:"id"`
	Name      string             `json:"name"`
	Type      models.ChannelType `json:"type"`
	Recipient models.User        `json:"recipient"`
}

type channelService struct {
	repo     repository.ChannelRepository
	userRepo repository.UserRepository
	notifSvc NotificationService
}

func NewChannelService(repo repository.ChannelRepository, userRepo repository.UserRepository, notifSvc NotificationService) ChannelService {
	return &channelService{repo: repo, userRepo: userRepo, notifSvc: notifSvc}
}

// authorizeChannelTenant loads the channel and verifies the user belongs to
// the same tenant. Returns the channel if authorized, or an error.
func (s *channelService) authorizeChannelTenant(channelID, userID uint, isSuperadmin bool) (*models.Channel, error) {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return nil, ErrChannelNotFound
	}
	if isSuperadmin {
		return channel, nil
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if !isSuperadminUser(user) && models.TenantForUser(user) != channel.TenantID {
		return nil, ErrCrossTenant
	}
	return channel, nil
}

func (s *channelService) GetChannels(userID uint, isSuperadmin bool, companyFilter uint) ([]ChannelWithUnread, error) {
	var channels []models.Channel
	var err error
	if isSuperadmin {
		// Superadmin must scope to a company; without it return nothing so we never
		// mix channels/DMs from different tenants in the sidebar.
		if companyFilter == 0 {
			return []ChannelWithUnread{}, nil
		}
		channels, err = s.repo.GetChannelsByCompany(companyFilter)
	} else {
		channels, err = s.repo.GetChannelsByUser(userID)
	}
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
						break
					}
				}
			} else {
				log.Printf("[ChannelService] error getting DM members for channel %d: %v", ch.ID, err)
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

	// Adjunta el estado del ticket a los canales de soporte (una sola consulta).
	ids := make([]uint, len(result))
	for i, r := range result {
		ids[i] = r.ID
	}
	if tickets, err := s.repo.GetSupportTicketsByChannelIDs(ids); err == nil {
		byChannel := make(map[uint]models.SupportTicket, len(tickets))
		for _, t := range tickets {
			byChannel[t.ChannelID] = t
		}
		for i := range result {
			if t, ok := byChannel[result[i].ID]; ok {
				info := &SupportInfo{
					Status:      t.Status,
					AssignedTo:  t.AssignedTo,
					RequesterID: t.RequesterID,
					CreatedAt:   t.CreatedAt,
				}
				if t.Assignee != nil {
					info.AssigneeName = t.Assignee.Name
				}
				if t.Requester != nil {
					info.RequesterName = t.Requester.Name
					info.RequesterEmail = t.Requester.Email
					info.RequesterPhone = t.Requester.PhoneNumber
					info.CompanyName = t.Requester.CompanyName
				}
				result[i].Support = info
			}
		}
	}

	return result, nil
}

func (s *channelService) GetChannel(id uint) (*models.Channel, error) {
	// Tenant/membership is enforced at the handler level.
	// For direct tenant-scoped access use authorizeChannelTenant instead.
	return s.repo.GetChannel(id)
}

func (s *channelService) Create(userID uint, name, description, channelType string, memberIDs []uint, tenantOverride uint) (*models.Channel, error) {
	cType := models.ChannelTypePublic
	if channelType == "private" {
		cType = models.ChannelTypePrivate
	}

	creator, _ := s.userRepo.GetByID(userID)
	tenantID := models.TenantForUser(creator)
	// Superadmins create channels scoped to the company they have selected, so the
	// channel is not orphaned (tenant 0) and shows up under that company's filter.
	if tenantOverride > 0 && isSuperadminUser(creator) {
		tenantID = tenantOverride
	}

	// Check if channel already exists within the same tenant
	if existing, _ := s.repo.GetChannelByNameAndType(name, cType, tenantID); existing != nil {
		return existing, nil
	}

	channel := &models.Channel{
		Name:        name,
		Description: description,
		Type:        cType,
		CreatedBy:   userID,
		TenantID:    tenantID,
		IsActive:    true,
	}

	if err := s.repo.CreateChannel(channel); err != nil {
		return nil, err
	}

	// Add creator as explicit member
	s.repo.AddMember(&models.ChannelMember{
		ChannelID: channel.ID,
		UserID:    userID,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	})

	if cType == models.ChannelTypePublic && tenantID > 0 {
		// Slack-like: public channels automatically include everyone in the company.
		// Private channels stay invitation-only (explicit member_ids below).
		users, err := s.repo.GetActiveUsers(tenantID, false)
		if err == nil {
			for _, u := range users {
				if u.ID == userID {
					continue
				}
				s.repo.AddMember(&models.ChannelMember{
					ChannelID: channel.ID,
					UserID:    u.ID,
					Role:      "member",
					JoinedAt:  time.Now(),
					CreatedAt: time.Now(),
				})
			}
		}
	} else if len(memberIDs) > 0 {
		var filteredIDs []uint
		for _, id := range memberIDs {
			if id != userID {
				filteredIDs = append(filteredIDs, id)
			}
		}
		for _, id := range filteredIDs {
			// Only add members that belong to the same tenant as the channel
			// (audit finding M-01). Superadmins are allowed cross-tenant.
			member, err := s.userRepo.GetByID(id)
			if err != nil || member == nil {
				continue
			}
			if !isSuperadminUser(member) && models.TenantForUser(member) != channel.TenantID {
				continue
			}
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
	channel, err := s.authorizeChannelTenant(id, userID, false)
	if err != nil {
		return nil, err
	}

	if channel.CreatedBy != userID {
		return nil, fmt.Errorf("only channel creator can update")
	}

	updates := map[string]any{}
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
	channel, err := s.authorizeChannelTenant(id, userID, isSuperadmin)
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

	target, err := s.userRepo.GetByID(memberToAdd)
	if err != nil {
		return ErrUserNotFound
	}

	// Rule: Professionals cannot add Superadmins
	actor, err := s.userRepo.GetByID(userID)
	if err == nil && actor.UserType == models.UserTypeProfessional && isSuperadminUser(target) {
		return ErrSuperadminBlocked
	}

	if !isSuperadminUser(target) && models.TenantForUser(target) != channel.TenantID {
		return ErrCrossTenant
	}

	if isMember, _ := s.repo.IsExplicitMember(channelID, memberToAdd); isMember {
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
	channel, err := s.authorizeChannelTenant(channelID, userID, false)
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

	// Tenant isolation: user must belong to the same tenant as the channel.
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return ErrUserNotFound
	}
	if !isSuperadminUser(user) && models.TenantForUser(user) != channel.TenantID {
		return ErrCrossTenant
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
	channel, err := s.authorizeChannelTenant(channelID, userID, false)
	if err != nil {
		return err
	}

	if channel.CreatedBy == userID {
		return fmt.Errorf("channel creator cannot leave")
	}

	return s.repo.RemoveMember(channelID, userID)
}
