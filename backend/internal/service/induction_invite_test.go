package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Invitación, reinicio, calificación por bloques y armado de programas. Mismo
// patrón de fakes que el resto del paquete: se embebe la interfaz real y se
// sobrescribe solo lo que toca el camino bajo prueba. El fake del repositorio
// guarda estado en memoria para que una secuencia de envíos se pueda seguir.

type fakeInductionRepo struct {
	repository.InductionRepository

	cfg *models.InductionConfig

	defaultProgram  *models.InductionProgram
	companyPrograms map[uint]*models.InductionProgram
	programs        map[uint]*models.InductionProgram
	blocks          map[uint]*models.InductionBlock
	blockUsage      map[uint]int64
	createdProgram  *models.InductionProgram
	defaultSet      uint

	invite        *models.InductionInvite
	inviteErr     error
	created       *models.InductionInvite
	createdBlocks []models.InductionInviteBlock
	inviteUpdate  map[string]interface{}
	deletedInvite uint
	history       []models.InductionInvite

	inviteBlocks []models.InductionInviteBlock
	blockUpdates map[uint]map[string]interface{}
	resetInvite  uint
	attempts     []models.InductionAttempt

	surveys   map[uint]*models.Survey
	tutorials map[uint]*models.Tutorial
}

func (f *fakeInductionRepo) GetConfig() (*models.InductionConfig, error) {
	if f.cfg == nil {
		return &models.InductionConfig{ID: 1, InviteTTLDays: 30}, nil
	}
	return f.cfg, nil
}

func (f *fakeInductionRepo) SaveConfig(cfg *models.InductionConfig) error {
	f.cfg = cfg
	return nil
}

func (f *fakeInductionRepo) GetDefaultProgram() (*models.InductionProgram, error) {
	return f.defaultProgram, nil
}

func (f *fakeInductionRepo) GetProgramForCompany(companyID uint) (*models.InductionProgram, error) {
	return f.companyPrograms[companyID], nil
}

