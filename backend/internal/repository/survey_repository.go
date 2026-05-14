package repository

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
	// GORM's Full Save Updates for nested relations can be tricky, 
	// usually it's better to update the main fields and clear/recreate questions if they changed
	// For now, we update the top-level fields
	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Updates(survey).Error
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
