package repository

// EmailRepository manages email templates and campaigns.
// NOTE: No tenant_id filtering — all endpoints are behind RequireSuperadmin()
// middleware (see routes/platform_routes.go). Do NOT expose to non-superadmin users.

import (
	"math"
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
	if err != nil {
		return campaigns, err
	}
	r.fillEngagementRates(campaigns)
	return campaigns, nil
}

func (r *emailRepository) GetCampaignByID(id uint) (*models.EmailCampaign, error) {
	var campaign models.EmailCampaign
	err := r.db.Preload("Template").First(&campaign, id).Error
	if err != nil {
		return &campaign, err
	}
	one := []models.EmailCampaign{campaign}
	r.fillEngagementRates(one)
	return &one[0], nil
}

// fillEngagementRates calcula open_rate y click_rate desde los eventos, en vez
// de leerlos de las columnas del mismo nombre.
//
// Esas columnas existen desde la primera versión pero NUNCA se escribieron: no
// hay un solo UPDATE que las toque, así que la tarjeta de cada campaña mostraba
// 0% de forma permanente. Calcularlas al leer las mantiene siempre al día —los
// eventos siguen llegando durante días después del envío— y evita tener que
// recalcular una fila por cada webhook entrante.
//
// Se cuentan PERSONAS distintas, no eventos: quien abre el correo tres veces
// genera tres 'opened', y Brevo además reintenta el webhook si no respondimos a
// tiempo. Contando eventos, los porcentajes pasaban del 100%. Es el mismo
// criterio que usa el panel de detalle, para que no se contradigan.
//
// Los proxy_open cuentan como apertura: Brevo los reporta aparte cuando la
// apertura viene de un proxy de privacidad (Apple Mail y similares, que
// precargan las imágenes). Omitirlos hundía la tasa sin que se notara.
func (r *emailRepository) fillEngagementRates(campaigns []models.EmailCampaign) {
	if len(campaigns) == 0 {
		return
	}

	ids := make([]uint, 0, len(campaigns))
	for _, c := range campaigns {
		if c.Recipients > 0 {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return
	}

	var rows []struct {
		CampaignID uint
		Opens      int
		Clicks     int
	}

	err := r.db.Model(&models.EmailEvent{}).
		Select(`campaign_id,
			COUNT(DISTINCT LOWER(email)) FILTER (WHERE event IN ('opened','unique_opened','proxy_open','unique_proxy_open')) AS opens,
			COUNT(DISTINCT LOWER(email)) FILTER (WHERE event = 'click') AS clicks`).
		Where("campaign_id IN ?", ids).
		Group("campaign_id").
		Scan(&rows).Error
	if err != nil {
		// Las tasas son informativas: si la agregación falla, el listado sale
		// igual (en 0%) en lugar de romperse entero.
		return
	}

	byID := make(map[uint]struct{ opens, clicks int }, len(rows))
	for _, row := range rows {
		byID[row.CampaignID] = struct{ opens, clicks int }{row.Opens, row.Clicks}
	}

	for i := range campaigns {
		agg, ok := byID[campaigns[i].ID]
		if !ok {
			continue
		}
		total := float64(campaigns[i].Recipients)
		campaigns[i].OpenRate = math.Round(float64(agg.opens)/total*1000) / 10
		campaigns[i].ClickRate = math.Round(float64(agg.clicks)/total*1000) / 10
	}
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