func (f *fakeInductionRepo) GetProgram(id uint) (*models.InductionProgram, error) {
	if p, ok := f.programs[id]; ok {
		return p, nil
	}
	if f.createdProgram != nil && f.createdProgram.ID == id {
		return f.createdProgram, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeInductionRepo) CreateProgram(program *models.InductionProgram) error {
	program.ID = 99
	f.createdProgram = program
	return nil
}

func (f *fakeInductionRepo) UpdateProgram(id uint, updates map[string]interface{}) error {
	return nil
}

func (f *fakeInductionRepo) SetDefaultProgram(id uint) error {
	f.defaultSet = id
	return nil
}

func (f *fakeInductionRepo) GetBlock(id uint) (*models.InductionBlock, error) {
	if b, ok := f.blocks[id]; ok {
		return b, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeInductionRepo) CountProgramsUsingBlock(blockID uint) (int64, error) {
	return f.blockUsage[blockID], nil
}

func (f *fakeInductionRepo) DeleteBlock(id uint) error {
	delete(f.blocks, id)
	return nil
}

func (f *fakeInductionRepo) GetInviteByUser(userID uint) (*models.InductionInvite, error) {
	if f.inviteErr != nil {
		return nil, f.inviteErr
	}
	return f.invite, nil
}

func (f *fakeInductionRepo) GetInviteByToken(token string) (*models.InductionInvite, error) {
	if f.invite == nil || f.invite.Token != token {
		return nil, errors.New("not found")
	}
	return f.invite, nil
}

func (f *fakeInductionRepo) UpdateInvite(invite *models.InductionInvite, updates map[string]interface{}) error {
	f.inviteUpdate = updates
	if status, ok := updates["status"].(string); ok && f.invite != nil {
		f.invite.Status = status
	}
	return nil
}

func (f *fakeInductionRepo) CreateInvite(invite *models.InductionInvite, blocks []models.InductionInviteBlock) error {
	invite.ID = 1
	f.created = invite
	f.createdBlocks = blocks
	return nil
}

func (f *fakeInductionRepo) DeleteInvite(inviteID uint) error {
	f.deletedInvite = inviteID
	return nil
}

func (f *fakeInductionRepo) GetPendingInviteByUser(userID uint) (*models.InductionInvite, error) {
	if f.invite != nil && f.invite.UserID == userID && f.invite.Status == models.InductionPending {
		return f.invite, nil
	}
	return nil, nil
}

func (f *fakeInductionRepo) ListInvitesByUser(userID uint) ([]models.InductionInvite, error) {
	var out []models.InductionInvite
	if f.invite != nil && f.invite.UserID == userID {
		out = append(out, *f.invite)
	}
	out = append(out, f.history...)
	return out, nil
}

func (f *fakeInductionRepo) GetInviteByID(id uint) (*models.InductionInvite, error) {
	if f.invite != nil && f.invite.ID == id {
		return f.invite, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeInductionRepo) ListInviteBlocks(inviteID uint) ([]models.InductionInviteBlock, error) {
	out := make([]models.InductionInviteBlock, len(f.inviteBlocks))
	copy(out, f.inviteBlocks)
	return out, nil
}

func (f *fakeInductionRepo) UpdateInviteBlock(inviteID, blockID uint, updates map[string]interface{}) error {
	if f.blockUpdates == nil {
		f.blockUpdates = map[uint]map[string]interface{}{}
	}
	f.blockUpdates[blockID] = updates
	for i := range f.inviteBlocks {
		b := &f.inviteBlocks[i]
		if b.BlockID != blockID {
			continue
		}
		if v, ok := updates["status"].(string); ok {
			b.Status = v
		}
		if v, ok := updates["attempts"].(int); ok {
			b.Attempts = v
		}
		if v, ok := updates["best_score"].(float64); ok {
			b.BestScore = v
		}
	}
	return nil
}

func (f *fakeInductionRepo) ResetInviteBlocks(inviteID uint) error {
	f.resetInvite = inviteID
	return nil
}

func (f *fakeInductionRepo) CreateAttempt(attempt *models.InductionAttempt) error {
	f.attempts = append(f.attempts, *attempt)
	return nil
}

func (f *fakeInductionRepo) ListAttempts(inviteID uint) ([]models.InductionAttempt, error) {
	return f.attempts, nil
}

func (f *fakeInductionRepo) GetSurveyWithQuestions(surveyID uint) (*models.Survey, error) {
	if s, ok := f.surveys[surveyID]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeInductionRepo) GetTutorial(tutorialID uint) (*models.Tutorial, error) {
	if t, ok := f.tutorials[tutorialID]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}

type fakeInductionUserRepo struct {
	repository.UserRepository
	users   map[uint]*models.User
	updates map[uint]map[string]interface{}
}

func (f *fakeInductionUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeInductionUserRepo) Update(user *models.User, updates map[string]interface{}) error {
	if f.updates == nil {
		f.updates = map[uint]map[string]interface{}{}
	}
	f.updates[user.ID] = updates
	return nil
}

// fakeInductionTicketSvc registra las alertas de bloqueo y los cierres de
// incorporación.
type fakeInductionTicketSvc struct {
	TicketService
	alerts []InductionAlertInput
	closed []uint
}

func (f *fakeInductionTicketSvc) CreateInductionFailureAlert(in InductionAlertInput) error {
	f.alerts = append(f.alerts, in)
	return nil
}

func (f *fakeInductionTicketSvc) CloseObersuiteHireAlert(userID uint) error {
	f.closed = append(f.closed, userID)
	return nil
}

func newInductionSvc(cfg *models.InductionConfig, users ...*models.User) (*inductionService, *fakeInductionRepo, *fakeInductionUserRepo) {
	byID := map[uint]*models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	repo := &fakeInductionRepo{
		cfg:             cfg,
		companyPrograms: map[uint]*models.InductionProgram{},
		programs:        map[uint]*models.InductionProgram{},
		blocks:          map[uint]*models.InductionBlock{},
		blockUsage:      map[uint]int64{},
		surveys:         map[uint]*models.Survey{},
		tutorials:       map[uint]*models.Tutorial{},
	}
	userRepo := &fakeInductionUserRepo{users: byID}
	// brevoSvc nil: sendInviteEmail sale sin hacer nada, así el test no manda correo.
	return &inductionService{repo: repo, userRepo: userRepo}, repo, userRepo
}

func enabledConfig() *models.InductionConfig {
	return &models.InductionConfig{ID: 1, IsActive: true, InviteTTLDays: 15}
}

// twoBlockProgram es un programa de dos bloques: el primero con mínimo propio
// (85), el segundo con el del programa (70).
func twoBlockProgram(id uint, name string) *models.InductionProgram {
	own := 85
	return &models.InductionProgram{
		ID: id, Name: name, IsActive: true, IsDefault: id == 1,
		DefaultPassingScore: 70, MaxAttempts: 3,
		Blocks: []models.InductionBlock{
			{ID: 11, Name: "Bienvenida", SurveyID: 7, PassingScore: &own},
			{ID: 12, Name: "Seguridad", SurveyID: 8},
		},
	}
}

func professional(id uint) *models.User {
	return &models.User{ID: id, UserType: models.UserTypeProfessional, Email: "pro@x.com", IsActive: true}
}

// --- Reset --------------------------------------------------------------------

// El caso que motivó el arreglo: reiniciar una invitación YA VENCIDA reenviaba
// el mismo enlace muerto, y quien lleva semanas parado es justo a quien se le
// pulsa este botón.
func TestReset_RenuevaLaVigenciaDeUnaInvitacionVencida(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	repo.invite = &models.InductionInvite{
		ID: 1, UserID: 5, Token: "viejo", Status: models.InductionBlocked,
		MaxAttempts: 3, GatesAccess: true,
		ExpiresAt: time.Now().AddDate(0, 0, -10), // venció hace 10 días
	}

	if err := svc.Reset(5); err != nil {
		t.Fatalf("reset: %v", err)
	}

	expires, ok := repo.inviteUpdate["expires_at"].(time.Time)
	if !ok {
		t.Fatal("el reset debe renovar expires_at")
	}
	if !expires.After(time.Now()) {
		t.Fatalf("la invitación renovada sigue vencida: %v", expires)
	}
	// Y respeta la vigencia configurada (15 días), no un valor fijo.
	if expires.After(time.Now().AddDate(0, 0, 16)) || expires.Before(time.Now().AddDate(0, 0, 14)) {
		t.Fatalf("la vigencia no sigue la configuración (15 días): %v", expires)
	}
	if repo.inviteUpdate["status"] != models.InductionPending {
		t.Fatalf("el reset debe reponer el estado: %v", repo.inviteUpdate)
	}
	// Los intentos viven en los bloques: se reinician todos.
	if repo.resetInvite != 1 {
		t.Fatalf("el reset debe reiniciar los bloques de la invitación 1, got %d", repo.resetInvite)
	}
	if userRepo.updates[5]["onboarding_status"] != models.OnboardingPending {
		t.Fatalf("el usuario debe volver a pending: %v", userRepo.updates[5])
	}
}

// Rotar el token invalida el enlace viejo, que pudo quedar reenviado o en un
// correo compartido.
func TestReset_RotaElToken(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.invite = &models.InductionInvite{ID: 1, UserID: 5, Token: "viejo", Status: models.InductionBlocked, MaxAttempts: 3}

	if err := svc.Reset(5); err != nil {
		t.Fatalf("reset: %v", err)
	}

	token, _ := repo.inviteUpdate["token"].(string)
	if token == "" || token == "viejo" {
		t.Fatalf("el reset debe emitir un token nuevo, got %q", token)
	}
}

// Sin configuración usable, la vigencia cae en un valor de respaldo en vez de
// emitir un enlace ya vencido.
func TestReset_VigenciaDeRespaldoSiLaConfigEstaAMedias(t *testing.T) {
	svc, repo, _ := newInductionSvc(&models.InductionConfig{ID: 1, InviteTTLDays: 0}, professional(5))
	repo.invite = &models.InductionInvite{ID: 1, UserID: 5, Token: "viejo", MaxAttempts: 3}

	if err := svc.Reset(5); err != nil {
		t.Fatalf("reset: %v", err)
	}
	expires, _ := repo.inviteUpdate["expires_at"].(time.Time)
	if !expires.After(time.Now().AddDate(0, 0, 29)) {
		t.Fatalf("se esperaba el respaldo de 30 días, got %v", expires)
	}
}

func TestReset_ErrorSiNoHayInduccion(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.inviteErr = errors.New("not found")

	if err := svc.Reset(5); err == nil {
		t.Fatal("se esperaba error si el profesional no tiene inducción")
	}
}

// --- Invite (envío manual) ----------------------------------------------------

func TestInvite_EmiteInvitacionConSnapshotYBloqueaElAcceso(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = twoBlockProgram(1, "Programa por defecto")

	if err := svc.Invite(5, 0, true); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created == nil {
		t.Fatal("se esperaba una invitación nueva")
	}
	if repo.created.UserID != 5 || repo.created.Token == "" {
		t.Fatalf("invitación mal formada: %+v", repo.created)
	}
	if repo.created.ProgramID == nil || *repo.created.ProgramID != 1 || repo.created.ProgramName != "Programa por defecto" {
		t.Fatalf("la invitación debe recordar su programa: %+v", repo.created)
	}
	// Las reglas se congelan en la invitación: cambiar el programa después no
	// altera lo que ya se emitió.
	if repo.created.MaxAttempts != 3 {
		t.Fatalf("los intentos deben congelarse: %+v", repo.created)
	}
	if len(repo.createdBlocks) != 2 {
		t.Fatalf("se esperaban 2 bloques en el snapshot, got %d", len(repo.createdBlocks))
	}
	first, second := repo.createdBlocks[0], repo.createdBlocks[1]
	if first.BlockID != 11 || first.OrderIndex != 0 || first.Name != "Bienvenida" || first.SurveyID != 7 {
		t.Fatalf("primer bloque mal congelado: %+v", first)
	}
	// El mínimo efectivo se resuelve al invitar: propio del bloque o del programa.
	if first.PassingScore != 85 || second.PassingScore != 70 {
		t.Fatalf("mínimos mal resueltos: %d y %d", first.PassingScore, second.PassingScore)
	}
	if first.Status != models.InductionPending || second.Status != models.InductionPending {
		t.Fatalf("los bloques nacen pendientes: %+v %+v", first, second)
	}
	if userRepo.updates[5]["onboarding_status"] != models.OnboardingPending {
		t.Fatalf("invitar debe dejar al profesional sin acceso: %v", userRepo.updates[5])
	}
}

// Una sola capacitación EN CURSO: un ingreso reemplaza la pendiente; una
// capacitación tiene que esperar a que termine.
func TestInvite_UnaSolaPendientePorPersona(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingNotRequired
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	repo.invite = &models.InductionInvite{ID: 9, UserID: 5, Token: "en-curso", Status: models.InductionPending, GatesAccess: false}

	if err := svc.Invite(5, 0, false); err == nil {
		t.Fatal("con una capacitación en curso no se manda otra")
	}
	if repo.created != nil {
		t.Fatal("no debe emitirse nada")
	}
	// Un ingreso sí la reemplaza.
	if err := svc.Invite(5, 0, true); err != nil {
		t.Fatalf("el ingreso debe reemplazar la pendiente: %v", err)
	}
	if repo.deletedInvite != 9 || repo.created == nil {
		t.Fatalf("debe borrar la pendiente (9) y crear la nueva: deleted=%d created=%v", repo.deletedInvite, repo.created != nil)
	}
}

// Las terminadas se conservan: mandar otro programa a quien ya aprobó uno
// no borra nada.
func TestInvite_ConservaLasTerminadas(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingPassed
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Repaso")
	repo.invite = &models.InductionInvite{ID: 9, UserID: 5, Token: "vieja", Status: models.InductionPassed, GatesAccess: true}

	if err := svc.Invite(5, 0, false); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.deletedInvite != 0 {
		t.Fatalf("la aprobada no debe borrarse: %d", repo.deletedInvite)
	}
	if repo.created == nil || repo.created.GatesAccess {
		t.Fatalf("debe crearse una capacitación nueva sin bloqueo: %+v", repo.created)
	}
}

func TestStatus_IncluyeElHistorial(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	done := time.Now().AddDate(0, 0, -30)
	repo.history = []models.InductionInvite{{ID: 2, UserID: 5, ProgramName: "Ingreso", Status: models.InductionPassed, GatesAccess: true, CompletedAt: &done}}

	view, err := svc.Status(5)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if view.InviteID != 1 || len(view.History) != 2 {
		t.Fatalf("historial: %+v", view.History)
	}
	if !view.History[0].Current || view.History[1].Current || view.History[1].ProgramName != "Ingreso" {
		t.Fatalf("la actual debe ir marcada: %+v", view.History)
	}
}

// Una aprobada no se reinicia: para repetirla se envía de nuevo.
func TestReset_RechazaUnaAprobada(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.invite = &models.InductionInvite{ID: 1, UserID: 5, Token: "t", Status: models.InductionPassed, MaxAttempts: 3}
	if err := svc.Reset(5); err == nil {
		t.Fatal("una aprobada no se reinicia")
	}
	if err := svc.ResetInvite(1); err == nil {
		t.Fatal("una aprobada no se reinicia (por id)")
	}
}

// La empresa contratante decide el programa; sin asignación, el por defecto.
func TestInvite_UsaElProgramaDeLaEmpresa(t *testing.T) {
	pro := professional(5)
	company := uint(10)
	pro.EmpleadorID = &company
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	repo.companyPrograms[10] = twoBlockProgram(2, "Acme")

	if err := svc.Invite(5, 0, true); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created.ProgramID == nil || *repo.created.ProgramID != 2 {
		t.Fatalf("debe usar el programa de la empresa (2), got %+v", repo.created.ProgramID)
	}
}

// Un programa apagado o vacío no se emite: se cae al por defecto en vez de
// dejar a la persona sin inducción y sin acceso.
func TestInvite_EmpresaConProgramaNoUsableCaeAlPorDefecto(t *testing.T) {
	pro := professional(5)
	company := uint(10)
	pro.EmpleadorID = &company
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	apagado := twoBlockProgram(2, "Acme")
	apagado.IsActive = false
	repo.companyPrograms[10] = apagado

	if err := svc.Invite(5, 0, true); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created.ProgramID == nil || *repo.created.ProgramID != 1 {
		t.Fatalf("debe caer al programa por defecto (1), got %+v", repo.created.ProgramID)
	}
}

// Soporte puede elegir un programa concreto sin tocar la asignación.
func TestInvite_ProgramaExplicito(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	repo.programs[3] = twoBlockProgram(3, "Especial")

	if err := svc.Invite(5, 3, true); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created.ProgramID == nil || *repo.created.ProgramID != 3 {
		t.Fatalf("debe usar el programa pedido (3), got %+v", repo.created.ProgramID)
	}
}

func TestInvite_ErrorSiElProgramaExplicitoNoEsUsable(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	vacio := twoBlockProgram(3, "Vacío")
	vacio.Blocks = nil
	repo.programs[3] = vacio

	if err := svc.Invite(5, 3, true); err == nil {
		t.Fatal("se esperaba error con un programa sin bloques")
	}
	if repo.created != nil {
		t.Fatal("no debería emitirse ninguna invitación")
	}
}

// Invitar a quien ya aprobó le quitaría el acceso que se ganó.
func TestInvite_RechazaAQuienYaAprobo(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingPassed
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")

	if err := svc.Invite(5, 0, true); err == nil {
		t.Fatal("se esperaba error al invitar a quien ya aprobó")
	}
	if repo.created != nil {
		t.Fatal("no debería emitirse ninguna invitación")
	}
}

// Una cuenta empresa o de soporte no pasa por inducción: invitarla le cortaría
// el acceso sin motivo.
func TestInvite_RechazaAQuienNoEsProfesional(t *testing.T) {
	empresa := &models.User{ID: 5, UserType: models.UserTypeEmployer}
	svc, repo, userRepo := newInductionSvc(enabledConfig(), empresa)
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")

	if err := svc.Invite(5, 0, true); err == nil {
		t.Fatal("se esperaba error al invitar a una cuenta que no es profesional")
	}
	if repo.created != nil || userRepo.updates[5] != nil {
		t.Fatal("no debería tocarse nada")
	}
}

// A diferencia del alta automática (que sigue de largo si la inducción está
// apagada), aquí es una acción explícita: hay que decir que no se puede.
func TestInvite_ErrorSiLaInduccionEstaApagada(t *testing.T) {
	svc, repo, _ := newInductionSvc(&models.InductionConfig{ID: 1, IsActive: false}, professional(5))
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")

	if err := svc.Invite(5, 0, true); err == nil {
		t.Fatal("se esperaba error con la inducción apagada")
	}
	if repo.created != nil {
		t.Fatal("no debería emitirse ninguna invitación")
	}
}

func TestInvite_ErrorSiNoHayProgramaUsable(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = nil

	if err := svc.Invite(5, 0, true); err == nil {
		t.Fatal("se esperaba error sin programa por defecto")
	}
}

func TestInvite_ErrorSiElUsuarioNoExiste(t *testing.T) {
	svc, _, _ := newInductionSvc(enabledConfig())

	if err := svc.Invite(404, 0, true); err == nil {
		t.Fatal("se esperaba error con un usuario inexistente")
	}
}

// --- InviteIfEnabled (puente de Obersuite) ------------------------------------

// Encendida pero sin programa usable = apagada: el alta sigue el flujo directo
// y no se rompe el puente.
func TestInviteIfEnabled_FalseSinProgramaUsable(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = &models.InductionProgram{ID: 1, IsActive: true}

	invited, err := svc.InviteIfEnabled(professional(5))
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if invited {
		t.Fatal("sin bloques no debe emitirse la inducción")
	}
	if userRepo.updates[5] != nil {
		t.Fatal("el acceso no debe tocarse")
	}
}

func TestInviteIfEnabled_EmiteConElProgramaPorDefecto(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")

	invited, err := svc.InviteIfEnabled(professional(5))
	if err != nil || !invited {
		t.Fatalf("se esperaba invitación, got invited=%v err=%v", invited, err)
	}
	if len(repo.createdBlocks) != 2 {
		t.Fatalf("snapshot de 2 bloques, got %d", len(repo.createdBlocks))
	}
}

// --- Submit (landing) ---------------------------------------------------------

// pendingInvite deja en el fake una invitación en curso de dos bloques, con
// un cuestionario de una pregunta cada uno (7: "a"; 8: "b").
func pendingInvite(repo *fakeInductionRepo, maxAttempts int) {
	// Invitación de INGRESO (bloquea el acceso), que es el caso base.
	repo.invite = &models.InductionInvite{
		ID: 1, UserID: 5, Token: "tok", Status: models.InductionPending,
		ProgramName: "Por defecto", MaxAttempts: maxAttempts, GatesAccess: true,
		ExpiresAt: time.Now().AddDate(0, 0, 10),
	}
	repo.inviteBlocks = []models.InductionInviteBlock{
		{InviteID: 1, BlockID: 11, OrderIndex: 0, Name: "Bienvenida", SurveyID: 7, PassingScore: 85, Status: models.InductionPending},
		{InviteID: 1, BlockID: 12, OrderIndex: 1, Name: "Seguridad", SurveyID: 8, PassingScore: 70, Status: models.InductionPending},
	}
	repo.surveys[7] = &models.Survey{ID: 7, Title: "Bienvenida", Questions: []models.SurveyQuestion{q(71, "a", 1)}}
	repo.surveys[8] = &models.Survey{ID: 8, Title: "Seguridad", Questions: []models.SurveyQuestion{q(81, "b", 1)}}
}

func TestSubmit_ApruebaElBloqueYAvanzaAlSiguiente(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)

	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "a"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !res.Passed || res.BlockStatus != models.InductionPassed || res.Completed {
		t.Fatalf("debe aprobar el bloque sin completar el programa: %+v", res)
	}
	if res.BlockIndex != 1 || res.NextBlockID == nil || *res.NextBlockID != 12 || res.NextBlock != "Seguridad" {
		t.Fatalf("debe anunciar el siguiente bloque: %+v", res)
	}
	if res.Status != models.InductionPending {
		t.Fatalf("la inducción sigue pendiente: %+v", res)
	}
	if repo.inviteBlocks[0].Status != models.InductionPassed || repo.inviteBlocks[0].Attempts != 1 {
		t.Fatalf("el bloque debe quedar aprobado con 1 intento: %+v", repo.inviteBlocks[0])
	}
	// Todavía sin acceso: falta el segundo bloque.
	if userRepo.updates[5] != nil {
		t.Fatalf("no debe tocarse el acceso a mitad del programa: %v", userRepo.updates[5])
	}
	if len(repo.attempts) != 1 || repo.attempts[0].BlockID != 11 || !repo.attempts[0].Passed {
		t.Fatalf("el intento debe registrarse con su bloque: %+v", repo.attempts)
	}
}

// No se puede saltar hacia adelante ni reenviar un bloque aprobado.
func TestSubmit_RechazaUnBloqueQueNoToca(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)

	if _, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "b"}}); err == nil {
		t.Fatal("se esperaba error al enviar el bloque 2 con el 1 pendiente")
	}
	if len(repo.attempts) != 0 {
		t.Fatal("no debe registrarse ningún intento")
	}
}

