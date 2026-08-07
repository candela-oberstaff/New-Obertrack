package service

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Estos tests fijan los dos comportamientos más delicados del módulo de
// soporte, que ya causaron errores en producción:
//
//  1. El claim de un ticket es ATÓMICO: dos agentes pulsando "Tomar" a la vez
//     no se pisan en silencio — el que pierde la carrera recibe un error claro
//     y no queda como miembro del canal. El takeover deliberado (desde el
//     chat) sigue permitido con su flag explícito.
//  2. ContactSupport REUTILIZA el ticket activo del canal: una "Nueva
//     solicitud" del cliente se anexa a la conversación en curso en vez de
//     crear otro ticket y resolver el anterior en silencio (que cerraba casos
//     asignados sin que su responsable se enterara).

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type supportFlowRepo struct {
	repository.ChannelRepository

	channel   *models.Channel
	active    *models.SupportTicket // GetActiveSupportTicketByChannel
	activeErr error
	ticket    *models.SupportTicket // GetSupportTicketByID

	claimFree         bool // resultado de ClaimSupportTicketIfFree
	claimIfFreeCalled bool
	updateCalled      bool
	updates           map[string]interface{}
	created           *models.SupportTicket
	resolveExcept     bool
	addedMembers      []uint
	posted            []string // contenidos persistidos vía CreateMessage
}

func (f *supportFlowRepo) GetChannel(id uint) (*models.Channel, error) { return f.channel, nil }

func (f *supportFlowRepo) GetChannelByNameAndType(name string, t models.ChannelType, tenant uint) (*models.Channel, error) {
	return f.channel, nil
}

func (f *supportFlowRepo) GetActiveSupportTicketByChannel(channelID uint) (*models.SupportTicket, error) {
	return f.active, f.activeErr
}

func (f *supportFlowRepo) GetSupportTicketByID(id uint) (*models.SupportTicket, error) {
	return f.ticket, nil
}

func (f *supportFlowRepo) CreateSupportTicket(t *models.SupportTicket) error {
	t.ID = 999
	f.created = t
	return nil
}

func (f *supportFlowRepo) ResolveOpenTicketsExcept(channelID, exceptID uint) error {
	f.resolveExcept = true
	return nil
}

func (f *supportFlowRepo) ClaimSupportTicketIfFree(ticketID, assigneeID uint, now time.Time) (bool, error) {
	f.claimIfFreeCalled = true
	return f.claimFree, nil
}

func (f *supportFlowRepo) UpdateSupportTicket(t *models.SupportTicket, updates map[string]interface{}) error {
	f.updateCalled = true
	f.updates = updates
	return nil
}

func (f *supportFlowRepo) GetMember(channelID, userID uint) (*models.ChannelMember, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *supportFlowRepo) AddMember(m *models.ChannelMember) error {
	f.addedMembers = append(f.addedMembers, m.UserID)
	return nil
}

func (f *supportFlowRepo) CreateMessage(m *models.ChannelMessage) error {
	m.ID = uint(len(f.posted) + 1)
	f.posted = append(f.posted, m.Content)
	return nil
}

func (f *supportFlowRepo) GetMessage(id uint) (*models.ChannelMessage, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *supportFlowRepo) IsMember(channelID, userID uint) (bool, error)   { return true, nil }
func (f *supportFlowRepo) IsArchived(userID, channelID uint) (bool, error) { return false, nil }
func (f *supportFlowRepo) GetUnreadCount(channelID, userID uint) (int64, error) {
	return 0, nil
}

type supportUserRepo struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (f *supportUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *supportUserRepo) GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error) {
	return nil, 0, nil
}

type recordedNotif struct {
	userID uint
	title  string
}

type supportNotifSvc struct {
	NotificationService
	sent []recordedNotif
}

func (f *supportNotifSvc) CreateNotification(userID uint, notifType, title, message string, data map[string]interface{}) error {
	f.sent = append(f.sent, recordedNotif{userID: userID, title: title})
	return nil
}

func notifiedUser(sent []recordedNotif, userID uint) bool {
	for _, n := range sent {
		if n.userID == userID {
			return true
		}
	}
	return false
}

