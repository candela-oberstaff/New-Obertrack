package repository

// EmailRepository manages email templates and campaigns.
// NOTE: No tenant_id filtering — all endpoints are behind RequireSuperadmin()
// middleware (see routes/platform_routes.go). Do NOT expose to non-superadmin users.

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type EmailRepository interface {
	CreateTemplate(template *models.EmailTemplate) error
	GetTemplates() ([]models.EmailTemplate, error)
	GetTemplateByID(id uint) (*models.EmailTemplate, error)
	UpdateTemplate(template *models.EmailTemplate) error
	DeleteTemplate(id uint) error

	CreateCampaign(campaign *models.EmailCampaign) error
	GetCampaigns() ([]models.EmailCampaign, error)
	GetCampaignByID(id uint) (*models.EmailCampaign, error)
	UpdateCampaign(campaign *models.EmailCampaign) error
	DeleteCampaign(id uint) error

	// RawQuery executes a raw SQL query scanning results into dest.
	RawQuery(query string, args []interface{}, dest interface{}) error

	CreateEvent(event *models.EmailEvent) error
	GetEventsByCampaign(campaignID uint) ([]models.EmailEvent, error)
	GetEventsByDay() ([]map[string]interface{}, error)
}

type emailRepository struct {
	db *gorm.DB
}

func NewEmailRepository(db *gorm.DB) EmailRepository {
	return &emailRepository{db: db}
}

func (r *emailRepository) CreateTemplate(template *models.EmailTemplate) error {
	return r.db.Create(template).Error
}

// Mismo criterio que GetCampaigns: la pestaña de plantillas comparte pantalla y
// tenía el mismo orden indefinido.
func (r *emailRepository) GetTemplates() ([]models.EmailTemplate, error) {
	var templates []models.EmailTemplate
	err := r.db.Order("created_at DESC, id DESC").Find(&templates).Error
	return templates, err
}

func (r *emailRepository) GetTemplateByID(id uint) (*models.EmailTemplate, error) {
	var template models.EmailTemplate
	err := r.db.First(&template, id).Error
	return &template, err
}

func (r *emailRepository) UpdateTemplate(template *models.EmailTemplate) error {
	return r.db.Model(template).Updates(map[string]interface{}{
		"title":      template.Title,
		"subject":    template.Subject,
		"content":    template.Content,
		"type":       template.Type,
		"is_active":  template.IsActive,
	}).Error
}

func (r *emailRepository) DeleteTemplate(id uint) error {
	return r.db.Delete(&models.EmailTemplate{}, id).Error
}

func (r *emailRepository) CreateCampaign(campaign *models.EmailCampaign) error {
	return r.db.Create(campaign).Error
}

// GetCampaigns lista las campañas de la más reciente a la más vieja.
//
// El ORDER BY no es cosmético: sin él Postgres devuelve las filas en el orden
// que le conviene —que arranca pareciéndose al de inserción, pero se desordena
// en cuanto se actualiza una fila, porque el UPDATE la reescribe en otro sitio—.
// El listado sale paginado, así que eso dejaba lo más nuevo en la última página
// y las fechas mezcladas dentro de la misma.
//
// El desempate por id importa: las campañas se crean en lote y comparten
// created_at al segundo, así que sin él volvería a ser arbitrario entre ellas.
func (r *emailRepository) GetCampaigns() ([]models.EmailCampaign, error) {
	var campaigns []models.EmailCampaign
	err := r.db.Preload("Template").
		Order("created_at DESC, id DESC").
		Find(&campaigns).Error
	return campaigns, err
}

func (r *emailRepository) GetCampaignByID(id uint) (*models.EmailCampaign, error) {
	var campaign models.EmailCampaign
	err := r.db.Preload("Template").First(&campaign, id).Error
	return &campaign, err
}

func (r *emailRepository) UpdateCampaign(campaign *models.EmailCampaign) error {
	updates := map[string]interface{}{
		"title":          campaign.Title,
		"subject":        campaign.Subject,
		"status":         campaign.Status,
		"scheduled_at":   campaign.ScheduledAt,
		"sent_at":        campaign.SentAt,
		"recipients":     campaign.Recipients,
		"recipient_list": campaign.RecipientList,
	}

	// Only update template_id if it's provided (> 0)
	if campaign.TemplateID > 0 {
		updates["template_id"] = campaign.TemplateID
	}

	return r.db.Model(campaign).Updates(updates).Error
}

func (r *emailRepository) DeleteCampaign(id uint) error {
	return r.db.Delete(&models.EmailCampaign{}, id).Error
}

func (r *emailRepository) RawQuery(query string, args []interface{}, dest interface{}) error {
	return r.db.Raw(query, args...).Scan(dest).Error
}

func (r *emailRepository) CreateEvent(event *models.EmailEvent) error {
	return r.db.Create(event).Error
}

// El detalle de una campaña es un registro de actividad: lo último arriba. Se
// ordena por timestamp (cuándo ocurrió en Brevo) y no por created_at (cuándo nos
// llegó el webhook), que pueden diferir bastante si hubo reintentos.
func (r *emailRepository) GetEventsByCampaign(campaignID uint) ([]models.EmailEvent, error) {
	var events []models.EmailEvent
	err := r.db.Where("campaign_id = ?", campaignID).
		Order("timestamp DESC, id DESC").
		Find(&events).Error
	return events, err
}

func (r *emailRepository) GetEventsByDay() ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	// This query aggregates events by day for the last 7 days
	err := r.db.Model(&models.EmailEvent{}).
		Select("DATE(timestamp) as date, event, COUNT(*) as count").
		Where("timestamp >= ?", time.Now().AddDate(0, 0, -7)).
		Group("DATE(timestamp), event").
		Order("DATE(timestamp) ASC").
		Scan(&results).Error
	return results, err
}
