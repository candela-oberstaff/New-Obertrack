package service

import (
	"errors"
	"fmt"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

var (
	ErrBoardNotFound      = errors.New("Tablero no encontrado")
	ErrBoardAccessDenied  = errors.New("Access denied")
	ErrAlreadyBoardMember = errors.New("Ya eres miembro de este tablero")
	ErrAlreadyPending     = errors.New("Ya existe una invitación o solicitud pendiente para este tablero")
	ErrInvitationNotFound = errors.New("Invitación no encontrada")
	ErrInvitationResolved = errors.New("Esta invitación ya fue resuelta")
	ErrCreatorCannotLeave = errors.New("El creador no puede salir de su propio tablero")
	ErrNotBoardMember     = errors.New("No eres miembro de este tablero")
)

type BoardService interface {
	GetAll(userID uint, role string, isSuperadmin bool, companyID uint) ([]models.Board, error)
	GetPublicBoards(userID uint, companyID uint) ([]models.Board, error)
	GetByID(userID uint, role string, isSuperadmin bool, companyID uint, boardID uint) (*models.Board, error)
	Create(userID uint, name, description, color string, memberIDs []uint, phases []struct {
		Name  string
		Color string
	}, tenantOverride uint) (*models.Board, error)
	Update(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.Board, error)
	Delete(userID, boardID, tenantID uint, isSuperadmin bool) error
	AddPhase(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, name, color string) (*models.Board, error)
	RemovePhase(boardID, phaseID, tenantID, userID uint, role string, isManager, isSuperadmin bool) (*models.Board, error)
	ReorderPhases(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, phaseIDs []uint) (*models.Board, error)

	InviteMembers(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, targetIDs []uint) ([]models.BoardInvitation, error)
	RequestJoin(userID, boardID uint) (*models.BoardInvitation, error)
	ListMyInvitations(userID uint) ([]models.BoardInvitation, error)
	ListBoardPending(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, kind string) ([]models.BoardInvitation, error)
	ResolveInvitation(invID, userID uint, role string, isManager, isSuperadmin bool, accept bool) (*models.BoardInvitation, error)
	CancelInvitation(invID, userID uint, role string, isManager, isSuperadmin bool) error
	RemoveMember(boardID, targetUserID, tenantID, userID uint, role string, isManager, isSuperadmin bool) (*models.Board, error)
	LeaveBoard(userID, boardID uint) error
}

type boardService struct {
	repo     repository.BoardRepository
	userRepo repository.UserRepository
	notifSvc NotificationService
}

func NewBoardService(
	repo repository.BoardRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
) BoardService {
	return &boardService{
		repo:     repo,
		userRepo: userRepo,
		notifSvc: notifSvc,
	}
}

func (s *boardService) canManageBoard(board *models.Board, userID uint, role string, isManager, isSuperadmin bool) bool {
	return isSuperadmin || board.CreatedBy == userID || isEmployerRole(role) || isManager
}

func boardHasMember(board *models.Board, userID uint) bool {
	for _, m := range board.Members {
		if m.ID == userID {
			return true
		}
	}
	return false
}

// notify avisa solo por notificación in-app (campana + WebSocket). La membresía
// de tableros NO manda correos: invitar, aceptar y aprobar se avisan únicamente
// dentro de la app.
func (s *boardService) notify(user *models.User, notifType, title, message, link string) {
	if user == nil || s.notifSvc == nil {
		return
	}
	_ = s.notifSvc.CreateNotification(user.ID, notifType, title, message, map[string]interface{}{"link": link})
}

func (s *boardService) authorizeBoardTenant(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool) (*models.Board, error) {
	var board *models.Board
	var err error
	if isSuperadmin || tenantID == 0 {
		board, err = s.repo.GetByID(boardID)
	} else {
		board, err = s.repo.GetByIDAndTenant(boardID, tenantID)
	}
	if err != nil {
		return nil, errors.New("Board not found")
	}
	if isSuperadmin {
		return board, nil
	}
	if tenantID == 0 || board.TenantID != tenantID {
		return nil, errors.New("Access denied")
	}
	isMember := false
	for _, m := range board.Members {
		if m.ID == userID {
			isMember = true
			break
		}
	}
	if board.CreatedBy == userID || isEmployerRole(role) || isManager || isMember {
		return board, nil
	}
	return nil, errors.New("Access denied")
}

func (s *boardService) GetAll(userID uint, role string, isSuperadmin bool, companyID uint) ([]models.Board, error) {
	// Superadmin without an explicit company selection: return nothing so we never
	// mix boards from different tenants in the same view.
	if isSuperadmin && companyID == 0 {
		return []models.Board{}, nil
	}

	filters := make(map[string]interface{})

	if companyID > 0 {
		// Superadmin scoped to a company, plus employers and managers, see all boards
		// within the tenant (no per-user restriction). Professionals still need the
		// user_id filter so they only see boards they belong to.
		filters["tenant_id"] = companyID
		if !isSuperadmin && !isEmployerRole(role) && role != "manager" {
			filters["user_id"] = userID
		}
	} else if role != "superadmin" {
		// No tenant context available — fall back to user scoping
		filters["user_id"] = userID
	}

	return s.repo.FindAll(filters)
}

func (s *boardService) GetPublicBoards(userID uint, companyID uint) ([]models.Board, error) {
	// Safety: if no tenant context, return empty instead of leaking boards from
	// all companies (same pattern as GetActiveUsers).
	if companyID == 0 {
		return []models.Board{}, nil
	}

	filters := map[string]interface{}{"tenant_id": companyID}
	all, err := s.repo.FindAll(filters)
	if err != nil {
		return nil, err
	}

	pendingIDs, err := s.repo.ListPendingBoardIDsForUser(userID)
	if err != nil {
		return nil, err
	}
	pending := make(map[uint]bool, len(pendingIDs))
	for _, id := range pendingIDs {
		pending[id] = true
	}

	public := []models.Board{}
	for _, b := range all {
		if b.CreatedBy == userID || boardHasMember(&b, userID) || pending[b.ID] {
			continue
		}
		public = append(public, b)
	}
	return public, nil
}

func (s *boardService) GetByID(userID uint, role string, isSuperadmin bool, companyID uint, boardID uint) (*models.Board, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, errors.New("Board not found")
	}

	// Enforce tenant: if caller is not superadmin, ensure board belongs to caller's tenant
	if !isSuperadmin && companyID > 0 {
		creator, err := s.userRepo.GetByID(board.CreatedBy)
		if err != nil {
			return nil, errors.New("Board creator not found")
		}
		if tenantForUser(creator) != companyID {
			return nil, errors.New("Access denied")
		}
	}

	if !isSuperadmin && role != "superadmin" {
		isMember := false
		for _, m := range board.Members {
			if m.ID == userID {
				isMember = true
				break
			}
		}
		if board.CreatedBy != userID && !isMember {
			return nil, errors.New("Access denied")
		}
	}
	return board, nil
}