// newSupportFlowService arma un channelService con el agente CS 6 ("Beto"),
// la agente CS 4 ("Ana") y el solicitante 30 ("Laura", profesional del tenant 5).
func newSupportFlowService(repo *supportFlowRepo) (*channelService, *supportNotifSvc) {
	empleador := uint(5)
	notif := &supportNotifSvc{}
	users := &supportUserRepo{users: map[uint]*models.User{
		4:  {ID: 4, Name: "Ana", UserType: models.UserTypeCustomerSuccess, IsActive: true},
		6:  {ID: 6, Name: "Beto", UserType: models.UserTypeCustomerSuccess, IsActive: true},
		30: {ID: 30, Name: "Laura", UserType: models.UserTypeProfessional, EmpleadorID: &empleador, IsActive: true},
	}}
	return &channelService{repo: repo, userRepo: users, notifSvc: notif}, notif
}

func supportChannel() *models.Channel {
	return &models.Channel{
		ID:       77,
		Name:     "Soporte · Laura #30",
		Type:     models.ChannelTypePrivate,
		TenantID: 5,
		IsActive: true,
	}
}

// ---------------------------------------------------------------------------
// Claim atómico
// ---------------------------------------------------------------------------

// El agente que pierde la carrera del "Tomar" recibe un error con el nombre del
// ganador, no un "Tomaste el ticket" falso; y NO queda como miembro del canal
// privado ni se escribe nada en el ticket.
func TestClaimSupport_LostRace_FailsWithoutSideEffects(t *testing.T) {
	assignee := uint(4)
	repo := &supportFlowRepo{
		channel: supportChannel(),
		ticket: &models.SupportTicket{
			ID: 9, ChannelID: 77, RequesterID: 30,
			Status: models.SupportStatusAssigned, AssignedTo: &assignee,
			Assignee: &models.User{ID: 4, Name: "Ana"},
		},
		claimFree: false,
	}
	svc, _ := newSupportFlowService(repo)

	_, err := svc.ClaimSupportTicket(9, 6, false)
	if err == nil || !strings.Contains(err.Error(), "ya tomó este ticket") {
		t.Fatalf("esperaba error de carrera perdida, obtuve: %v", err)
	}
	if !strings.Contains(err.Error(), "Ana") {
		t.Fatalf("el error debería nombrar al responsable actual: %v", err)
	}
	if !repo.claimIfFreeCalled {
		t.Fatal("el claim debe pasar por el UPDATE condicional (ClaimSupportTicketIfFree)")
	}
	if repo.updateCalled {
		t.Fatal("perder la carrera no debe escribir el ticket por el camino incondicional")
	}
	if len(repo.addedMembers) != 0 {
		t.Fatal("el perdedor de la carrera no debe quedar como miembro del canal")
	}
	if len(repo.posted) != 0 {
		t.Fatal("perder la carrera no debe publicar mensajes de sistema")
	}
}

// El claim normal de un ticket libre asigna vía UPDATE condicional, une al
// agente al canal, publica el mensaje de sistema y avisa al solicitante.
func TestClaimSupport_FreeTicket_AssignsAndAnnounces(t *testing.T) {
	repo := &supportFlowRepo{
		channel: supportChannel(),
		ticket: &models.SupportTicket{
			ID: 9, ChannelID: 77, RequesterID: 30,
			Status: models.SupportStatusOpen,
		},
		claimFree: true,
	}
	svc, notif := newSupportFlowService(repo)

	if _, err := svc.ClaimSupportTicket(9, 6, false); err != nil {
		t.Fatalf("claim de un ticket libre: %v", err)
	}
	if !repo.claimIfFreeCalled {
		t.Fatal("el claim debe usar el UPDATE condicional")
	}
	if len(repo.addedMembers) != 1 || repo.addedMembers[0] != 6 {
		t.Fatalf("el agente debe quedar como miembro del canal, añadidos: %v", repo.addedMembers)
	}
	if len(repo.posted) != 1 || !strings.Contains(repo.posted[0], "Beto tomó el ticket") {
		t.Fatalf("falta el mensaje de sistema del claim, publicados: %v", repo.posted)
	}
	if !notifiedUser(notif.sent, 30) {
		t.Fatal("el solicitante debe recibir la campanita de 'Soporte en camino'")
	}
}

