package service

import (
	"fmt"
	"log"
	"strings"
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
	// ErrChannelNotDeletable se devuelve al intentar eliminar un canal que no es
	// "normal": los DMs (type=direct) y los canales de soporte (type=private cuyo
	// nombre empieza por "Soporte · ") no se borran (decisión de producto A-1).
	ErrChannelNotDeletable = fmt.Errorf("channel cannot be deleted")
	// ErrDuplicateChannelName se devuelve al renombrar un canal a un nombre que ya
	// usa OTRO canal con el mismo (name, type, tenant), evitando el 500 que daría
	// el índice único idx_channel_name_type_tenant (A-2).
	ErrDuplicateChannelName = fmt.Errorf("channel name already in use")
	// ErrInvalidChannelType se devuelve al intentar cambiar el tipo de un canal a un
	// valor no permitido: solo se admite "public"<->"private". Cambiar a/desde
	// "direct" (los DMs no cambian de tipo) o el tipo de un canal de soporte está
	// prohibido, igual que cualquier valor desconocido (→ 400 en el handler).
	ErrInvalidChannelType = fmt.Errorf("invalid channel type change")
)

func isSuperadminUser(user *models.User) bool {
	return user != nil && (user.IsSuperadmin || user.UserType == models.UserTypeSuperadmin)
}

// supportChannelPrefix es el prefijo de nombre que distingue a un canal de
// soporte. Debe mantenerse en sincronía con la construcción del nombre en
// channel_messages.go ("Soporte · %s #%d").
const supportChannelPrefix = "Soporte · "

// isSupportChannel indica si el canal es un canal de soporte (private cuyo
// nombre empieza por el prefijo de soporte). Único punto de verdad para esta
// detección en el paquete service.
func isSupportChannel(channel *models.Channel) bool {
	return channel != nil &&
		channel.Type == models.ChannelTypePrivate &&
		strings.HasPrefix(channel.Name, supportChannelPrefix)
}

// canManageChannel decide si userID puede GESTIONAR el canal (editar, eliminar,
// añadir/quitar miembros). Tienen poderes de gestión (decisión de producto B):
//   - el superadmin (supervisión global),
//   - el CREADOR del canal,
//   - cualquier MIEMBRO de ESE canal con Role=="admin".
//
// CRÍTICO: el chequeo de admin es POR CANAL (GetMember(channel.ID, userID)), no
// global: un admin del canal X no debe poder gestionar el canal Y. Si no hay fila
// de miembro (o falla la consulta) el usuario no es admin de este canal.
func (s *channelService) canManageChannel(channel *models.Channel, userID uint, isSuperadmin bool) bool {
	if channel == nil {
		return false
	}
	if isSuperadmin {
		return true
	}
	if channel.CreatedBy == userID {
		return true
	}
	member, err := s.repo.GetMember(channel.ID, userID)
	if err != nil || member == nil {
		return false
	}
	return member.Role == "admin"
}

type ChannelService interface {
	GetChannels(userID uint, isSuperadmin bool, companyFilter uint) ([]ChannelWithUnread, error)
	GetChannel(id uint) (*models.Channel, error)
	// GetMembersWithRole devuelve los miembros del canal con su rol (admin|member),
	// uniendo User + ChannelMember.Role para el contrato del frontend.
	GetMembersWithRole(channelID uint) ([]ChannelMemberDTO, error)
	Create(userID uint, name, description, channelType string, memberIDs []uint, tenantOverride uint) (*models.Channel, error)
	// Update edita nombre/descripcion y, opcionalmente, el tipo de canal
	// (publico<->privado). channelType vacio = sin cambio de tipo.
	Update(id, userID uint, name, description, channelType string, isSuperadmin bool) (*models.Channel, error)
	Delete(id, userID uint, isSuperadmin bool) error
	AddMember(channelID, userID, memberToAdd uint, isSuperadmin bool) error
	RemoveMember(channelID, userID, memberToRemove uint, isSuperadmin bool) error
	// SetMemberRole promueve/degrada a un miembro (admin|member). Solo el CREADOR
	// del canal o un superadmin pueden hacerlo (un admin normal NO).
	SetMemberRole(channelID, actorID, targetID uint, role string, isSuperadmin bool) error
	Join(channelID, userID uint) error
	Leave(channelID, userID uint) error
	// IsExplicitMember indica si el usuario tiene una fila de membresía explícita en
	// el canal (channel_members), sin el auto-join de canales públicos. Lo usa el
	// handler para impedir que un NO-miembro (incluido un superadmin que solo
	// supervisa) escriba en DMs o canales privados ajenos.
	IsExplicitMember(channelID, userID uint) (bool, error)

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
	ContactSupport(userID uint, subject, message, priority, module string, forceNew bool) (*ChannelWithUnread, error)
	ReopenSupportTicket(ticketID, actorID uint) (*models.SupportTicket, error)
	ListSupportAgents() ([]models.User, error)
	ListPendingSupport(userID uint, companyFilter uint) ([]models.SupportTicket, error)
	ListSupportTicketsForBoard() ([]models.SupportTicket, error)
	ListMySupportTickets(userID uint) ([]MySupportTicket, error)
	NotifySupportReply(channelID, senderID uint, content string, alreadyNotified []uint)
	ClaimSupportTicket(ticketID, userID uint) (*models.SupportTicket, error)
	AssignSupportTicket(ticketID, actorID, assigneeID uint) (*models.SupportTicket, error)
	ResolveSupportTicket(ticketID, actorID uint) (*models.SupportTicket, error)
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

	SetSupportNotifier(n *SupportNotifier)
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
	// Supervised marca los canales que el solicitante está AUDITANDO sin ser
	// miembro: true cuando NO tiene fila de membresía explícita Y el canal es
	// direct o private (DMs y privados ajenos que un superadmin supervisa). Es
	// false para canales public y para cualquier canal donde el usuario SÍ es
	// miembro. El frontend lo usa para agrupar las conversaciones supervisadas.
	Supervised bool `json:"supervised,omitempty"`
}

