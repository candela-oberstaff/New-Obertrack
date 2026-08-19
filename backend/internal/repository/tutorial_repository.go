package repository

// TutorialRepository manages onboarding tutorials.
// NOTE: No tenant_id filtering — tutorials are platform-wide content managed only
// by superadmins (writes are behind RequireSuperadmin(), see routes/platform_routes.go).
// Read visibility is segmented by audience: 'all', 'empleador' or 'profesional'.

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TutorialRepository interface {
	FindAll(onlyActive bool, audience string) ([]models.Tutorial, error)
	GetByID(id uint) (*models.Tutorial, error)
	Create(tutorial *models.Tutorial) error
	Update(tutorial *models.Tutorial, updates map[string]interface{}) error
	Delete(id uint) error
	Reorder(ids []uint) error
	RecordView(tutorialID, userID uint, source string, acknowledged bool) error
	// RecordClick deja constancia de que alguien pulso el boton de accion.
	RecordClick(tutorialID, userID uint) error
	// ClickersFor son los usuarios que pulsaron el boton de una novedad.
	ClickersFor(tutorialID uint) (map[uint]bool, error)
	// ListDuePublications son las novedades programadas cuya hora ya llego.
	ListDuePublications(now time.Time) ([]models.Tutorial, error)
	// ListDueExpirations son las novedades vivas que ya caducaron.
	ListDueExpirations(now time.Time) ([]models.Tutorial, error)
	// SetFields actualiza columnas sueltas por ID, sin pasar por la validacion
	// del formulario. Lo usan el reloj de publicacion y el recordatorio.
	SetFields(id uint, fields map[string]interface{}) error
	GetUserViewedIDs(userID uint) ([]uint, error)
	// FindPendingAnnouncements lista las novedades ANUNCIADAS que este usuario
	// todavía no ha visto: es lo que emerge al iniciar sesión.
	FindPendingAnnouncements(userID uint, audience string, now time.Time, limit int) ([]models.Tutorial, error)
	// MarkAnnounced sella el momento del anuncio de una novedad.
	MarkAnnounced(id uint, at time.Time) error
	// ViewsFor devuelve las vistas crudas de una novedad. Las métricas se
	// arman en el servicio, que es quien conoce el público objetivo.
	ViewsFor(tutorialID uint) ([]models.TutorialView, error)
	// UsersInGroups son los IDs de usuario que pertenecen a alguno de los
	// grupos de audiencia dados.
	UsersInGroups(groupIDs []uint) (map[uint]bool, error)
	// ListAudienceGroups lista los grupos de audiencia (los mismos de Correos)
	// con cuantos miembros tiene cada uno, para poder elegirlos como publico.
	ListAudienceGroups() ([]models.TutorialAudienceOption, error)
}

type tutorialRepository struct {
	db *gorm.DB
}

func NewTutorialRepository(db *gorm.DB) TutorialRepository {
	return &tutorialRepository{db: db}
}

// FindAll lists tutorials. audience == "" means no audience filter (platform staff);
// otherwise only tutorials targeted at that audience or at everyone are returned.
func (r *tutorialRepository) FindAll(onlyActive bool, audience string) ([]models.Tutorial, error) {
	var tutorials []models.Tutorial
	query := r.db.Model(&models.Tutorial{}).Preload("Creator")
	if onlyActive {
		query = query.Where("is_active = ?", true)
	}
	if audience != "" {
		query = query.Where("audience IN ?", []string{models.TutorialAudienceAll, audience})
	}
	if err := query.Order("order_index ASC, created_at DESC").Find(&tutorials).Error; err != nil {
		return nil, err
	}
	return tutorials, nil
}

func (r *tutorialRepository) GetByID(id uint) (*models.Tutorial, error) {
	var tutorial models.Tutorial
	if err := r.db.Preload("Creator").First(&tutorial, id).Error; err != nil {
		return nil, err
	}
	return &tutorial, nil
}

func (r *tutorialRepository) Create(tutorial *models.Tutorial) error {
	return r.db.Create(tutorial).Error
}

func (r *tutorialRepository) Update(tutorial *models.Tutorial, updates map[string]interface{}) error {
	return r.db.Model(tutorial).Updates(updates).Error
}

func (r *tutorialRepository) Delete(id uint) error {
	return r.db.Delete(&models.Tutorial{}, id).Error
}

func (r *tutorialRepository) Reorder(ids []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&models.Tutorial{}).Where("id = ?", id).Update("order_index", index).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordView deja constancia de que alguien vio la novedad. Si ya había visto
// antes solo se refresca updated_at: el origen NO se pisa, porque interesa
// saber qué la puso delante la primera vez.
func (r *tutorialRepository) RecordView(tutorialID, userID uint, source string, acknowledged bool) error {
	now := time.Now()
	view := models.TutorialView{
		TutorialID: tutorialID,
		UserID:     userID,
		Source:     source,
		ViewedAt:   now,
		UpdatedAt:  now,
	}
	// El acuse se sella al confirmarse y ya no se toca: es evidencia, no un
	// estado que vaya y venga.
	updated := []string{"updated_at"}
	if acknowledged {
		view.AcknowledgedAt = &now
		updated = append(updated, "acknowledged_at")
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tutorial_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns(updated),
	}).Create(&view).Error
}

