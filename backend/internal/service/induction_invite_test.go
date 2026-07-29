package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Reset y envío manual de la inducción. Mismo patrón de fakes que el resto del
// paquete: se embebe la interfaz real y se sobrescribe solo lo que toca el
// camino bajo prueba.

type fakeInductionRepo struct {
	repository.InductionRepository

	cfg *models.InductionConfig

	invite       *models.InductionInvite
	inviteErr    error
	created      *models.InductionInvite
	inviteUpdate map[string]interface{}
	deletedUser  uint
}

func (f *fakeInductionRepo) GetConfig() (*models.InductionConfig, error) {
	if f.cfg == nil {
		return &models.InductionConfig{ID: 1, PassingScore: 70, MaxAttempts: 3, InviteTTLDays: 30}, nil
	}
	return f.cfg, nil
}

func (f *fakeInductionRepo) GetInviteByUser(userID uint) (*models.InductionInvite, error) {
	if f.inviteErr != nil {
		return nil, f.inviteErr
	}
	return f.invite, nil
}

func (f *fakeInductionRepo) UpdateInvite(_ *models.InductionInvite, updates map[string]interface{}) error {
	f.inviteUpdate = updates
	return nil
}

func (f *fakeInductionRepo) CreateInvite(invite *models.InductionInvite) error {
	f.created = invite
	return nil
}

func (f *fakeInductionRepo) DeleteInviteByUser(userID uint) error {
	f.deletedUser = userID
	return nil
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

func newInductionSvc(cfg *models.InductionConfig, users ...*models.User) (*inductionService, *fakeInductionRepo, *fakeInductionUserRepo) {
	byID := map[uint]*models.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	repo := &fakeInductionRepo{cfg: cfg}
	userRepo := &fakeInductionUserRepo{users: byID}
	// brevoSvc nil: sendInviteEmail sale sin hacer nada, así el test no manda correo.
	return &inductionService{repo: repo, userRepo: userRepo}, repo, userRepo
}

func enabledConfig() *models.InductionConfig {
	surveyID := uint(7)
	return &models.InductionConfig{
		ID: 1, IsActive: true, SurveyID: &surveyID,
		PassingScore: 70, MaxAttempts: 3, InviteTTLDays: 15,
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
		Attempts: 3, MaxAttempts: 3,
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
	if repo.inviteUpdate["attempts"] != 0 || repo.inviteUpdate["status"] != models.InductionPending {
		t.Fatalf("el reset debe reponer intentos y estado: %v", repo.inviteUpdate)
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

func TestInvite_EmiteInvitacionYBloqueaElAcceso(t *testing.T) {
	svc, repo, userRepo := newInductionSvc(enabledConfig(), professional(5))

	if err := svc.Invite(5); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if repo.created == nil {
		t.Fatal("se esperaba una invitación nueva")
	}
	if repo.created.UserID != 5 || repo.created.Token == "" {
		t.Fatalf("invitación mal formada: %+v", repo.created)
	}
	// Las reglas se congelan en la invitación: cambiar la config después no
	// altera lo que ya se emitió.
	if repo.created.MaxAttempts != 3 || repo.created.PassingScore != 70 {
		t.Fatalf("las reglas deben congelarse: %+v", repo.created)
	}
	if userRepo.updates[5]["onboarding_status"] != models.OnboardingPending {
		t.Fatalf("invitar debe dejar al profesional sin acceso: %v", userRepo.updates[5])
	}
}

// Invitar a quien ya aprobó le quitaría el acceso que se ganó.
func TestInvite_RechazaAQuienYaAprobo(t *testing.T) {
	pro := professional(5)
	pro.OnboardingStatus = models.OnboardingPassed
	svc, repo, _ := newInductionSvc(enabledConfig(), pro)

	if err := svc.Invite(5); err == nil {
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

	if err := svc.Invite(5); err == nil {
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

	err := svc.Invite(5)
	if err == nil {
		t.Fatal("se esperaba error con la inducción apagada")
	}
	if repo.created != nil {
		t.Fatal("no debería emitirse ninguna invitación")
	}
}

func TestInvite_ErrorSiElUsuarioNoExiste(t *testing.T) {
	svc, _, _ := newInductionSvc(enabledConfig())

	if err := svc.Invite(404); err == nil {
		t.Fatal("se esperaba error con un usuario inexistente")
	}
}
