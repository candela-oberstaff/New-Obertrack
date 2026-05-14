package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type EmailRepository interface {
	CreateTemplate(template *models.EmailTemplate) error
	GetTemplates() ([]models.EmailTemplate, error)
	GetTemplateByID(id uint) (*models.EmailTemplate, error)
	UpdateTemplate(template *models.EmailTemplate) error

	CreateCampaign(campaign *models.EmailCampaign) error
	GetCampaigns() ([]models.EmailCampaign, error)
	GetCampaignByID(id uint) (*models.EmailCampaign, error)
	UpdateCampaign(campaign *models.EmailCampaign) error

	// RawQuery executes a raw SQL query scanning results into dest.
	RawQuery(query string, args []interface{}, dest interface{}) error
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

func (r *emailRepository) GetTemplates() ([]models.EmailTemplate, error) {
	var templates []models.EmailTemplate
	err := r.db.Find(&templates).Error
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

func (r *emailRepository) CreateCampaign(campaign *models.EmailCampaign) error {
	return r.db.Create(campaign).Error
}

func (r *emailRepository) GetCampaigns() ([]models.EmailCampaign, error) {
	var campaigns []models.EmailCampaign
	err := r.db.Preload("Template").Find(&campaigns).Error
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

func (r *emailRepository) RawQuery(query string, args []interface{}, dest interface{}) error {
	return r.db.Raw(query, args...).Scan(dest).Error
}