// takeover=true es el traspaso deliberado del chat ("lo atiende X — Tómalo"):
// se salta el UPDATE condicional y reasigna aunque el ticket tenga responsable.
func TestClaimSupport_Takeover_ReassignsAssignedTicket(t *testing.T) {
	assignee := uint(4)
	repo := &supportFlowRepo{
		channel: supportChannel(),
		ticket: &models.SupportTicket{
			ID: 9, ChannelID: 77, RequesterID: 30,
			Status: models.SupportStatusAssigned, AssignedTo: &assignee,
			Assignee: &models.User{ID: 4, Name: "Ana"},
		},
		// claimFree en false a propósito: el takeover no debe depender de él.
		claimFree: false,
	}
	svc, _ := newSupportFlowService(repo)

	if _, err := svc.ClaimSupportTicket(9, 6, true); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	if repo.claimIfFreeCalled {
		t.Fatal("el takeover no debe pasar por el UPDATE condicional")
	}
	if !repo.updateCalled {
		t.Fatal("el takeover debe escribir el ticket por el camino incondicional")
	}
	if got, ok := repo.updates["assigned_to"].(uint); !ok || got != 6 {
		t.Fatalf("assigned_to: quería 6, obtuve %v", repo.updates["assigned_to"])
	}
}

// ---------------------------------------------------------------------------
// ContactSupport: reutilización del ticket activo
// ---------------------------------------------------------------------------

// Una "Nueva solicitud" con un ticket activo en el canal se ANEXA a él: no se
// crea otro ticket ni se resuelve el anterior (eso cerraba en silencio casos
// asignados y en atención). El asunto nuevo queda visible como encabezado en la
// conversación y el responsable recibe campanita.
func TestContactSupport_ActiveTicket_IsReusedNotReplaced(t *testing.T) {
	assignee := uint(4)
	repo := &supportFlowRepo{
		channel: supportChannel(),
		active: &models.SupportTicket{
			ID: 9, ChannelID: 77, RequesterID: 30,
			Subject: "Consulta original",
			Status:  models.SupportStatusAssigned, AssignedTo: &assignee,
		},
	}
	svc, notif := newSupportFlowService(repo)

	// forceNew=true y con asunto: exactamente lo que manda el formulario del
	// cliente, que era lo que antes resolvía el ticket del agente.
	if _, err := svc.ContactSupport(30, "Impresora rota", "no imprime nada", "Alta", "Tareas", true); err != nil {
		t.Fatalf("ContactSupport: %v", err)
	}

	if repo.created != nil {
		t.Fatalf("no debe crearse un ticket nuevo habiendo uno activo; se creó %+v", repo.created)
	}
	if repo.resolveExcept {
		t.Fatal("no debe resolverse el ticket activo del canal")
	}
	header := ""
	for _, p := range repo.posted {
		if strings.Contains(p, "Nueva consulta en la solicitud abierta") {
			header = p
			break
		}
	}
	if header == "" {
		t.Fatalf("falta el encabezado de la nueva consulta, publicados: %v", repo.posted)
	}
	for _, want := range []string{"Impresora rota", "Prioridad Alta", "Módulo Tareas"} {
		if !strings.Contains(header, want) {
			t.Fatalf("el encabezado debería incluir %q: %q", want, header)
		}
	}
	if !notifiedUser(notif.sent, 4) {
		t.Fatal("el responsable del ticket activo debe recibir campanita de la nueva actividad")
	}
}

