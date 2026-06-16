package repository

// SurveyRepository manages surveys, questions and responses.
// NOTE: No tenant_id filtering — CRUD endpoints are behind RequireSuperadmin()
// middleware. The QuickResponse endpoint (public) is validated separately in the handler.

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type SurveyRepository interface {
	CreateSurvey(survey *models.Survey) error
	GetSurveys() ([]models.Survey, error)
	GetSurveyByID(id uint) (*models.Survey, error)
	UpdateSurvey(survey *models.Survey) error
	DeleteSurvey(id uint) error

	CreateResponse(response *models.SurveyResponse) error
	GetSurveyResponses(surveyID uint) ([]models.SurveyResponse, error)
}

type surveyRepository struct {
	db *gorm.DB
}

func NewSurveyRepository(db *gorm.DB) SurveyRepository {
	return &surveyRepository{db: db}
}

func (r *surveyRepository) CreateSurvey(survey *models.Survey) error {
	return r.db.Create(survey).Error
}

func (r *surveyRepository) GetSurveys() ([]models.Survey, error) {
	var surveys []models.Survey
	err := r.db.Preload("Questions").Preload("Responses").Find(&surveys).Error
	return surveys, err
}

func (r *surveyRepository) GetSurveyByID(id uint) (*models.Survey, error) {
	var survey models.Survey
	err := r.db.Preload("Questions").Preload("Responses.User").Preload("Responses.Answers").First(&survey, id).Error
	return &survey, err
}

func (r *surveyRepository) UpdateSurvey(survey *models.Survey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing questions first to prevent duplicates/orphans
		if err := tx.Where("survey_id = ?", survey.ID).Delete(&models.SurveyQuestion{}).Error; err != nil {
			return err
		}
		
		// Reset IDs of questions to let GORM create them as new records
		for i := range survey.Questions {
			survey.Questions[i].ID = 0
			survey.Questions[i].SurveyID = survey.ID
		}

		// Save the top-level survey and its questions
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(survey).Error
	})
}

func (r *surveyRepository) DeleteSurvey(id uint) error {
	return r.db.Delete(&models.Survey{}, id).Error
}

func (r *surveyRepository) CreateResponse(response *models.SurveyResponse) error {
	return r.db.Create(response).Error
}

func (r *surveyRepository) GetSurveyResponses(surveyID uint) ([]models.SurveyResponse, error) {
	var responses []models.SurveyResponse
	err := r.db.Preload("Answers").Preload("User").Where("survey_id = ?", surveyID).Find(&responses).Error
	return responses, err
}
