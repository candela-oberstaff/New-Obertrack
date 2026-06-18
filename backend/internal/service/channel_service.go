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
	SendMessage(channelID, userID uint, content, attachment, fileName string, fileSize int64, fileType string) (*models.ChannelMessage, []uint, error)
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
	ListPendingSupport(userID uint, companyFilter uint) ([]models.SupportTicket, error)
	ListSupportTicketsForBoard() ([]models.SupportTicket, error)
	NotifySupportReply(channelID, senderID uint, content string, alreadyNotified []uint)
	ClaimSupportTicket(channelID, userID uint) (*models.SupportTicket, error)
	AssignSupportTicket(channelID, actorID, assigneeID uint) (*models.SupportTicket, error)
	ResolveSupportTicket(channelID, actorID uint) (*models.SupportTicket, error)
	UpdateStatus(userID uint, status string) (*models.UserStatus, error)
	GetStatuses(userIDs []uint, tenantID uint, isSuperadmin bool) ([]models.UserStatus, error)
	GetTotalUnreadCount(userID uint) (int64, error)
	MarkAsRead(channelID, userID uint) error
	GetAllUsers(tenantID uint, isSuperadmin bool, companyFilter uint) ([]models.User, error)

	// SetBroadcaster cablea el difusor WebSocket de mensajes de sistema (soporte).
	SetBroadcaster(fn func(channelID uint, msg *models.ChannelMessage))

	// SetMembershipChangeHandler cablea el invalidador del caché de miembros, que
	// se invoca tras CADA mutación de membresía para que el hub WS refleje los
	// cambios al instante (sin esperar el TTL del caché).
	SetMembershipChangeHandler(fn func(channelID uint))
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
	// Participants se llena SOLO para DMs vistos por alguien que no participa
	// (supervisión de superadmin): no existe un "otro" único, así que se exponen
	// ambos miembros para que la UI muestre "A ↔ B" en vez de un nombre arbitrario.
	Participants []models.User `json:"participants,omitempty"`
	Support      *SupportInfo  `json:"support,omitempty"`
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
	// broadcast difunde por WebSocket un mensaje recién persistido a los miembros
	// del canal. Es un callback con tipos del dominio (sin acoplar el paquete
	// service al paquete websocket, evitando un ciclo de imports). Lo cablea
	// routes/deps.go apuntando al hub. Hoy solo lo usan los mensajes de SISTEMA
	// de soporte (tomó/asignó/resolvió), que no pasan por el handler HTTP que ya
	// difunde los mensajes normales de usuario. Puede ser nil (no-op) en tests.
	broadcast func(channelID uint, msg *models.ChannelMessage)
	// onMembershipChange invalida el caché de miembros (que vive en el paquete
	// routes y alimenta al MemberResolver del hub WS) tras CADA mutación de
	// membresía exitosa, para que un miembro recién añadido reciba broadcasts en
	// vivo de inmediato y uno removido deje de recibirlos al instante (sin esperar
	// el TTL de 30s). Mismo patrón de callback inyectado que broadcast, para no
	// acoplar service→routes. Lo cablea routes/deps.go. Puede ser nil (no-op) en
	// tests. NUNCA se llama en el path de envío de mensajes (no degradar el caché).
	onMembershipChange func(channelID uint)
}

func NewChannelService(repo repository.ChannelRepository, userRepo repository.UserRepository, notifSvc NotificationService) ChannelService {
	return &channelService{repo: repo, userRepo: userRepo, notifSvc: notifSvc}
}

// SetBroadcaster inyecta el difusor WebSocket usado por los mensajes de sistema
// de soporte. Se cablea en routes/deps.go tras construir el hub.
func (s *channelService) SetBroadcaster(fn func(channelID uint, msg *models.ChannelMessage)) {
	s.broadcast = fn
}