// Sin ticket activo (p. ej. el anterior ya se resolvió) sí se abre uno nuevo,
// con la limpieza de duplicados heredados de siempre.
func TestContactSupport_NoActiveTicket_CreatesNewOne(t *testing.T) {
	repo := &supportFlowRepo{
		channel:   supportChannel(),
		activeErr: gorm.ErrRecordNotFound,
	}
	svc, _ := newSupportFlowService(repo)

	if _, err := svc.ContactSupport(30, "Impresora rota", "no imprime nada", "Alta", "Tareas", true); err != nil {
		t.Fatalf("ContactSupport: %v", err)
	}

	if repo.created == nil {
		t.Fatal("sin ticket activo debe crearse uno nuevo")
	}
	if repo.created.Subject != "Impresora rota" || repo.created.Status != models.SupportStatusOpen {
		t.Fatalf("ticket creado inesperado: %+v", repo.created)
	}
	if !repo.resolveExcept {
		t.Fatal("la limpieza de duplicados heredados debe seguir corriendo al crear")
	}
	found := false
	for _, p := range repo.posted {
		if strings.Contains(p, "Nueva solicitud: Impresora rota") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("falta el encabezado de la solicitud nueva, publicados: %v", repo.posted)
	}
}

// ---------------------------------------------------------------------------
// WhatsApp: claim atómico
// ---------------------------------------------------------------------------

type waClaimRepo struct {
	repository.TicketRepository

	ticket        *models.Ticket
	claimFree     bool
	claimCalled   bool
	updatedFields map[string]interface{}
}

func (f *waClaimRepo) GetByID(id uint) (*models.Ticket, error) { return f.ticket, nil }

func (f *waClaimRepo) ClaimTicketIfFree(ticketID, agentID uint) (bool, error) {
	f.claimCalled = true
	return f.claimFree, nil
}

func (f *waClaimRepo) UpdateTicketFields(ticketID uint, updates map[string]interface{}) error {
	f.updatedFields = updates
	return nil
}

// Un agente normal que llega tarde al "Tomar" recibe ErrConflict (409) en vez
// de quedarse con la conversación de un compañero.
func TestWhatsAppClaim_LostRace_ReturnsConflict(t *testing.T) {
	repo := &waClaimRepo{
		ticket:    &models.Ticket{ID: 12, Origin: string(models.ChannelWhatsApp)},
		claimFree: false,
	}
	svc := &ticketService{repo: repo}

	_, err := svc.WhatsAppAction(12, 6, "claim", false)
	if err != apperrors.ErrConflict {
		t.Fatalf("quería ErrConflict, obtuve: %v", err)
	}
	if !repo.claimCalled {
		t.Fatal("el claim debe pasar por el UPDATE condicional")
	}
	if repo.updatedFields != nil {
		t.Fatal("perder la carrera no debe escribir el ticket")
	}
}

// El superadmin retoma la conversación sin condición: es el único "Tomar" sobre
// un chat ajeno que ofrece la interfaz.
func TestWhatsAppClaim_SuperadminTakeover_Unconditional(t *testing.T) {
	repo := &waClaimRepo{
		ticket:    &models.Ticket{ID: 12, Origin: string(models.ChannelWhatsApp)},
		claimFree: false, // irrelevante: no debe consultarse
	}
	svc := &ticketService{repo: repo}

	if _, err := svc.WhatsAppAction(12, 6, "claim", true); err != nil {
		t.Fatalf("retomar como superadmin: %v", err)
	}
	if repo.claimCalled {
		t.Fatal("el retomar de superadmin no debe pasar por el UPDATE condicional")
	}
	if got, ok := repo.updatedFields["assigned_to"].(uint); !ok || got != 6 {
		t.Fatalf("assigned_to: quería 6, obtuve %v", repo.updatedFields["assigned_to"])
	}
}

// El claim normal de una conversación libre asigna y no devuelve error.
func TestWhatsAppClaim_FreeChat_Succeeds(t *testing.T) {
	repo := &waClaimRepo{
		ticket:    &models.Ticket{ID: 12, Origin: string(models.ChannelWhatsApp)},
		claimFree: true,
	}
	svc := &ticketService{repo: repo}

	if _, err := svc.WhatsAppAction(12, 6, "claim", false); err != nil {
		t.Fatalf("claim de una conversación libre: %v", err)
	}
	if !repo.claimCalled {
		t.Fatal("el claim debe usar el UPDATE condicional")
	}
}
