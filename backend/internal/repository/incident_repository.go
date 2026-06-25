package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IncidentRepository interface {
	List() ([]models.Incident, error)
	GetByID(id uint) (*models.Incident, error)
	Create(incident *models.Incident) error
	Update(incident *models.Incident, updates map[string]interface{}) error
	GetResponses(incidentID uint) ([]models.IncidentResponse, error)
	UpsertResponse(incidentID, userID uint, status, note string) error
	UpsertResponseIfPending(incidentID, userID uint, status string) error
}

type incidentRepository struct {
	db *gorm.DB
}

func NewIncidentRepository(db *gorm.DB) IncidentRepository {
	return &incidentRepository{db: db}
}

func (r *incidentRepository) List() ([]models.Incident, error) {
	var incidents []models.Incident
	err := r.db.Order("created_at DESC").Find(&incidents).Error
	return incidents, err
}

func (r *incidentRepository) GetByID(id uint) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.First(&incident, id).Error; err != nil {
		return nil, err
	}
	return &incident, nil
}

func (r *incidentRepository) Create(incident *models.Incident) error {
	return r.db.Create(incident).Error
}

func (r *incidentRepository) Update(incident *models.Incident, updates map[string]interface{}) error {
	return r.db.Model(incident).Updates(updates).Error
}

func (r *incidentRepository) GetResponses(incidentID uint) ([]models.IncidentResponse, error) {
	var responses []models.IncidentResponse
	err := r.db.Where("incident_id = ?", incidentID).Find(&responses).Error
	return responses, err
}

func (r *incidentRepository) UpsertResponse(incidentID, userID uint, status, note string) error {
	resp := models.IncidentResponse{
		IncidentID: incidentID,
		UserID:     userID,
		Status:     status,
		Note:       note,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "incident_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"status": status, "note": note, "updated_at": time.Now()}),
	}).Create(&resp).Error
}

func (r *incidentRepository) UpsertResponseIfPending(incidentID, userID uint, status string) error {
	resp := models.IncidentResponse{
		IncidentID: incidentID,
		UserID:     userID,
		Status:     status,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "incident_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			gorm.Expr("incident_responses.status = ?", models.IncidentResponsePendiente),
		}},
	}).Create(&resp).Error
}