// SetMembershipChangeHandler inyecta el invalidador del caché de miembros usado
// tras cada mutación de membresía. Se cablea en routes/deps.go apuntando a
// memberCache.Invalidate.
func (s *channelService) SetMembershipChangeHandler(fn func(channelID uint)) {
	s.onMembershipChange = fn
}

// invalidateMembers notifica un cambio de membresía (nil-check). Llamar SOLO
// tras una mutación de membresía exitosa, nunca en el envío de mensajes.
func (s *channelService) invalidateMembers(channelID uint) {
	if s.onMembershipChange != nil {
		s.onMembershipChange(channelID)
	}
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

	// Unread counts for all of the user's channels in a single grouped query
	// (instead of one GetUnreadCount per channel). Same predicate, so badges match.
	unreadByChannel := make(map[uint]int64)
	if counts, err := s.repo.GetUnreadCounts(userID); err == nil {
		for _, c := range counts {
			unreadByChannel[c.ChannelID] = c.Count
		}
	} else {
		log.Printf("[ChannelService] error getting unread counts for user %d: %v", userID, err)
	}

	var result []ChannelWithUnread
	for _, ch := range channels {
		unreadCount := unreadByChannel[ch.ID]

		var recipient *models.User
		var participants []models.User
		if ch.Type == models.ChannelTypeDirect {
			members, err := s.repo.GetMembers(ch.ID)
			if err == nil {
				isMember := false
				for i := range members {
					if members[i].ID == userID {
						isMember = true
						break
					}
				}
				if isMember {
					// El viewer participa: muestra al OTRO miembro.
					for i := range members {
						if members[i].ID != userID {
							recipient = &members[i]
							break
						}
					}
				} else {
					// El viewer NO participa (supervisión de superadmin): no hay un
					// "otro" único, así que se exponen ambos participantes en vez de
					// elegir uno arbitrario (antes el bucle tomaba el primero).
					participants = members
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
			UnreadCount:  unreadCount,
			Recipient:    recipient,
			Participants: participants,
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

	// Check if channel already exists within the same tenant. If it was soft-deleted
	// (is_active=false), reactivate it in place (B-7) instead of returning a "ghost"
	// inactive channel or trying to insert a duplicate (which would violate the
	// uniqueIndex idx_channel_name_type_tenant). Recreating == reopening.
	if existing, _ := s.repo.GetChannelByNameAndType(name, cType, tenantID); existing != nil {
		reactivated, err := s.reactivateChannel(existing, userID)
		if err != nil {
			return nil, err
		}
		return reactivated, nil
	}

	channel := &models.Channel{
		Name:        name,
		Description: description,
		Type:        cType,
		CreatedBy:   userID,
		TenantID:    tenantID,
		IsActive:    true,
	}

	now := time.Now()
	// Creator is always an explicit member.
	members := []models.ChannelMember{{
		UserID:    userID,
		Role:      "member",
		JoinedAt:  now,
		CreatedAt: now,
	}}

	if cType == models.ChannelTypePublic && tenantID > 0 {
		// Slack-like: public channels automatically include everyone in the company.
		// Private channels stay invitation-only (explicit member_ids below).
		users, err := s.repo.GetActiveUsers(tenantID, false)
		if err == nil {
			for _, u := range users {
				if u.ID == userID {
					continue
				}
				members = append(members, models.ChannelMember{
					UserID:    u.ID,
					Role:      "member",
					JoinedAt:  now,
					CreatedAt: now,
				})
			}
		}
	} else if len(memberIDs) > 0 {
		for _, id := range memberIDs {
			if id == userID {
				continue
			}
			// Only add members that belong to the same tenant as the channel
			// (audit finding M-01). Superadmins are allowed cross-tenant.
			member, err := s.userRepo.GetByID(id)
			if err != nil || member == nil {
				continue
			}
			if !isSuperadminUser(member) && models.TenantForUser(member) != channel.TenantID {
				continue
			}
			members = append(members, models.ChannelMember{
				UserID:    id,
				Role:      "member",
				JoinedAt:  now,
				CreatedAt: now,
			})
		}
	}

	// Channel + all members in one transaction with a single batch insert; if any
	// member insert fails the whole creation rolls back (no orphaned channel).
	if err := s.repo.CreateWithMembers(channel, members); err != nil {
		return nil, err
	}
	s.invalidateMembers(channel.ID)

	return s.repo.GetChannel(channel.ID)
}

// reactivateChannel implements B-7: a find-or-create lookup may return a channel
// that was soft-deleted (is_active=false). Recreating it must REOPEN the existing
// row rather than hand back a "ghost" inactive channel (invisible in the sidebar,
// which filters is_active=true) or insert a duplicate (which would collide with the
// uniqueIndex idx_channel_name_type_tenant). If the channel is already active it is
// returned untouched. On reactivation we flip is_active=true, ensure the caller is
// still an explicit member (re-adding only if missing — existing memberships are
// reused), and invalidate the member cache so the WS hub sees it live. No migration
// is needed: this only mutates is_active on an existing row.
func (s *channelService) reactivateChannel(channel *models.Channel, userID uint) (*models.Channel, error) {
	if channel.IsActive {
		return channel, nil
	}

	if err := s.repo.UpdateChannel(channel, map[string]interface{}{"is_active": true}); err != nil {
		return nil, err
	}

	// Ensure the reactivating user is still a member. Reuse the existing membership
	// if present; otherwise re-add them (e.g. they had left before deletion).
	if isMember, _ := s.repo.IsExplicitMember(channel.ID, userID); !isMember {
		now := time.Now()
		if err := s.repo.AddMember(&models.ChannelMember{
			ChannelID: channel.ID,
			UserID:    userID,
			Role:      "member",
			JoinedAt:  now,
			CreatedAt: now,
		}); err != nil {
			return nil, err
		}
	}

	s.invalidateMembers(channel.ID)

	// Reload so the returned channel reflects is_active=true and fresh members.
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

	if err := s.repo.AddMember(&models.ChannelMember{
		ChannelID: channelID,
		UserID:    memberToAdd,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}
	s.invalidateMembers(channelID)
	return nil
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

	if err := s.repo.RemoveMember(channelID, memberToRemove); err != nil {
		return err
	}
	s.invalidateMembers(channelID)
	return nil
}

func (s *channelService) Join(channelID, userID uint) error {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return err
	}

	if channel.Type == models.ChannelTypePrivate {
		return fmt.Errorf("cannot join private channel directly")
	}

	// Idempotente: si YA existe la fila de membresía es un no-op exitoso. Usamos
	// IsExplicitMember (no IsMember) a propósito: en canales públicos IsMember
	// devuelve true para cualquiera del tenant aunque no tenga fila, lo que haría
	// que un usuario nuevo nunca creara su membresía (ni se invalidara el cache).
	if isMember, _ := s.repo.IsExplicitMember(channelID, userID); isMember {
		return nil
	}

	// Tenant isolation: user must belong to the same tenant as the channel.
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return ErrUserNotFound
	}
	if !isSuperadminUser(user) && models.TenantForUser(user) != channel.TenantID {
		return ErrCrossTenant
	}

	if err := s.repo.AddMember(&models.ChannelMember{
		ChannelID: channelID,
		UserID:    userID,
		Role:      "member",
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}
	s.invalidateMembers(channelID)
	return nil
}

func (s *channelService) Leave(channelID, userID uint) error {
	channel, err := s.authorizeChannelTenant(channelID, userID, false)
	if err != nil {
		return err
	}

	if channel.CreatedBy == userID {
		return fmt.Errorf("channel creator cannot leave")
	}

	if err := s.repo.RemoveMember(channelID, userID); err != nil {
		return err
	}
	s.invalidateMembers(channelID)
	return nil
}
