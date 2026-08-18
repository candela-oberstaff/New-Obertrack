package repository

import (
	"encoding/json"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type MetricsRepository interface {
	GetEmailMetrics(days int) (map[string]interface{}, error)
	GetSurveyMetrics(days int) (map[string]interface{}, error)
}

type metricsRepository struct {
	db *gorm.DB
}

func NewMetricsRepository(db *gorm.DB) MetricsRepository {
	return &metricsRepository{db: db}
}

func (r *metricsRepository) GetEmailMetrics(days int) (map[string]interface{}, error) {
	// Total emails sent = sum of all recipients across every "sent" campaign (all time).
	// We count individual dispatches, NOT unique recipients, so sending to the same
	// person in two different campaigns counts as 2.
	var totalSent int64
	if err := r.db.Raw(`
		SELECT COALESCE(SUM(recipients), 0)
		FROM email_campaigns
		WHERE status = 'sent'
		  AND deleted_at IS NULL
		  AND COALESCE(sent_at, created_at) >= NOW() - (? * INTERVAL '1 day')
	`, days).Scan(&totalSent).Error; err != nil {
		return nil, err
	}

	// Count campaigns sent in the requested period (for the campaign_count card)
	var campaignCount int64
	r.db.Raw(`
		SELECT COUNT(*)
		FROM email_campaigns
		WHERE status = 'sent'
		  AND deleted_at IS NULL
		  AND COALESCE(sent_at, created_at) >= NOW() - (? * INTERVAL '1 day')
	`, days).Scan(&campaignCount)

	// Aperturas, clics y rebotes DE ESAS CAMPAÑAS.
	//
	// Antes se contaban los eventos de todos los correos del sistema —avisos de
	// tareas, recordatorios de horas, tickets, inducciones— contra un
	// denominador que solo suma destinatarios de campañas. Mezclar los dos
	// universos daba tasas sin sentido: el sistema manda muchos más avisos que
	// campañas, así que la apertura se iba muy por encima del 100%. El error
	// estuvo escondido mientras la tabla de eventos estuvo vacía; aparece en
	// cuanto el webhook empieza a entregar.
	//
	// Se cuentan PERSONAS por campaña y no eventos —quien abre tres veces genera
	// tres eventos, y el webhook se reintenta— con el mismo criterio que el
	// panel de campañas, para que las dos pantallas no se contradigan.
	var agg struct {
		Opened  int64
		Clicked int64
		Bounced int64
	}
	r.db.Raw(`
		SELECT
		  COUNT(DISTINCT (e.campaign_id, LOWER(e.email)))
		    FILTER (WHERE e.event IN ('opened','unique_opened','proxy_open','unique_proxy_open')) AS opened,
		  COUNT(DISTINCT (e.campaign_id, LOWER(e.email)))
		    FILTER (WHERE e.event = 'click') AS clicked,
		  COUNT(DISTINCT (e.campaign_id, LOWER(e.email)))
		    FILTER (WHERE e.event IN ('hard_bounce','soft_bounce')) AS bounced
		FROM email_events e
		JOIN email_campaigns c ON c.id = e.campaign_id
		WHERE c.status = 'sent'
		  AND c.deleted_at IS NULL
		  AND COALESCE(c.sent_at, c.created_at) >= NOW() - (? * INTERVAL '1 day')
	`, days).Scan(&agg)

	totalOpened, totalClicked, totalBounced := agg.Opened, agg.Clicked, agg.Bounced

	openRate := 0.0
	clickRate := 0.0
	if totalSent > 0 {
		openRate = (float64(totalOpened) / float64(totalSent)) * 100
		clickRate = (float64(totalClicked) / float64(totalSent)) * 100
	}

	// Evolución diaria, también acotada a campañas: un gráfico que incluyera los
	// correos de aviso al lado de unas tasas que no los incluyen contaría dos
	// historias distintas en la misma pantalla.
	var evolution []map[string]interface{}
	r.db.Raw(`
		SELECT DATE(e.timestamp) as date, e.event, COUNT(*) as count
		FROM email_events e
		JOIN email_campaigns c ON c.id = e.campaign_id
		WHERE c.status = 'sent'
		  AND c.deleted_at IS NULL
		  AND e.timestamp >= NOW() - (? * INTERVAL '1 day')
		GROUP BY DATE(e.timestamp), e.event
		ORDER BY DATE(e.timestamp) ASC
	`, days).Scan(&evolution)

	return map[string]interface{}{
		"total_sent":     totalSent,
		"open_rate":      openRate,
		"click_rate":     clickRate,
		"total_opened":   totalOpened,
		"total_clicked":  totalClicked,
		"total_bounced":  totalBounced,
		"campaign_count": campaignCount,
		"evolution":      evolution,
	}, nil
}


func (r *metricsRepository) GetSurveyMetrics(days int) (map[string]interface{}, error) {
	var surveys []models.Survey
	// Filter surveys created in the last X days
	if err := r.db.Preload("Responses.Answers").Where("created_at >= NOW() - (? * INTERVAL '1 day')", days).Find(&surveys).Error; err != nil {
		return nil, err
	}

	totalResponses := 0
	var totalSatisfaction float64
	ratingCount := 0

	for _, s := range surveys {
		totalResponses += len(s.Responses)
		for _, res := range s.Responses {
			for _, ans := range res.Answers {
				if ans.NumberValue > 0 {
					totalSatisfaction += float64(ans.NumberValue)
					ratingCount++
				}
			}
		}
	}

	avgSat := 0.0
	if ratingCount > 0 {
		avgSat = totalSatisfaction / float64(ratingCount)
	}

	totalSent := 0
	for _, s := range surveys {
		if s.Status == models.SurveyStatusActive || s.Status == models.SurveyStatusClosed {
			var ids []int
			if s.RecipientList != "" {
				if err := json.Unmarshal([]byte(s.RecipientList), &ids); err == nil {
					totalSent += len(ids)
				}
			}
		}
	}

	return map[string]interface{}{
		"total_sent":       totalSent,
		"total_responses":  totalResponses,
		"avg_satisfaction": avgSat,
	}, nil
}