func TestSubmit_ElUltimoBloqueApruebaElProgramaYHabilitaElAcceso(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	tickets := &fakeInductionTicketSvc{}
	svc.ticketSvc = tickets
	pendingInvite(repo, 3)
	repo.inviteBlocks[0].Status = models.InductionPassed

	res, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "b"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !res.Passed || !res.Completed || res.Status != models.InductionPassed || res.NextBlockID != nil {
		t.Fatalf("debe completar el programa: %+v", res)
	}
	if repo.invite.Status != models.InductionPassed || repo.inviteUpdate["completed_at"] == nil {
		t.Fatalf("la invitación debe quedar aprobada: %+v %v", repo.invite, repo.inviteUpdate)
	}
	if userRepo.updates[5]["onboarding_status"] != models.OnboardingPassed {
		t.Fatalf("aprobar el último bloque habilita el acceso: %v", userRepo.updates[5])
	}
	// Aprobar cierra la incorporación de Obersuite.
	if len(tickets.closed) != 1 || tickets.closed[0] != 5 {
		t.Fatalf("debe cerrarse el ticket de incorporación: %v", tickets.closed)
	}
}

func TestSubmit_FallaConIntentosRestantesSigueEnElMismoBloque(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)

	res, err := svc.Submit("tok", 11, []SubmittedAnswer{{QuestionID: 71, Value: "mal"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if res.Passed || res.BlockStatus != models.InductionPending || res.AttemptsLeft != 2 {
		t.Fatalf("debe quedar pendiente con 2 intentos: %+v", res)
	}
	if repo.inviteBlocks[0].Attempts != 1 || repo.inviteBlocks[0].Status != models.InductionPending {
		t.Fatalf("el bloque sigue pendiente con 1 intento: %+v", repo.inviteBlocks[0])
	}
	if userRepo.updates[5] != nil {
		t.Fatal("el acceso no se toca mientras queden intentos")
	}
	// El siguiente envío sigue siendo sobre el mismo bloque.
	if _, err := svc.Submit("tok", 12, nil); err == nil {
		t.Fatal("el bloque 2 no debe abrirse sin aprobar el 1")
	}
}

// Agotar los intentos de CUALQUIER bloque bloquea al profesional, y la alerta
// dice en cuál: los intentos son por bloque, no por programa.
func TestSubmit_AgotarIntentosEnUnBloqueIntermedioBloquea(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))
	tickets := &fakeInductionTicketSvc{}
	svc.ticketSvc = tickets
	pendingInvite(repo, 2)
	repo.inviteBlocks[0].Status = models.InductionPassed
	repo.inviteBlocks[0].Attempts = 2 // gastó los dos en el primero y pasó
	repo.inviteBlocks[1].Attempts = 1 // ya falló una vez el segundo

	res, err := svc.Submit("tok", 12, []SubmittedAnswer{{QuestionID: 81, Value: "mal"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if res.Status != models.InductionBlocked || res.BlockStatus != models.InductionBlocked || res.AttemptsLeft != 0 {
		t.Fatalf("debe bloquear: %+v", res)
	}
	if repo.invite.Status != models.InductionBlocked {
		t.Fatalf("la invitación debe quedar bloqueada: %+v", repo.invite)
	}
	if userRepo.updates[5]["onboarding_status"] != models.OnboardingBlocked {
		t.Fatalf("el usuario debe quedar bloqueado: %v", userRepo.updates[5])
	}
	if len(tickets.alerts) != 1 {
		t.Fatalf("se esperaba una alerta en Soporte, got %d", len(tickets.alerts))
	}
	alert := tickets.alerts[0]
	if alert.BlockName != "Seguridad" || alert.BlockIndex != 2 || alert.BlockCount != 2 {
		t.Fatalf("la alerta debe decir en qué bloque cayó: %+v", alert)
	}
	if alert.PassingScore != 70 || alert.Attempts != 2 {
		t.Fatalf("la alerta lleva el mínimo y los intentos del bloque: %+v", alert)
	}
}

func TestSubmit_ErrorSiYaSeResolvio(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	repo.invite.Status = models.InductionPassed
	if _, err := svc.Submit("tok", 11, nil); err == nil {
		t.Fatal("una inducción aprobada no acepta más envíos")
	}
	repo.invite.Status = models.InductionBlocked
	if _, err := svc.Submit("tok", 11, nil); err == nil {
		t.Fatal("una inducción bloqueada no acepta más envíos")
	}
}

func TestSubmit_ErrorSiElEnlaceVencio(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	repo.invite.ExpiresAt = time.Now().AddDate(0, 0, -1)
	if _, err := svc.Submit("tok", 11, nil); err == nil {
		t.Fatal("un enlace vencido no debe calificar nada")
	}
}

// --- Landing ------------------------------------------------------------------

func TestLanding_DevuelveElBloqueActualSinRespuestas(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	repo.inviteBlocks[0].Status = models.InductionPassed
	tutorial := uint(40)
	repo.inviteBlocks[1].TutorialID = &tutorial
	repo.tutorials[40] = &models.Tutorial{ID: 40, Title: "Video de seguridad", GoogleDriveURL: "https://youtu.be/abc", DurationMin: 4}

	view, err := svc.Landing("tok")
	if err != nil {
		t.Fatalf("landing: %v", err)
	}
	if view.TotalBlocks != 2 || view.CompletedBlocks != 1 || len(view.Blocks) != 2 {
		t.Fatalf("progreso mal contado: %+v", view)
	}
	if view.Current == nil || view.Current.BlockID != 12 || view.Current.OrderIndex != 1 {
		t.Fatalf("el bloque actual debe ser el segundo: %+v", view.Current)
	}
	if view.Current.VideoURL != "https://youtu.be/abc" || view.Current.VideoTitle != "Video de seguridad" {
		t.Fatalf("el video del bloque debe viajar: %+v", view.Current)
	}
	if len(view.Current.Questions) != 1 || view.Current.Questions[0].ID != 81 {
		t.Fatalf("deben viajar las preguntas del bloque actual: %+v", view.Current.Questions)
	}
	if view.Current.PassingScore != 70 || view.Current.AttemptsLeft != 3 {
		t.Fatalf("reglas del bloque: %+v", view.Current)
	}
}

func TestLanding_SinBloqueActualCuandoYaSeResolvio(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig(), professional(5))
	pendingInvite(repo, 3)
	repo.invite.Status = models.InductionBlocked

	view, err := svc.Landing("tok")
	if err != nil {
		t.Fatalf("landing: %v", err)
	}
	if view.Current != nil {
		t.Fatalf("una inducción resuelta no expone preguntas: %+v", view.Current)
	}
	if view.Status != models.InductionBlocked {
		t.Fatalf("estado: %s", view.Status)
	}
}

// --- Programas y bloques (armado) ---------------------------------------------

// El primer programa es el por defecto aunque no lo pidan: sin uno, la
// inducción no tiene a quién mandar a nadie.
func TestCreateProgram_ElPrimeroEsElPorDefecto(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig())
	repo.defaultProgram = nil

	if _, err := svc.CreateProgram(1, ProgramInput{Name: "Primero", DefaultPassingScore: 70, MaxAttempts: 3, IsActive: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.defaultSet != 99 {
		t.Fatalf("el primer programa debe marcarse por defecto, got %d", repo.defaultSet)
	}
}

func TestCreateProgram_ValidaLasReglas(t *testing.T) {
	svc, _, _ := newInductionSvc(enabledConfig())
	cases := []ProgramInput{
		{Name: "", DefaultPassingScore: 70, MaxAttempts: 3},
		{Name: "x", DefaultPassingScore: 101, MaxAttempts: 3},
		{Name: "x", DefaultPassingScore: 70, MaxAttempts: 0},
	}
	for i, in := range cases {
		if _, err := svc.CreateProgram(1, in); err == nil {
			t.Errorf("caso %d: se esperaba error de validación", i)
		}
	}
}

// Quitarle la marca al programa por defecto (o apagarlo) dejaría la inducción
// encendida sin destino.
func TestUpdateProgram_ProtegeAlPorDefecto(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig())
	repo.programs[1] = twoBlockProgram(1, "Por defecto")

	if _, err := svc.UpdateProgram(1, ProgramInput{Name: "Por defecto", DefaultPassingScore: 70, MaxAttempts: 3, IsDefault: false, IsActive: true}); err == nil {
		t.Fatal("no debe poder desmarcarse el por defecto")
	}
	if _, err := svc.UpdateProgram(1, ProgramInput{Name: "Por defecto", DefaultPassingScore: 70, MaxAttempts: 3, IsDefault: true, IsActive: false}); err == nil {
		t.Fatal("no debe poder apagarse el por defecto")
	}
}

func TestDeleteProgram_RechazaAlPorDefecto(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig())
	repo.programs[1] = twoBlockProgram(1, "Por defecto")
	if err := svc.DeleteProgram(1); err == nil {
		t.Fatal("el programa por defecto no se borra")
	}
}

// Un bloque en uso no se borra: dejaría un hueco en un programa que alguien
// puede estar recibiendo ahora mismo.
func TestDeleteBlock_RechazaSiEstaEnUso(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig())
	repo.blocks[11] = &models.InductionBlock{ID: 11, Name: "Bienvenida", SurveyID: 7}
	repo.blockUsage[11] = 1

	if err := svc.DeleteBlock(11); err == nil {
		t.Fatal("se esperaba error al borrar un bloque en uso")
	}
	if _, ok := repo.blocks[11]; !ok {
		t.Fatal("el bloque no debe borrarse")
	}

	repo.blockUsage[11] = 0
	if err := svc.DeleteBlock(11); err != nil {
		t.Fatalf("sin uso debe borrarse: %v", err)
	}
}