func (s *boardService) Create(userID uint, name, description, color string, memberIDs []uint, phases []struct {
	Name  string
	Color string
}, tenantOverride uint) (*models.Board, error) {
	if color == "" {
		color = "#3b82f6"
	}

	user, _ := s.userRepo.GetByID(userID)

	tenantID := models.TenantForUser(user)
	// Superadmins create boards scoped to the company they have selected, so the
	// board is not orphaned (tenant 0) and shows up under that company's filter.
	if tenantOverride > 0 && isSuperadminUser(user) {
		tenantID = tenantOverride
	}

	board := &models.Board{
		Name:        name,
		Description: description,
		Color:       color,
		CreatedBy:   userID,
		TenantID:    tenantID,
	}

	if err := s.repo.Create(board); err != nil {
		return nil, err
	}

	if user != nil {
		s.repo.AddMember(board, user)
	}

	if len(memberIDs) > 0 {
		for _, mid := range memberIDs {
			if mid != userID {
				m, _ := s.userRepo.GetByID(mid)
				if m != nil && (isSuperadminUser(m) || models.TenantForUser(m) == board.TenantID) {
					s.repo.AddMember(board, m)
				}
			}
		}
	}

	phasesToCreate := phases
	if len(phasesToCreate) == 0 {
		phasesToCreate = []struct {
			Name  string
			Color string
		}{
			{Name: "Por hacer", Color: "#6b7280"},
			{Name: "En proceso", Color: "#3b82f6"},
			{Name: "Finalizado", Color: "#22c55e"},
		}
	}

	statusNames := []string{"por_hacer", "en_proceso", "finalizado", "", "", ""}
	for i, p := range phasesToCreate {
		scolor := p.Color
		if scolor == "" {
			scolor = "#6b7280"
		}
		status := ""
		if i < len(statusNames) {
			status = statusNames[i]
		}
		phase := &models.Phase{
			Name:   p.Name,
			Color:  scolor,
			Status: status,
			Order:  i,
		}
		s.repo.AddPhase(board, phase)
	}

	return s.repo.GetByID(board.ID)
}