// RecordClick cuenta a la persona una sola vez: la pregunta es a cuanta gente
// movio la novedad, no cuantas veces pulso la misma persona.
func (r *tutorialRepository) RecordClick(tutorialID, userID uint) error {
	now := time.Now()
	click := models.TutorialClick{
		TutorialID: tutorialID,
		UserID:     userID,
		ClickedAt:  now,
		UpdatedAt:  now,
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tutorial_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
	}).Create(&click).Error
}

func (r *tutorialRepository) ClickersFor(tutorialID uint) (map[uint]bool, error) {
	var ids []uint
	if err := r.db.Model(&models.TutorialClick{}).
		Where("tutorial_id = ?", tutorialID).
		Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	clickers := make(map[uint]bool, len(ids))
	for _, id := range ids {
		clickers[id] = true
	}
	return clickers, nil
}

// ListDuePublications busca borradores con hora cumplida. Se exige
// announced_at IS NULL para que el reloj no vuelva a anunciar algo que ya se
// anuncio a mano.
func (r *tutorialRepository) ListDuePublications(now time.Time) ([]models.Tutorial, error) {
	var tutorials []models.Tutorial
	if err := r.db.Where("is_active = ? AND publish_at IS NOT NULL AND publish_at <= ? AND announced_at IS NULL", false, now).
		Find(&tutorials).Error; err != nil {
		return nil, err
	}
	return tutorials, nil
}

func (r *tutorialRepository) ListDueExpirations(now time.Time) ([]models.Tutorial, error) {
	var tutorials []models.Tutorial
	if err := r.db.Where("is_active = ? AND expires_at IS NOT NULL AND expires_at <= ?", true, now).
		Find(&tutorials).Error; err != nil {
		return nil, err
	}
	return tutorials, nil
}

func (r *tutorialRepository) SetFields(id uint, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&models.Tutorial{}).Where("id = ?", id).Updates(fields).Error
}

func (r *tutorialRepository) ViewsFor(tutorialID uint) ([]models.TutorialView, error) {
	var views []models.TutorialView
	if err := r.db.Where("tutorial_id = ?", tutorialID).Find(&views).Error; err != nil {
		return nil, err
	}
	return views, nil
}

func (r *tutorialRepository) UsersInGroups(groupIDs []uint) (map[uint]bool, error) {
	members := map[uint]bool{}
	if len(groupIDs) == 0 {
		return members, nil
	}
	var ids []uint
	if err := r.db.Table("audience_group_members").
		Where("audience_group_id IN ?", groupIDs).
		Distinct().
		Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		members[id] = true
	}
	return members, nil
}

// FindPendingAnnouncements devuelve las novedades activas cuya ventana de
// anuncio sigue abierta y que el usuario aun no ha visto, de la mas reciente a
// la mas vieja. La ventana la fija cada novedad (announce_days) y evita que
// una que nadie abrio persiga a la gente para siempre; el registro de vistas
// (tutorial_views) es lo que la apaga antes de tiempo: cerrar el aviso
// emergente cuenta como verla.
//
// El publico objetivo NO se filtra aqui (vive en JSON): lo aplica el servicio
// sobre estas pocas filas, con la misma regla que uso el reparto.
func (r *tutorialRepository) FindPendingAnnouncements(userID uint, audience string, now time.Time, limit int) ([]models.Tutorial, error) {
	var tutorials []models.Tutorial
	query := r.db.Model(&models.Tutorial{}).
		Where("is_active = ?", true).
		Where("announced_at IS NOT NULL AND announce_days > 0").
		Where("announced_at + (announce_days * INTERVAL '1 day') > ?", now).
		Where("NOT EXISTS (SELECT 1 FROM tutorial_views v WHERE v.tutorial_id = tutorials.id AND v.user_id = ?)", userID)
	if audience != "" {
		query = query.Where("audience IN ?", []string{models.TutorialAudienceAll, audience})
	}
	if err := query.Order("announced_at DESC").Limit(limit).Find(&tutorials).Error; err != nil {
		return nil, err
	}
	return tutorials, nil
}

func (r *tutorialRepository) MarkAnnounced(id uint, at time.Time) error {
	return r.db.Model(&models.Tutorial{}).Where("id = ?", id).Update("announced_at", at).Error
}

func (r *tutorialRepository) ListAudienceGroups() ([]models.TutorialAudienceOption, error) {
	var groups []models.TutorialAudienceOption
	if err := r.db.Table("audience_groups AS g").
		Select("g.id AS id, g.name AS name, COUNT(m.user_id) AS count").
		Joins("LEFT JOIN audience_group_members m ON m.audience_group_id = g.id").
		Where("g.deleted_at IS NULL").
		Group("g.id, g.name").
		Order("g.name ASC").
		Scan(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *tutorialRepository) GetUserViewedIDs(userID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&models.TutorialView{}).
		Where("user_id = ?", userID).
		Pluck("tutorial_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
