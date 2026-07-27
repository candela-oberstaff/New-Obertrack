package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// InductionRepository accede a la configuración global de inducción, a las
// invitaciones personales y a los intentos registrados.
type InductionRepository interface {
	// GetConfig devuelve la fila única de configuración (id=1). Si no existe,
	// devuelve una configuración apagada en lugar de error: la inducción
	// simplemente no aplica.
	GetConfig() (*models.InductionConfig, error)
	SaveConfig(cfg *models.InductionConfig) error

	CreateInvite(invite *models.InductionInvite) error
	UpdateInvite(invite *models.InductionInvite, updates map[string]interface{}) error
	GetInviteByToken(token string) (*models.InductionInvite, error)
	GetInviteByUser(userID uint) (*models.InductionInvite, error)
	DeleteInviteByUser(userID uint) error

	CreateAttempt(attempt *models.InductionAttempt) error
	ListAttempts(inviteID uint) ([]models.InductionAttempt, error)

	// GetSurveyWithQuestions carga el cuestionario con sus preguntas ordenadas.
	// Vive aquí (y no en el repo de encuestas) para que el servicio de inducción
	// no dependa del módulo completo de encuestas.
	GetSurveyWithQuestions(surveyID uint) (*models.Survey, error)
	GetTutorial(tutorialID uint) (*models.Tutorial, error)
}

type inductionRepository struct {
	db *gorm.DB
}

func NewInductionRepository(db *gorm.DB) InductionRepository {
	return &inductionRepository{db: db}
}

func (r *inductionRepository) GetConfig() (*models.InductionConfig, error) {
	var cfg models.InductionConfig
	if err := r.db.First(&cfg, 1).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Sin configuración = inducción apagada (no es un error).
			return &models.InductionConfig{ID: 1, PassingScore: 70, MaxAttempts: 3, InviteTTLDays: 30}, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *inductionRepository) SaveConfig(cfg *models.InductionConfig) error {
	cfg.ID = 1
	return r.db.Save(cfg).Error
}

func (r *inductionRepository) CreateInvite(invite *models.InductionInvite) error {
	return r.db.Create(invite).Error
}

func (r *inductionRepository) UpdateInvite(invite *models.InductionInvite, updates map[string]interface{}) error {
	return r.db.Model(invite).Updates(updates).Error
}

func (r *inductionRepository) GetInviteByToken(token string) (*models.InductionInvite, error) {
	var invite models.InductionInvite
	if err := r.db.Where("token = ?", token).First(&invite).Error; err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *inductionRepository) GetInviteByUser(userID uint) (*models.InductionInvite, error) {
	var invite models.InductionInvite
	if err := r.db.Where("user_id = ?", userID).First(&invite).Error; err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *inductionRepository) DeleteInviteByUser(userID uint) error {
	return r.db.Unscoped().Where("user_id = ?", userID).Delete(&models.InductionInvite{}).Error
}

func (r *inductionRepository) CreateAttempt(attempt *models.InductionAttempt) error {
	return r.db.Create(attempt).Error
}

func (r *inductionRepository) ListAttempts(inviteID uint) ([]models.InductionAttempt, error) {
	var attempts []models.InductionAttempt
	err := r.db.Where("invite_id = ?", inviteID).Order("created_at ASC").Find(&attempts).Error
	return attempts, err
}

func (r *inductionRepository) GetSurveyWithQuestions(surveyID uint) (*models.Survey, error) {
	var survey models.Survey
	err := r.db.
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_index ASC, id ASC")
		}).
		First(&survey, surveyID).Error
	if err != nil {
		return nil, err
	}
	return &survey, nil
}

func (r *inductionRepository) GetTutorial(tutorialID uint) (*models.Tutorial, error) {
	var tutorial models.Tutorial
	if err := r.db.First(&tutorial, tutorialID).Error; err != nil {
		return nil, err
	}
	return &tutorial, nil
}