// SupportInfo es el estado del ticket asociado a un canal de soporte, embebido en
// la lista de canales para que el frontend muestre responsable/estado y el panel
// de contexto (datos del solicitante) sin pedir nada más.
type SupportInfo struct {
	TicketID       uint      `json:"ticket_id"`
	Subject        string    `json:"subject,omitempty"`
	Priority       string    `json:"priority,omitempty"`
	Module         string    `json:"module,omitempty"`
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

// ChannelMemberDTO es la forma que consume el frontend para la lista de miembros
// de un canal: datos del usuario + su rol en ESE canal (admin|member). Contrato
// compartido con el frontend: GET /channels/:id/members → [{id,name,email,role}].
type ChannelMemberDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
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
	supportNtfy *SupportNotifier
}

func NewChannelService(repo repository.ChannelRepository, userRepo repository.UserRepository, notifSvc NotificationService) ChannelService {
	return &channelService{repo: repo, userRepo: userRepo, notifSvc: notifSvc}
}

func (s *channelService) SetSupportNotifier(n *SupportNotifier) {
	s.supportNtfy = n
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

	// Conjunto de canales donde el usuario es miembro explícito, resuelto en UNA
	// sola consulta (no N): así sabemos por canal si está supervisando (auditando
	// sin ser miembro) o participando, sin un IsExplicitMember por canal. Para un
	// usuario normal GetChannelsByUser solo devuelve sus canales, así que estará en
	// memberSet para todos → Supervised siempre false (no le afecta).
	memberSet := make(map[uint]bool)
	if memberIDs, err := s.repo.GetMemberChannelIDs(userID); err == nil {
		for _, id := range memberIDs {
			memberSet[id] = true
		}
	} else {
		log.Printf("[ChannelService] error getting member channel ids for user %d: %v", userID, err)
	}

	var result []ChannelWithUnread
	for _, ch := range channels {
		unreadCount := unreadByChannel[ch.ID]

		// Supervised: el usuario NO es miembro explícito y el canal es direct o
		// private. Coherente con el bloque DM de abajo (cuando no es miembro se
		// llena Participants, y aquí Supervised será true; cuando sí es miembro se
		// llena Recipient y Supervised será false). Para public siempre false.
		// Los canales de SOPORTE se excluyen: tienen su propio flujo (tomar/
		// reasignar/resolver) y el agente que atiende debe poder escribir, no es
		// "supervisión".
		chCopy := ch
		isMemberOfChannel := memberSet[ch.ID]
		supervised := !isMemberOfChannel &&
			(ch.Type == models.ChannelTypeDirect || ch.Type == models.ChannelTypePrivate) &&
			!isSupportChannel(&chCopy)

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
			ID:           ch.ID,
			Name:         ch.Name,
			Description:  ch.Description,
			Type:         ch.Type,
			CreatedBy:    ch.CreatedBy,
			IsActive:     ch.IsActive,
			CreatedAt:    ch.CreatedAt,
			UnreadCount:  unreadCount,
			Recipient:    recipient,
			Participants: participants,
			Supervised:   supervised,
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
			cur, ok := byChannel[t.ChannelID]
			if !ok {
				byChannel[t.ChannelID] = t
				continue
			}
			curResolved := cur.Status == models.SupportStatusResolved
			tResolved := t.Status == models.SupportStatusResolved
			if curResolved != tResolved {
				if curResolved {
					byChannel[t.ChannelID] = t
				}
				continue
			}
			if t.UpdatedAt.After(cur.UpdatedAt) {
				byChannel[t.ChannelID] = t
			}
		}
		for i := range result {
			if t, ok := byChannel[result[i].ID]; ok {
				info := &SupportInfo{
					TicketID:    t.ID,
					Subject:     t.Subject,
					Priority:    t.Priority,
					Module:      t.Module,
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

// IsExplicitMember reporta si el usuario tiene fila de membresía explícita en el
// canal (delegando en el repo). No aplica el auto-join de públicos: un usuario sin
// fila devuelve false aunque vea el canal público. El handler lo usa para bloquear
// el envío de contenido por parte de no-miembros en DMs/privados (supervisión).
func (s *channelService) IsExplicitMember(channelID, userID uint) (bool, error) {
	return s.repo.IsExplicitMember(channelID, userID)
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
		if existing.IsActive {
			return nil, ErrDuplicateChannelName
		}
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
	// Creator is always an explicit member, and is the channel admin (Sprint B):
	// the creator gets management powers (edit/delete/add-remove members) by default.
	members := []models.ChannelMember{{
		UserID:    userID,
		Role:      "admin",
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

func (s *channelService) Update(id, userID uint, name, description, channelType string, isSuperadmin bool) (*models.Channel, error) {
	channel, err := s.authorizeChannelTenant(id, userID, isSuperadmin)
	if err != nil {
		return nil, err
	}

	// Sprint B: pueden editar el creador, un admin del canal o un superadmin.
	if !s.canManageChannel(channel, userID, isSuperadmin) {
		return nil, ErrUnauthorized
	}

	// targetType determina el (name,type,tenant) usado para validar duplicados:
	// si se va a cambiar el tipo, el nombre debe ser único bajo el NUEVO tipo.
	targetType := channel.Type

	updates := map[string]any{}

	// Cambio de privacidad (publico<->privado), opcional. channelType vacio = sin
	// cambio. Solo se permiten "public"/"private": cambiar a/desde "direct" no es
	// valido (los DMs no cambian de tipo) y cualquier otro valor se rechaza.
	if channelType != "" {
		ct := models.ChannelType(channelType)
		if ct != models.ChannelTypePublic && ct != models.ChannelTypePrivate {
			return nil, ErrInvalidChannelType
		}
		// Un DM nunca cambia de tipo (ni a public ni a private).
		if channel.Type == models.ChannelTypeDirect {
			return nil, ErrInvalidChannelType
		}
		// Los canales de soporte (private + prefijo "Soporte · ") no cambian de tipo.
		if isSupportChannel(channel) {
			return nil, ErrInvalidChannelType
		}
		if ct != channel.Type {
			// NOTA: cambiar publico->privado (o viceversa) NO toca la membresia: los
			// miembros actuales se conservan. Es la decision de producto aceptada; un
			// canal que pasa a privado deja dentro a quienes ya estaban (se gestionan
			// despues con add/remove member si hace falta).
			updates["type"] = string(ct)
			targetType = ct
		}
	}

	if name != "" && name != channel.Name {
		// Validar nombre duplicado ANTES de escribir: si OTRO canal (id distinto)
		// ya usa ese (name, type, tenant) devolvemos un error claro en vez de dejar
		// que reviente el índice único idx_channel_name_type_tenant con un 500.
		// Usa targetType para que la validación contemple un posible cambio de tipo.
		if existing, _ := s.repo.GetChannelByNameAndType(name, targetType, channel.TenantID); existing != nil && existing.ID != channel.ID {
			return nil, ErrDuplicateChannelName
		}
		updates["name"] = name
	} else if _, changingType := updates["type"]; changingType {
		// Si solo cambia el tipo (no el nombre), revalida que el nombre actual no
		// colisione con otro canal del NUEVO tipo (mismo índice único).
		if existing, _ := s.repo.GetChannelByNameAndType(channel.Name, targetType, channel.TenantID); existing != nil && existing.ID != channel.ID {
			return nil, ErrDuplicateChannelName
		}
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

	// Decisión de producto (A-1): solo se borran canales normales. Los DMs y los
	// canales de soporte no son eliminables ni siquiera por el creador/superadmin.
	if channel.Type == models.ChannelTypeDirect || isSupportChannel(channel) {
		return ErrChannelNotDeletable
	}

	// Sprint B: pueden eliminar el creador, un admin del canal o un superadmin.
	if !s.canManageChannel(channel, userID, isSuperadmin) {
		return ErrUnauthorized
	}

	return s.repo.DeleteChannel(id)
}

func (s *channelService) AddMember(channelID, userID, memberToAdd uint, isSuperadmin bool) error {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return ErrChannelNotFound
	}

	// Sprint B: añadir miembros lo puede hacer quien gestione el canal (creador,
	// admin del canal o superadmin). Antes era "solo el creador" en privados.
	if !s.canManageChannel(channel, userID, isSuperadmin) {
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

func (s *channelService) RemoveMember(channelID, userID, memberToRemove uint, isSuperadmin bool) error {
	channel, err := s.authorizeChannelTenant(channelID, userID, isSuperadmin)
	if err != nil {
		return err
	}

	// Sprint B: cualquiera puede quitarse a SÍ MISMO; para quitar a OTRO hace falta
	// gestionar el canal (creador, admin del canal o superadmin).
	if memberToRemove != userID && !s.canManageChannel(channel, userID, isSuperadmin) {
		return ErrUnauthorized
	}

	// El creador nunca se puede quitar (ni él mismo, ni un admin, ni superadmin):
	// dejaría al canal sin su dueño/último admin garantizado.
	if memberToRemove == channel.CreatedBy {
		return fmt.Errorf("cannot remove channel creator")
	}

	if err := s.repo.RemoveMember(channelID, memberToRemove); err != nil {
		return err
	}
	s.invalidateMembers(channelID)
	return nil
}

// SetMemberRole promueve/degrada a un miembro del canal (admin|member).
// Reglas (decisión de producto B):
//   - SOLO el CREADOR del canal o un superadmin pueden cambiar roles (un admin
//     normal NO puede promover/degradar a otros).
//   - role debe ser "admin" o "member".
//   - el target debe ser miembro del canal.
//   - NO se puede degradar al CREADOR (queda siempre como admin del canal).
func (s *channelService) SetMemberRole(channelID, actorID, targetID uint, role string, isSuperadmin bool) error {
	if role != "admin" && role != "member" {
		return fmt.Errorf("invalid role")
	}

	channel, err := s.authorizeChannelTenant(channelID, actorID, isSuperadmin)
	if err != nil {
		return err
	}

	// Solo creador + superadmin. Un admin del canal NO puede gestionar roles.
	if !isSuperadmin && channel.CreatedBy != actorID {
		return ErrUnauthorized
	}

	// El creador es admin permanente: no se puede degradar (ni a member).
	if targetID == channel.CreatedBy && role != "admin" {
		return fmt.Errorf("cannot change channel creator role")
	}

	// El target debe ser miembro del canal.
	member, err := s.repo.GetMember(channelID, targetID)
	if err != nil || member == nil {
		return fmt.Errorf("target is not a channel member")
	}

	if member.Role == role {
		// No-op idempotente: el rol ya es el deseado.
		return nil
	}

	if err := s.repo.UpdateMemberRole(channelID, targetID, role); err != nil {
		return err
	}
	// Invalida el caché de miembros: el rol cambia poderes/lo que muestra la UI.
	s.invalidateMembers(channelID)
	return nil
}

// GetMembersWithRole une los usuarios miembros del canal con su Role para el
// contrato del frontend ([{id,name,email,role}]). No rompe a otros consumidores
// de GetMembers ([]User), que sigue intacto.
func (s *channelService) GetMembersWithRole(channelID uint) ([]ChannelMemberDTO, error) {
	users, err := s.repo.GetMembers(channelID)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.GetMemberRoles(channelID)
	if err != nil {
		return nil, err
	}
	roleByUser := make(map[uint]string, len(members))
	for _, m := range members {
		roleByUser[m.UserID] = m.Role
	}
	result := make([]ChannelMemberDTO, 0, len(users))
	for _, u := range users {
		role := roleByUser[u.ID]
		if role == "" {
			role = "member"
		}
		result = append(result, ChannelMemberDTO{
			ID:    u.ID,
			Name:  u.Name,
			Email: u.Email,
			Role:  role,
		})
	}
	return result, nil
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
