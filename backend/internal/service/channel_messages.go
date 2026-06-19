package service

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/obertrack/backend/internal/models"
)

// Messages

// privateHistorySince returns the history cutoff for the user in a channel:
// in private channels members only see messages from the moment they joined
// (Slack-like); public channels and DMs expose the full history.
func (s *channelService) privateHistorySince(channelID, userID uint) *time.Time {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil || channel.Type != models.ChannelTypePrivate {
		return nil
	}
	member, err := s.repo.GetMember(channelID, userID)
	if err != nil || member.JoinedAt.IsZero() {
		return nil
	}
	return &member.JoinedAt
}

func (s *channelService) GetMessages(channelID, userID, beforeID uint) ([]models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return nil, fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.GetMessages(channelID, 100, s.privateHistorySince(channelID, userID), beforeID)
}

// isSuperadmin reports whether the user is a platform superadmin. Superadmins are
// authorized for channel actions at the handler level (channelAccessAllowed) so
// the service-level membership gate must let them through to avoid 500s when they
// oversee a company's channels they are not an explicit member of.
//
// PRODUCT DECISION (intentional, do NOT "fix"): every `!isMember && !s.isSuperadmin`
// gate below deliberately lets superadmins read/act on ANY channel of the tenant —
// private channels and direct messages included — for supervision. This is the same
// intentional bypass documented on GetChannelsByCompany. It is by design, not a
// privacy bug; keep the superadmin escape hatch in place.
func (s *channelService) isSuperadmin(userID uint) bool {
	user, err := s.userRepo.GetByID(userID)
	return err == nil && isSuperadminUser(user)
}

func (s *channelService) SendMessage(channelID, userID uint, content, attachment, fileName string, fileSize int64, fileType string) (*models.ChannelMessage, []uint, error) {
	channel, err := s.repo.GetChannel(channelID)
	if err != nil {
		return nil, nil, err
	}

	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
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
		FileType:   fileType,
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
			"channel_id": channelID,
			"message_id": message.ID,
			"link":       fmt.Sprintf("/chat?channel=%d&message=%d", channelID, message.ID),
		})
	}

	return message, mentionedUserIDs, nil
}