func (s *boardService) Update(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.Board, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}

	if !s.canManageBoard(board, userID, role, isManager, isSuperadmin) {
		return nil, ErrBoardAccessDenied
	}

	if len(updates) > 0 {
		if err := s.repo.Update(board, updates); err != nil {
			return nil, err
		}
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) Delete(userID, boardID, tenantID uint, isSuperadmin bool) error {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return errors.New("Board not found")
	}

	if isSuperadmin {
		return s.repo.Delete(board)
	}

	// Verify tenant ownership
	if tenantID == 0 || board.TenantID != tenantID {
		return errors.New("Access denied")
	}

	if board.CreatedBy != userID {
		return errors.New("Solo el creador puede eliminar el tablero")
	}

	return s.repo.Delete(board)
}

func (s *boardService) AddPhase(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, name, color string) (*models.Board, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}

	maxOrder := -1
	for _, p := range board.Phases {
		if p.Order > maxOrder {
			maxOrder = p.Order
		}
	}

	if color == "" {
		color = "#6b7280"
	}

	phase := &models.Phase{
		Name:  name,
		Color: color,
		Order: maxOrder + 1,
	}

	if err := s.repo.AddPhase(board, phase); err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) RemovePhase(boardID, phaseID, tenantID, userID uint, role string, isManager, isSuperadmin bool) (*models.Board, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}

	found := false
	var phaseToRemove *models.Phase
	for _, p := range board.Phases {
		if p.ID == phaseID {
			found = true
			phaseToRemove = &p
			break
		}
	}
	if !found {
		return nil, errors.New("Phase not found on this board")
	}

	// Safety check: are there tasks in this phase?
	// We can add a simple FindAll call to check
	tasks, _, _ := s.repo.FindTasksByPhase(boardID, phaseToRemove.Status)
	if len(tasks) > 0 {
		return nil, errors.New("Cannot remove phase with tasks. Move or delete tasks first.")
	}

	if err := s.repo.RemovePhase(board, phaseID); err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}

func (s *boardService) ReorderPhases(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, phaseIDs []uint) (*models.Board, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdatePhasesOrder(board, phaseIDs); err != nil {
		return nil, err
	}

	return s.repo.GetByID(boardID)
}


