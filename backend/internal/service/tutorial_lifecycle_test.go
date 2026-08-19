package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// ── Dobles ───────────────────────────────────────────────────────────────────

type fakeLifecycleRepo struct {
	repository.TutorialRepository
	tutorials map[uint]*models.Tutorial
	views     map[uint][]models.TutorialView
	updates   []map[string]interface{}
}

func (f *fakeLifecycleRepo) GetByID(id uint) (*models.Tutorial, error) {
	t, ok := f.tutorials[id]
	if !ok {
		return nil, errFakeNotFound
	}
	copy := *t
	return &copy, nil
}

func (f *fakeLifecycleRepo) ViewsFor(tutorialID uint) ([]models.TutorialView, error) {
	return f.views[tutorialID], nil
}

func (f *fakeLifecycleRepo) UsersInGroups(groupIDs []uint) (map[uint]bool, error) {
	return map[uint]bool{}, nil
}

func (f *fakeLifecycleRepo) SetFields(id uint, fields map[string]interface{}) error {
	f.updates = append(f.updates, fields)
	tutorial, ok := f.tutorials[id]
	if !ok {
		return errFakeNotFound
	}
	if active, ok := fields["is_active"].(bool); ok {
		tutorial.IsActive = active
	}
	if at, ok := fields["announced_at"].(time.Time); ok {
		tutorial.AnnouncedAt = &at
	}
	if at, ok := fields["reminded_at"].(time.Time); ok {
		tutorial.RemindedAt = &at
	}
	return nil
}

func (f *fakeLifecycleRepo) ListDuePublications(now time.Time) ([]models.Tutorial, error) {
	var due []models.Tutorial
	for _, t := range f.tutorials {
		if !t.IsActive && t.PublishAt != nil && !t.PublishAt.After(now) && t.AnnouncedAt == nil {
			due = append(due, *t)
		}
	}
	return due, nil
}

func (f *fakeLifecycleRepo) ListDueExpirations(now time.Time) ([]models.Tutorial, error) {
	var due []models.Tutorial
	for _, t := range f.tutorials {
		if t.IsActive && t.ExpiresAt != nil && !t.ExpiresAt.After(now) {
			due = append(due, *t)
		}
	}
	return due, nil
}

type errNotFound struct{}

func (errNotFound) Error() string { return "no encontrado" }

var errFakeNotFound = errNotFound{}

type recordingNotifier struct {
	NotificationService
	sent []uint
}

func (r *recordingNotifier) CreateNotification(userID uint, notifType, title, message string, data map[string]interface{}) error {
	r.sent = append(r.sent, userID)
	return nil
}

func lifecycleUsers() []models.User {
	return []models.User{
		{ID: 1, Name: "Ana", UserType: models.UserTypeProfessional},
		{ID: 2, Name: "Luis", UserType: models.UserTypeProfessional},
		{ID: 3, Name: "Sara", UserType: models.UserTypeProfessional},
		{ID: 9, Name: "Root", UserType: models.UserTypeSuperadmin},
	}
}

// ── Recordatorio ─────────────────────────────────────────────────────────────

func TestRemindPendingSkipsWhoAlreadySawIt(t *testing.T) {
	announced := time.Now().Add(-48 * time.Hour)
	repo := &fakeLifecycleRepo{
		tutorials: map[uint]*models.Tutorial{
			1: {ID: 1, Title: "Cambio de pago", Audience: models.TutorialAudienceAll, IsActive: true, AnnouncedAt: &announced, AnnounceDays: 2},
		},
		views: map[uint][]models.TutorialView{
			1: {{TutorialID: 1, UserID: 2, Source: models.TutorialViewFromSection}},
		},
	}
	notifier := &recordingNotifier{}
	svc := NewTutorialService(repo, &fakeAudienceUserRepo{users: lifecycleUsers()}, notifier)

	reminded, err := svc.RemindPending(9, 1)
	if err != nil {
		t.Fatalf("recordatorio falló: %v", err)
	}
	// Ana y Sara: Luis ya la vio y el superadmin nunca cuenta.
	if reminded != 2 {
		t.Errorf("se recordó a %d personas, esperaba 2", reminded)
	}
	for _, id := range notifier.sent {
		if id == 2 {
			t.Error("no debería recordarse a quien ya la vio")
		}
		if id == 9 {
			t.Error("el superadmin no debería recibir el recordatorio")
		}
	}

	// Reabrir la ventana es la mitad del recordatorio: sin eso solo llegaría
	// una campanita más y el aviso no volvería a emerger.
	var reopened bool
	for _, update := range repo.updates {
		if _, ok := update["announced_at"]; ok {
			reopened = true
		}
	}
	if !reopened {
		t.Error("el recordatorio debería reabrir la ventana del aviso")
	}
}