// Encender la inducción sin un programa por defecto usable dejaría a todo
// profesional nuevo sin poder entrar nunca.
func TestSaveConfig_NoEnciendeSinProgramaUsable(t *testing.T) {
	svc, repo, _ := newInductionSvc(&models.InductionConfig{ID: 1})
	repo.defaultProgram = &models.InductionProgram{ID: 1, IsActive: true} // sin bloques

	if err := svc.SaveConfig(&models.InductionConfig{IsActive: true, InviteTTLDays: 30}); err == nil {
		t.Fatal("no debe encenderse sin programa usable")
	}
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	if err := svc.SaveConfig(&models.InductionConfig{IsActive: true, InviteTTLDays: 30}); err != nil {
		t.Fatalf("con programa usable debe encenderse: %v", err)
	}
	// Apagar siempre se puede.
	repo.defaultProgram = nil
	if err := svc.SaveConfig(&models.InductionConfig{IsActive: false}); err != nil {
		t.Fatalf("apagar no depende de los programas: %v", err)
	}
}

func TestEnabled_RequiereInterruptorYProgramaUsable(t *testing.T) {
	svc, repo, _ := newInductionSvc(enabledConfig())
	repo.defaultProgram = nil
	if svc.Enabled() {
		t.Fatal("sin programa por defecto no está habilitada")
	}
	repo.defaultProgram = twoBlockProgram(1, "Por defecto")
	if !svc.Enabled() {
		t.Fatal("con interruptor y programa usable debe estar habilitada")
	}
	repo.cfg.IsActive = false
	if svc.Enabled() {
		t.Fatal("con el interruptor apagado no está habilitada")
	}
}