func (s *boardService) InviteMembers(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, targetIDs []uint) ([]models.BoardInvitation, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}
	if !s.canManageBoard(board, userID, role, isManager, isSuperadmin) {
		return nil, ErrBoardAccessDenied
	}

	inviter, _ := s.userRepo.GetByID(userID)
	created := []models.BoardInvitation{}

	for _, tid := range targetIDs {
		if tid == userID || boardHasMember(board, tid) {
			continue
		}
		target, err := s.userRepo.GetByID(tid)
		if err != nil || target == nil {
			continue
		}
		if !isSuperadminUser(target) && models.TenantForUser(target) != board.TenantID {
			continue
		}
		if has, err := s.repo.HasPendingInvitation(boardID, tid); err != nil || has {
			continue
		}

		inv := &models.BoardInvitation{
			BoardID:   boardID,
			UserID:    tid,
			Kind:      models.BoardInviteKindInvitation,
			Status:    models.BoardInviteStatusPending,
			CreatedBy: userID,
		}
		if err := s.repo.CreateInvitation(inv); err != nil {
			continue
		}
		created = append(created, *inv)

		inviterName := "Un responsable"
		if inviter != nil {
			inviterName = inviter.Name
		}
		go s.notify(target,
			"board_invitation",
			"Invitación a un tablero",
			fmt.Sprintf("%s te invitó al tablero \"%s\".", inviterName, board.Name),
			"/tasks?invitations=1",
		)
	}

	return created, nil
}

func (s *boardService) RequestJoin(userID, boardID uint) (*models.BoardInvitation, error) {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return nil, ErrBoardNotFound
	}
	if board.CreatedBy == userID || boardHasMember(board, userID) {
		return nil, ErrAlreadyBoardMember
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return nil, errors.New("User not found")
	}
	if !isSuperadminUser(user) && models.TenantForUser(user) != board.TenantID {
		return nil, ErrBoardAccessDenied
	}

	if has, err := s.repo.HasPendingInvitation(boardID, userID); err != nil {
		return nil, err
	} else if has {
		return nil, ErrAlreadyPending
	}

	inv := &models.BoardInvitation{
		BoardID:   boardID,
		UserID:    userID,
		Kind:      models.BoardInviteKindRequest,
		Status:    models.BoardInviteStatusPending,
		CreatedBy: userID,
	}
	if err := s.repo.CreateInvitation(inv); err != nil {
		return nil, err
	}

	if owner, err := s.userRepo.GetByID(board.CreatedBy); err == nil {
		go s.notify(owner,
			"board_join_request",
			"Solicitud para unirse a un tablero",
			fmt.Sprintf("%s quiere unirse al tablero \"%s\".", user.Name, board.Name),
			fmt.Sprintf("/tasks?board=%d&requests=1", board.ID),
		)
	}

	return inv, nil
}

func (s *boardService) ListMyInvitations(userID uint) ([]models.BoardInvitation, error) {
	return s.repo.ListPendingInvitationsForUser(userID)
}

func (s *boardService) ListBoardPending(boardID, tenantID, userID uint, role string, isManager, isSuperadmin bool, kind string) ([]models.BoardInvitation, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}
	if !s.canManageBoard(board, userID, role, isManager, isSuperadmin) {
		return nil, ErrBoardAccessDenied
	}
	return s.repo.ListPendingByBoard(boardID, kind)
}

func (s *boardService) ResolveInvitation(invID, userID uint, role string, isManager, isSuperadmin bool, accept bool) (*models.BoardInvitation, error) {
	inv, err := s.repo.GetInvitationByID(invID)
	if err != nil {
		return nil, ErrInvitationNotFound
	}
	if inv.Status != models.BoardInviteStatusPending {
		return nil, ErrInvitationResolved
	}

	board, err := s.repo.GetByID(inv.BoardID)
	if err != nil {
		return nil, ErrBoardNotFound
	}

	switch inv.Kind {
	case models.BoardInviteKindInvitation:
		if inv.UserID != userID {
			return nil, ErrBoardAccessDenied
		}
	case models.BoardInviteKindRequest:
		if !s.canManageBoard(board, userID, role, isManager, isSuperadmin) {
			return nil, ErrBoardAccessDenied
		}
	default:
		return nil, ErrInvitationNotFound
	}

	if accept {
		if err := s.repo.AcceptInvitation(inv, userID); err != nil {
			return nil, err
		}
	} else if err := s.repo.ResolveInvitation(invID, models.BoardInviteStatusRejected, userID); err != nil {
		return nil, err
	}

	s.notifyResolution(inv, board, accept)

	return s.repo.GetInvitationByID(invID)
}

