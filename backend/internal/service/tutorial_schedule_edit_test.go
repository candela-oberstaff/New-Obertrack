package service

import (
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// Editar la programación de una novedad: es donde se rompía. Quitar la fecha
// no se guardaba y marcar "visible" una programada la publicaba al instante.

type fakeEditRepo struct {
	fakeLifecycleRepo
	saved map[string]interface{}
}

func (f *fakeEditRepo) Update(tutorial *models.Tutorial, updates map[string]interface{}) error {
	f.saved = updates
	return nil
}

func editFixture(t *models.Tutorial) (*fakeEditRepo, TutorialService) {
	repo := &fakeEditRepo{fakeLifecycleRepo: fakeLifecycleRepo{tutorials: map[uint]*models.Tutorial{t.ID: t}}}
	return repo, NewTutorialService(repo, &fakeAudienceUserRepo{users: lifecycleUsers()}, &recordingNotifier{})
}

func TestUpdateKeepsScheduledNovedadWaiting(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	repo, svc := editFixture(&models.Tutorial{ID: 1, Title: "Lunes", ContentType: models.TutorialContentVideo, GoogleDriveURL: "https://youtu.be/dQw4w9WgXcQ", IsActive: false, PublishAt: &future})

	// El formulario la abre como visible: guardarla no debe publicarla ya.
	if _, err := svc.Update(9, 1, map[string]interface{}{"is_active": true, "publish_at": future}); err != nil {
		t.Fatalf("editar falló: %v", err)
	}
	if active, _ := repo.saved["is_active"].(bool); active {
		t.Error("una programada debe seguir esperando su hora")
	}
	if _, announced := repo.saved["announced_at"]; announced {
		t.Error("no debe anunciarse antes de tiempo")
	}
}

func TestUpdateClearingScheduleePublishesNow(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	repo, svc := editFixture(&models.Tutorial{ID: 1, Title: "Lunes", ContentType: models.TutorialContentVideo, GoogleDriveURL: "https://youtu.be/dQw4w9WgXcQ", IsActive: false, PublishAt: &future})

	if _, err := svc.Update(9, 1, map[string]interface{}{"is_active": true, "publish_at": nil}); err != nil {
		t.Fatalf("editar falló: %v", err)
	}
	if v, ok := repo.saved["publish_at"]; !ok || v != nil {
		t.Errorf("quitar la fecha debe guardarse como NULL, quedó %v", v)
	}
	if active, _ := repo.saved["is_active"].(bool); !active {
		t.Error("sin programación y visible, se publica al guardar")
	}
	if _, announced := repo.saved["announced_at"]; !announced {
		t.Error("publicarla es anunciarla")
	}
}

func TestUpdateRejectsReschedulingPublished(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(24 * time.Hour)
	_, svc := editFixture(&models.Tutorial{ID: 1, Title: "Ya salió", IsActive: true, PublishAt: &past, AnnouncedAt: &past})

	if _, err := svc.Update(9, 1, map[string]interface{}{"publish_at": future}); err == nil {
		t.Error("una ya publicada no se puede volver a programar")
	}
}

func TestUpdateValidatesScheduleOrder(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	before := time.Now().Add(24 * time.Hour)
	_, svc := editFixture(&models.Tutorial{ID: 1, Title: "Lunes", PublishAt: &future})

	if _, err := svc.Update(9, 1, map[string]interface{}{"expires_at": before}); err == nil {
		t.Error("retirar antes de publicar debe rechazarse también al editar")
	}
}