func TestRemindPendingRespectsCooldownAndDraft(t *testing.T) {
	announced := time.Now().Add(-48 * time.Hour)
	justReminded := time.Now().Add(-time.Minute)
	repo := &fakeLifecycleRepo{
		tutorials: map[uint]*models.Tutorial{
			1: {ID: 1, Audience: models.TutorialAudienceAll, IsActive: true, AnnouncedAt: &announced, RemindedAt: &justReminded},
			2: {ID: 2, Audience: models.TutorialAudienceAll, IsActive: false},
		},
	}
	svc := NewTutorialService(repo, &fakeAudienceUserRepo{users: lifecycleUsers()}, &recordingNotifier{})

	// Dos pulsaciones seguidas mandarían dos avisos a media empresa.
	if _, err := svc.RemindPending(9, 1); err == nil {
		t.Error("un recordatorio inmediato debería rechazarse")
	}
	// Y no se puede recordar algo que todavía no se publicó.
	if _, err := svc.RemindPending(9, 2); err == nil {
		t.Error("no debería poder recordarse un borrador")
	}
}

// ── Reloj de publicación ─────────────────────────────────────────────────────

func TestRunSchedulePublishesAndExpires(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(24 * time.Hour)
	repo := &fakeLifecycleRepo{
		tutorials: map[uint]*models.Tutorial{
			1: {ID: 1, Title: "Programada", Audience: models.TutorialAudienceAll, IsActive: false, PublishAt: &past},
			2: {ID: 2, Title: "Aún no toca", Audience: models.TutorialAudienceAll, IsActive: false, PublishAt: &future},
			3: {ID: 3, Title: "Caducada", Audience: models.TutorialAudienceAll, IsActive: true, ExpiresAt: &past},
		},
	}
	notifier := &recordingNotifier{}
	svc := NewTutorialService(repo, &fakeAudienceUserRepo{users: lifecycleUsers()}, notifier)

	published, expired, err := svc.RunSchedule()
	if err != nil {
		t.Fatalf("pasada falló: %v", err)
	}
	if published != 1 || expired != 1 {
		t.Errorf("publicadas=%d retiradas=%d, esperaba 1 y 1", published, expired)
	}
	if !repo.tutorials[1].IsActive || repo.tutorials[1].AnnouncedAt == nil {
		t.Error("la novedad programada debería quedar publicada y anunciada")
	}
	if repo.tutorials[2].IsActive {
		t.Error("la que aún no toca no debería publicarse")
	}
	if repo.tutorials[3].IsActive {
		t.Error("la caducada debería retirarse")
	}

	// Publicar es anunciar: la segunda pasada no debe repetir el reparto.
	repo.updates = nil
	published, _, err = svc.RunSchedule()
	if err != nil {
		t.Fatalf("segunda pasada falló: %v", err)
	}
	if published != 0 {
		t.Errorf("la segunda pasada publicó %d, esperaba 0", published)
	}
}

// ── Validaciones del formulario ──────────────────────────────────────────────

func TestValidateCTA(t *testing.T) {
	if _, _, err := validateCTA("", ""); err != nil {
		t.Errorf("sin botón no debería haber error: %v", err)
	}
	if _, url, err := validateCTA("  Ir a horas  ", "  /work-hours  "); err != nil || url != "/work-hours" {
		t.Errorf("botón válido rechazado: url=%q err=%v", url, err)
	}
	// Las dos mitades van juntas.
	if _, _, err := validateCTA("Ir", ""); err == nil {
		t.Error("un texto sin destino es un botón muerto")
	}
	if _, _, err := validateCTA("", "/work-hours"); err == nil {
		t.Error("un destino sin texto no se ve")
	}
	// Un javascript: en un botón que ve toda la empresa no es una opción.
	for _, url := range []string{"javascript:alert(1)", "data:text/html,x", "work-hours"} {
		if _, _, err := validateCTA("Ir", url); err == nil {
			t.Errorf("destino inválido aceptado: %q", url)
		}
	}
}

func TestValidateSchedule(t *testing.T) {
	publish := time.Now().Add(time.Hour)
	after := publish.Add(time.Hour)
	before := publish.Add(-time.Hour)

	if err := validateSchedule(nil, nil); err != nil {
		t.Errorf("sin programación no debería haber error: %v", err)
	}
	if err := validateSchedule(&publish, &after); err != nil {
		t.Errorf("programación válida rechazada: %v", err)
	}
	// Caducar antes de publicar deja una novedad que nace muerta.
	if err := validateSchedule(&publish, &before); err == nil {
		t.Error("el retiro anterior a la publicación debería rechazarse")
	}
}