func (s *boardService) notifyResolution(inv *models.BoardInvitation, board *models.Board, accept bool) {
	target, _ := s.userRepo.GetByID(inv.UserID)
	owner, _ := s.userRepo.GetByID(board.CreatedBy)
	if target == nil {
		return
	}

	if inv.Kind == models.BoardInviteKindInvitation {
		verb := "rechazó"
		notifType := "board_invitation_rejected"
		if accept {
			verb = "aceptó"
			notifType = "board_invitation_accepted"
		}
		go s.notify(owner, notifType,
			"Invitación "+verb,
			fmt.Sprintf("%s %s la invitación al tablero \"%s\".", target.Name, verb, board.Name),
			fmt.Sprintf("/tasks?board=%d", board.ID),
		)
		return
	}

	if accept {
		go s.notify(target, "board_request_approved",
			"Solicitud aprobada",
			fmt.Sprintf("Tu solicitud para unirte al tablero \"%s\" fue aprobada.", board.Name),
			fmt.Sprintf("/tasks?board=%d", board.ID),
		)
		return
	}
	go s.notify(target, "board_request_rejected",
		"Solicitud rechazada",
		fmt.Sprintf("Tu solicitud para unirte al tablero \"%s\" fue rechazada.", board.Name),
		"/tasks",
	)
}

func (s *boardService) CancelInvitation(invID, userID uint, role string, isManager, isSuperadmin bool) error {
	inv, err := s.repo.GetInvitationByID(invID)
	if err != nil {
		return ErrInvitationNotFound
	}
	if inv.Status != models.BoardInviteStatusPending {
		return ErrInvitationResolved
	}

	board, err := s.repo.GetByID(inv.BoardID)
	if err != nil {
		return ErrBoardNotFound
	}

	isOwnRequest := inv.Kind == models.BoardInviteKindRequest && inv.UserID == userID
	if !isOwnRequest && !s.canManageBoard(board, userID, role, isManager, isSuperadmin) {
		return ErrBoardAccessDenied
	}

	return s.repo.ResolveInvitation(invID, models.BoardInviteStatusCanceled, userID)
}

func (s *boardService) RemoveMember(boardID, targetUserID, tenantID, userID uint, role string, isManager, isSuperadmin bool) (*models.Board, error) {
	board, err := s.authorizeBoardTenant(boardID, tenantID, userID, role, isManager, isSuperadmin)
	if err != nil {
		return nil, err
	}
	if !s.canManageBoard(board, userID, role, isManager, isSuperadmin) {
		return nil, ErrBoardAccessDenied
	}
	if targetUserID == board.CreatedBy {
		return nil, errors.New("No se puede quitar al creador del tablero")
	}
	if !boardHasMember(board, targetUserID) {
		return nil, ErrNotBoardMember
	}
	if err := s.repo.RemoveMember(board, targetUserID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(boardID)
}

func (s *boardService) LeaveBoard(userID, boardID uint) error {
	board, err := s.repo.GetByID(boardID)
	if err != nil {
		return ErrBoardNotFound
	}
	if board.CreatedBy == userID {
		return ErrCreatorCannotLeave
	}
	if !boardHasMember(board, userID) {
		return ErrNotBoardMember
	}
	if err := s.repo.RemoveMember(board, userID); err != nil {
		return err
	}

	if owner, err := s.userRepo.GetByID(board.CreatedBy); err == nil {
		if leaver, err := s.userRepo.GetByID(userID); err == nil {
			go s.notify(owner, "board_member_left",
				"Un miembro salió del tablero",
				fmt.Sprintf("%s salió del tablero \"%s\".", leaver.Name, board.Name),
				fmt.Sprintf("/tasks?board=%d", board.ID),
			)
		}
	}
	return nil
}