// normalizeMention lowercases and strips ALL diacritics so that mention matching
// is accent- and case-insensitive ("@José" matches "jose"). Mantiene EXACTAMENTE
// la misma semántica que el frontend foldMention (ChatUtils.ts):
//
//	s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
//
// es decir, descompone en NFD y elimina toda marca combinante (categoría Unicode
// Mn), no un puñado de acentos hardcodeados. Así caracteres como ý, ě, ā, etc. se
// normalizan igual en backend y frontend (antes el switch fijo no los cubría y las
// menciones divergían). Solo cambia la normalización de acentos; los límites de
// token (@Ana ≠ @Anabel) los sigue aplicando processMentions.
func normalizeMention(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			// Mn = Mark, Nonspacing → marcas combinantes (acentos descompuestos).
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isMentionBoundary reports whether r ends a name token: anything that is not a
// letter or digit (whitespace, punctuation, end-of-string handled by caller).
func isMentionBoundary(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// processMentions resolves mentions against the real members of the channel.
// A mention is "@<member name>" appearing in the content as a whole token: the
// character right after the name must be end-of-string or a non-alphanumeric
// (isMentionBoundary), so "@Ana" does not match inside "@Anabel". The "@" itself
// is the left boundary (no check on the preceding character). Names with spaces
// like "@Laura Méndez" are supported. Returns deduplicated member IDs.
//
// Complexity: the previous implementation scanned the whole content once per
// member (O(members × length)), which is costly in company-wide public channels.
// This version returns early when there is no "@", and resolves single-word
// names by extracting the alphanumeric token right after each "@" and looking it
// up in a name→IDs map (O(length + members)). Multi-word names (a small
// minority) keep the original per-member scan as a fallback, so the observable
// behaviour — boundaries, accent/case folding, "@Ana" ≠ "@Anabel", multiple
// mentions — is preserved exactly.
func (s *channelService) processMentions(messageID uint, content string, channelID uint) []uint {
	var mentionedUserIDs []uint

	// Shortcut: the vast majority of messages contain no mention at all, so skip
	// loading members and scanning entirely when there is no "@".
	if !strings.Contains(content, "@") {
		return mentionedUserIDs
	}

	members, err := s.repo.GetMembers(channelID)
	if err != nil {
		return mentionedUserIDs
	}

	normContent := normalizeMention(content)
	contentRunes := []rune(normContent)

	// Partition members by normalized name. Single-word names go into a map for
	// O(1) token lookup; multi-word names (with spaces) keep the per-member scan.
	nameToIDs := make(map[string][]uint)
	var multiWord []struct {
		id   uint
		name string
	}
	for _, member := range members {
		name := normalizeMention(strings.TrimSpace(member.Name))
		if name == "" {
			continue
		}
		if strings.ContainsRune(name, ' ') {
			multiWord = append(multiWord, struct {
				id   uint
				name string
			}{member.ID, name})
		} else {
			nameToIDs[name] = append(nameToIDs[name], member.ID)
		}
	}

	seen := make(map[uint]bool)
	addMention := func(id uint) {
		if seen[id] {
			return
		}
		seen[id] = true
		mentionedUserIDs = append(mentionedUserIDs, id)
		s.repo.CreateMention(&models.Mention{
			MessageID: messageID,
			UserID:    id,
			Notified:  true,
		})
	}

	// Single pass over the runes: at every "@", read the maximal alphanumeric run
	// that follows it (the run ends exactly at end-of-string or an
	// isMentionBoundary rune, mirroring the original boundary check) and resolve
	// that token against the name map.
	for i := 0; i < len(contentRunes); i++ {
		if contentRunes[i] != '@' {
			continue
		}
		j := i + 1
		for j < len(contentRunes) && !isMentionBoundary(contentRunes[j]) {
			j++
		}
		if j > i+1 {
			token := string(contentRunes[i+1 : j])
			for _, id := range nameToIDs[token] {
				addMention(id)
			}
		}
	}

	// Fallback for names containing spaces: replicate the original byte-indexed
	// scan with a rune-position boundary check, so multi-word names keep exactly
	// the same matching behaviour.
	for _, m := range multiWord {
		needle := "@" + m.name
		needleRunes := []rune(needle)

		from := 0
		for {
			idx := strings.Index(normContent[from:], needle)
			if idx < 0 {
				break
			}
			runeEnd := len([]rune(normContent[:from+idx])) + len(needleRunes)
			if runeEnd >= len(contentRunes) || isMentionBoundary(contentRunes[runeEnd]) {
				addMention(m.id)
				break
			}
			from = from + idx + len(needle)
			if from >= len(normContent) {
				break
			}
		}
	}

	return mentionedUserIDs
}

func (s *channelService) EditMessage(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error) {
	// Defense in depth: enforce channel membership in the service like every other
	// method (AddReaction/PinMessage/...), so any internal caller that bypasses the
	// handler still hits the gate. Superadmins are allowed through (handler-level
	// channelAccessAllowed already authorizes them) to stay consistent with the rest.
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return nil, err
	}

	if message.ChannelID != channelID {
		return nil, fmt.Errorf("message does not belong to channel")
	}

	// Authorship: only the author edits content. Superadmins may moderate (delete)
	// but not silently rewrite another user's message, so editing stays author-only.
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
	// Defense in depth: same membership gate as the rest of the service. The handler
	// passes isSuperadmin; we also fall back to s.isSuperadmin(userID) so the gate is
	// correct even for internal callers that don't compute it (kept aligned with Edit).
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !isSuperadmin && !s.isSuperadmin(userID) {
		return fmt.Errorf("you are not a member of this channel")
	}

	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return err
	}

	if message.ChannelID != channelID {
		return fmt.Errorf("message does not belong to channel")
	}

	if message.UserID != userID && !isSuperadmin && !s.isSuperadmin(userID) {
		return fmt.Errorf("you can only delete your own messages")
	}

	return s.repo.DeleteMessage(messageID)
}

