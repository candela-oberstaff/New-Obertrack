package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserIncident es una incidencia que alcanzó a un profesional, con la respuesta
// que se registró por él. Es la incidencia vista desde su ficha: lo que importa
// ahí no es el recuento global sino si a esta persona se la pudo contactar.
type UserIncident struct {
	ID             uint       `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Kind           string     `json:"kind"`
	Country        string     `json:"country"`
	State          string     `json:"state"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
	ResponseStatus string     `json:"response_status"`
	ResponseNote   string     `json:"response_note"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
}

type IncidentRepository interface {
	List() ([]models.Incident, error)
	// ListForUser son las incidencias en las que ESTE profesional fue incluido.
	// Se cruza por incident_responses y no por país/estado: la fila de respuesta
	// es la prueba de que se le incluyó en el aviso, mientras que su ubicación
	// actual pudo cambiar después de la incidencia.
	ListForUser(userID uint) ([]UserIncident, error)
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

func (r *incidentRepository) ListForUser(userID uint) ([]UserIncident, error) {
	var incidents []UserIncident
	err := r.db.Raw(`
		SELECT
			i.id, i.title, i.description, i.kind, i.country, i.state, i.status,
			i.created_at, i.closed_at,
			COALESCE(ir.status, '') as response_status,
			COALESCE(ir.note, '') as response_note,
			ir.updated_at as responded_at
		FROM incidents i
		JOIN incident_responses ir ON ir.incident_id = i.id AND ir.user_id = ?
		WHERE i.deleted_at IS NULL
		ORDER BY i.created_at DESC
		LIMIT 100
	`, userID).Scan(&incidents).Error
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