func (s *channelService) AddReaction(channelID, messageID, userID uint, emoji string) (*models.MessageReaction, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
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
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.RemoveReaction(messageID, userID, emoji)
}

func (s *channelService) PinMessage(channelID, messageID, userID uint) (*models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
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
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
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
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return nil, fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.GetPinnedMessages(channelID, s.privateHistorySince(channelID, userID))
}

func (s *channelService) GetReactions(messageID, userID uint) ([]models.MessageReaction, error) {
	message, err := s.repo.GetMessage(messageID)
	if err != nil {
		return nil, err
	}

	if isMember, _ := s.repo.IsMember(message.ChannelID, userID); !isMember && !s.isSuperadmin(userID) {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	return s.repo.GetReactions(messageID)
}

func (s *channelService) GetThreadReplies(channelID, messageID, userID uint) ([]models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return nil, fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.GetThreadReplies(messageID)
}

func (s *channelService) SendThreadReply(channelID, messageID, userID uint, content string) (*models.ChannelMessage, error) {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
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
	if isMember, _ := s.repo.IsMember(message.ChannelID, userID); !isMember && !s.isSuperadmin(userID) {
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
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return nil, fmt.Errorf("you are not a member of this channel")
	}

	return s.repo.SearchMessages(channelID, query, 50, s.privateHistorySince(channelID, userID))
}

func (s *channelService) CreateDirectMessage(userID, recipientID uint, tenantOverride uint) (*DirectMessageResponse, error) {
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

	// The DM belongs to the creator's tenant. For superadmins (tenant 0) scope it to
	// the selected company so the DM is not orphaned and shows under that company.
	dmTenant := models.TenantForUser(creator)
	if tenantOverride > 0 && isSuperadminUser(creator) {
		dmTenant = tenantOverride
	}

	dmName := fmt.Sprintf("DM-%d-%d", userID, recipientID)
	if userID > recipientID {
		dmName = fmt.Sprintf("DM-%d-%d", recipientID, userID)
	}

	dmChannel, err := s.repo.GetChannelByNameAndType(dmName, models.ChannelTypeDirect, dmTenant)
	if err == nil && dmChannel != nil {
		// B-7: a previously soft-deleted DM must be reopened in place, not returned
		// as a "ghost" inactive channel that won't show in the sidebar.
		dmChannel, err = s.reactivateChannel(dmChannel, userID)
		if err != nil {
			return nil, err
		}
		return s.buildDMResponse(dmChannel, recipientID)
	}

	newChannel := &models.Channel{
		Name:      dmName,
		Type:      models.ChannelTypeDirect,
		CreatedBy: userID,
		TenantID:  dmTenant,
		IsActive:  true,
	}

	if err := s.repo.CreateDMChannel(newChannel, []uint{userID, recipientID}); err != nil {
		return nil, err
	}
	s.invalidateMembers(newChannel.ID)

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

// ContactSupport finds-or-creates a private support channel between the calling
// client user (profesional/empleador) and every active customer_success agent,
// posts an intro message on first contact and alerts all CS agents. Returns the
// channel so the frontend can open it right away.
//
// The channel lives in the client's tenant but CS agents (tenant 0) are added as
// explicit members, so they see it and can read/reply via membership checks —
// tenant isolation is bypassed deliberately, only for this support channel.
func (s *channelService) ContactSupport(userID uint) (*ChannelWithUnread, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	// El soporte es para usuarios cliente; CS y superadmins no lo necesitan.
	if isSuperadminUser(user) || user.UserType == models.UserTypeCustomerSuccess {
		return nil, fmt.Errorf("el soporte está disponible solo para usuarios cliente")
	}

	tenantID := models.TenantForUser(user)
	if tenantID == 0 && user.UserType == models.UserTypeEmployer {
		tenantID = user.ID
	}

	// Agentes de Customer Success activos: miembros del canal y destinatarios de la alerta.
	csUsers, _, err := s.userRepo.GetAll(string(models.UserTypeCustomerSuccess), "", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}
	activeCS := make([]models.User, 0, len(csUsers))
	for _, cs := range csUsers {
		if cs.IsActive {
			activeCS = append(activeCS, cs)
		}
	}

	// Nombre único por profesional (incluye el id para evitar colisiones de
	// nombres dentro del mismo tenant, que enrutarían a otro a su canal).
	channelName := fmt.Sprintf("Soporte · %s #%d", user.Name, user.ID)

	channel, lookupErr := s.repo.GetChannelByNameAndType(channelName, models.ChannelTypePrivate, tenantID)
	isNew := lookupErr != nil || channel == nil
	if !isNew && !channel.IsActive {
		// B-7: the support channel existed but was soft-deleted. Reopen it in place so
		// the user lands back in their (now visible) support channel instead of a
		// ghost inactive one. Reuses the existing membership; re-adds only if missing.
		var err error
		channel, err = s.reactivateChannel(channel, userID)
		if err != nil {
			return nil, err
		}
	}
	if isNew {
		// El canal se crea SOLO con el profesional. Los agentes de soporte NO se
		// agregan automáticamente (eso saturaría a todos los CS con cada canal):
		// reciben una invitación y se unen al aceptar el ticket (claim/assign).
		channel = &models.Channel{
			Name:        channelName,
			Description: "Canal de soporte con Customer Success",
			Type:        models.ChannelTypePrivate,
			CreatedBy:   userID,
			TenantID:    tenantID,
			IsActive:    true,
		}
		if err := s.repo.CreateDMChannel(channel, []uint{userID}); err != nil {
			return nil, err
		}
		s.invalidateMembers(channel.ID)
		channel, _ = s.repo.GetChannel(channel.ID)
	}

	// Asegura el ticket de gestión asociado al canal (crea si falta — también
	// hace backfill de canales de soporte creados antes de esta funcionalidad).
	ticket, terr := s.repo.GetSupportTicketByChannel(channel.ID)
	if terr != nil || ticket == nil {
		ticket = &models.SupportTicket{
			ChannelID:   channel.ID,
			TenantID:    tenantID,
			RequesterID: userID,
			Status:      models.SupportStatusOpen,
		}
		if err := s.repo.CreateSupportTicket(ticket); err != nil {
			log.Printf("[ContactSupport] no se pudo crear el ticket: %v", err)
		}
	}

	// Mensaje inicial solo al crear, para dar contexto a Customer Success.
	if isNew {
		intro := fmt.Sprintf("👋 Hola, soy %s. Necesito ayuda del equipo de soporte.", user.Name)
		if _, _, err := s.SendMessage(channel.ID, userID, intro, "", "", 0, ""); err != nil {
			log.Printf("[ContactSupport] no se pudo publicar el mensaje inicial: %v", err)
		}
	}

	// Invitación: si el ticket está sin asignar, se avisa a TODOS los agentes para
	// que alguno lo acepte (entra a la cola de pendientes). Si ya tiene responsable,
	// se avisa solo a ese agente para no saturar al resto del equipo.
	who := user.Name
	if user.CompanyName != "" {
		who = fmt.Sprintf("%s (%s)", user.Name, user.CompanyName)
	}
	if ticket.AssignedTo != nil {
		s.notifySupport(*ticket.AssignedTo, channel.ID, "Soporte: nueva actividad", fmt.Sprintf("%s volvió a escribir en su solicitud de soporte.", who))
	} else {
		for _, cs := range activeCS {
			s.notifySupport(cs.ID, channel.ID, "Nueva solicitud de soporte", fmt.Sprintf("%s solicita soporte. Acéptala para atenderla.", who))
		}
	}

	unread, _ := s.repo.GetUnreadCount(channel.ID, userID)
	return &ChannelWithUnread{
		ID:          channel.ID,
		Name:        channel.Name,
		Description: channel.Description,
		Type:        channel.Type,
		CreatedBy:   channel.CreatedBy,
		IsActive:    channel.IsActive,
		CreatedAt:   channel.CreatedAt,
		UnreadCount: unread,
	}, nil
}

// isSupportAgent reports whether the user can manage support tickets (CS o superadmin).
func (s *channelService) isSupportAgent(userID uint) bool {
	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return false
	}
	return isSuperadminUser(user) || user.UserType == models.UserTypeCustomerSuccess
}

// ListSupportAgents devuelve los agentes activos (customer_success + superadmin),
// usados para el selector de "reasignar".
func (s *channelService) ListSupportAgents() ([]models.User, error) {
	cs, _, err := s.userRepo.GetAll(string(models.UserTypeCustomerSuccess), "", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}
	sa, _, err := s.userRepo.GetAll(string(models.UserTypeSuperadmin), "", "", 0, 0, 1000)
	if err != nil {
		return nil, err
	}
	agents := make([]models.User, 0, len(cs)+len(sa))
	for _, u := range append(cs, sa...) {
		if u.IsActive {
			agents = append(agents, u)
		}
	}
	return agents, nil
}

// ListPendingSupport devuelve los tickets de soporte sin asignar (cola de
// solicitudes que cualquier agente puede aceptar). Solo para agentes de soporte.
func (s *channelService) ListPendingSupport(userID uint, companyFilter uint) ([]models.SupportTicket, error) {
	if !s.isSupportAgent(userID) {
		return nil, fmt.Errorf("solo Customer Success o superadmins pueden ver la cola de soporte")
	}
	return s.repo.GetPendingSupportTickets(companyFilter)
}

// ListSupportTicketsForBoard devuelve todos los tickets de soporte (con
// solicitante y responsable) para mostrarlos en el tablero de Tickets de Soporte.
func (s *channelService) ListSupportTicketsForBoard() ([]models.SupportTicket, error) {
	return s.repo.GetAllSupportTickets()
}

// NotifySupportReply avisa por campana cuando hay una respuesta en un ticket de
// soporte YA ASIGNADO: si responde el solicitante, avisa al responsable; si
// responde un agente, avisa al solicitante. Best-effort, no bloquea el envío.
// Se invoca desde el handler (solo mensajes reales de usuario), no para los
// mensajes de sistema internos (tomó/asignó/resolvió).
func (s *channelService) NotifySupportReply(channelID, senderID uint, content string, alreadyNotified []uint) {
	ticket, err := s.repo.GetSupportTicketByChannel(channelID)
	if err != nil || ticket == nil || ticket.AssignedTo == nil {
		return // no es un canal de soporte, o aún no tiene responsable
	}

	// No repetir a quien ya recibió notificación por mención, ni al propio emisor.
	skip := map[uint]bool{senderID: true}
	for _, id := range alreadyNotified {
		skip[id] = true
	}

	preview := strings.TrimSpace(content)
	if preview == "" {
		preview = "📎 Adjunto"
	}
	if r := []rune(preview); len(r) > 80 {
		preview = string(r[:80]) + "…"
	}

	senderName := s.userName(senderID)
	if senderID == ticket.RequesterID {
		// El solicitante respondió → avisar al responsable.
		if !skip[*ticket.AssignedTo] {
			s.notifySupport(*ticket.AssignedTo, channelID, "Soporte: nueva respuesta", fmt.Sprintf("%s: %s", senderName, preview))
		}
	} else {
		// Un agente respondió → avisar al solicitante.
		if !skip[ticket.RequesterID] {
			s.notifySupport(ticket.RequesterID, channelID, "Soporte respondió", fmt.Sprintf("%s: %s", senderName, preview))
		}
	}
}

func (s *channelService) userName(userID uint) string {
	if u, err := s.userRepo.GetByID(userID); err == nil && u != nil {
		return u.Name
	}
	return ""
}

// postSupportSystemMessage persiste un mensaje de sistema (🛟 tomó / asignó /
// ✅ resuelto) y lo difunde en vivo por WebSocket. A diferencia de los mensajes
// normales de usuario —que difunde el handler HTTP SendMessage— estos se
// generan dentro del servicio (claim/assign/resolve) y NO pasan por ese
// handler, así que sin esta difusión no llegaban a los clientes conectados.
func (s *channelService) postSupportSystemMessage(channelID, actorID uint, content string) {
	message, _, err := s.SendMessage(channelID, actorID, content, "", "", 0, "")
	if err != nil {
		log.Printf("[support] no se pudo publicar mensaje de sistema: %v", err)
		return
	}
	if s.broadcast != nil && message != nil {
		s.broadcast(channelID, message)
	}
}

func (s *channelService) notifySupport(userID uint, channelID uint, title, message string) {
	if err := s.notifSvc.CreateNotification(userID, "support", title, message, map[string]interface{}{
		"channel_id": channelID,
		"link":       fmt.Sprintf("/chat?channel=%d", channelID),
	}); err != nil {
		log.Printf("[support] no se pudo notificar a %d: %v", userID, err)
	}
}

// ClaimSupportTicket asigna el ticket al agente que lo toma (estado → asignado).
func (s *channelService) ClaimSupportTicket(channelID, userID uint) (*models.SupportTicket, error) {
	if !s.isSupportAgent(userID) {
		return nil, fmt.Errorf("solo Customer Success o superadmins pueden gestionar tickets de soporte")
	}
	return s.assignSupport(channelID, userID, userID, true)
}

// AssignSupportTicket reasigna el ticket a otro agente de soporte.
func (s *channelService) AssignSupportTicket(channelID, actorID, assigneeID uint) (*models.SupportTicket, error) {
	if !s.isSupportAgent(actorID) {
		return nil, fmt.Errorf("solo Customer Success o superadmins pueden gestionar tickets de soporte")
	}
	if !s.isSupportAgent(assigneeID) {
		return nil, fmt.Errorf("solo puedes asignar el ticket a un agente de soporte")
	}
	return s.assignSupport(channelID, actorID, assigneeID, false)
}

// assignSupport es la lógica común de tomar/reasignar.
func (s *channelService) assignSupport(channelID, actorID, assigneeID uint, selfClaim bool) (*models.SupportTicket, error) {
	ticket, err := s.repo.GetSupportTicketByChannel(channelID)
	if err != nil || ticket == nil {
		return nil, fmt.Errorf("ticket de soporte no encontrado")
	}

	// El asignado debe ser miembro del canal para verlo y responder.
	if member, _ := s.repo.GetMember(channelID, assigneeID); member == nil {
		// Los canales de soporte son privados: privateHistorySince oculta el
		// historial previo a JoinedAt. El agente de soporte debe ver TODO el
		// ticket, así que lo unimos desde la fecha de creación del canal.
		joinedAt := time.Now()
		if channel, err := s.repo.GetChannel(channelID); err == nil && channel != nil && !channel.CreatedAt.IsZero() {
			joinedAt = channel.CreatedAt
		}
		if err := s.repo.AddMember(&models.ChannelMember{
			ChannelID: channelID, UserID: assigneeID, Role: "member", JoinedAt: joinedAt,
		}); err != nil {
			log.Printf("[support] no se pudo añadir al asignado %d: %v", assigneeID, err)
		} else {
			// El agente ahora es miembro: invalida el caché para que reciba los
			// broadcasts del ticket en vivo de inmediato (sin esperar el TTL).
			s.invalidateMembers(channelID)
		}
	}

	now := time.Now()
	if err := s.repo.UpdateSupportTicket(ticket, map[string]interface{}{
		"assigned_to": assigneeID,
		"assigned_at": now,
		"status":      models.SupportStatusAssigned,
		// Reabre si estaba resuelto.
		"resolved_by": nil,
		"resolved_at": nil,
	}); err != nil {
		return nil, err
	}

	actorName := s.userName(actorID)
	assigneeName := s.userName(assigneeID)
	if selfClaim {
		s.postSupportSystemMessage(channelID, actorID, fmt.Sprintf("🛟 %s tomó el ticket.", actorName))
	} else {
		s.postSupportSystemMessage(channelID, actorID, fmt.Sprintf("🛟 %s asignó el ticket a %s.", actorName, assigneeName))
		if assigneeID != actorID {
			s.notifySupport(assigneeID, channelID, "Ticket de soporte asignado", fmt.Sprintf("%s te asignó un ticket de soporte.", actorName))
		}
	}
	if ticket.RequesterID != actorID {
		s.notifySupport(ticket.RequesterID, channelID, "Soporte en camino", fmt.Sprintf("%s está atendiendo tu solicitud.", assigneeName))
	}

	return s.repo.GetSupportTicketByChannel(channelID)
}

// ResolveSupportTicket marca el ticket como resuelto.
func (s *channelService) ResolveSupportTicket(channelID, actorID uint) (*models.SupportTicket, error) {
	if !s.isSupportAgent(actorID) {
		return nil, fmt.Errorf("solo Customer Success o superadmins pueden gestionar tickets de soporte")
	}
	ticket, err := s.repo.GetSupportTicketByChannel(channelID)
	if err != nil || ticket == nil {
		return nil, fmt.Errorf("ticket de soporte no encontrado")
	}

	now := time.Now()
	if err := s.repo.UpdateSupportTicket(ticket, map[string]interface{}{
		"status":      models.SupportStatusResolved,
		"resolved_by": actorID,
		"resolved_at": now,
	}); err != nil {
		return nil, err
	}

	actorName := s.userName(actorID)
	s.postSupportSystemMessage(channelID, actorID, fmt.Sprintf("✅ %s marcó el ticket como resuelto.", actorName))
	if ticket.RequesterID != actorID {
		s.notifySupport(ticket.RequesterID, channelID, "Ticket de soporte resuelto", fmt.Sprintf("%s marcó tu solicitud como resuelta.", actorName))
	}

	return s.repo.GetSupportTicketByChannel(channelID)
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

func (s *channelService) GetStatuses(userIDs []uint, tenantID uint, isSuperadmin bool) ([]models.UserStatus, error) {
	return s.repo.GetUserStatuses(userIDs, tenantID, isSuperadmin)
}

func (s *channelService) GetTotalUnreadCount(userID uint) (int64, error) {
	return s.repo.GetTotalUnreadCount(userID)
}

func (s *channelService) MarkAsRead(channelID, userID uint) error {
	if isMember, _ := s.repo.IsMember(channelID, userID); !isMember && !s.isSuperadmin(userID) {
		return fmt.Errorf("you are not a member of this channel")
	}
	return s.repo.MarkAsRead(channelID, userID)
}

func (s *channelService) GetAllUsers(tenantID uint, isSuperadmin bool, companyFilter uint) ([]models.User, error) {
	if isSuperadmin {
		// Superadmin must scope to a company; without it return no users so DMs
		// can't be started across tenants.
		if companyFilter == 0 {
			return []models.User{}, nil
		}
		// Scope to the selected company's members (tenant-scoped, not superadmin-wide).
		return s.repo.GetActiveUsers(companyFilter, false)
	}
	return s.repo.GetActiveUsers(tenantID, isSuperadmin)
}
