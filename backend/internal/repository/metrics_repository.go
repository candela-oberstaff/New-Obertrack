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

	var totalOpened int64
	var totalClicked int64
	var totalBounced int64

	r.db.Model(&models.EmailEvent{}).Where("event = ? AND timestamp >= NOW() - (? * INTERVAL '1 day')", "opened", days).Count(&totalOpened)
	r.db.Model(&models.EmailEvent{}).Where("event = ? AND timestamp >= NOW() - (? * INTERVAL '1 day')", "click", days).Count(&totalClicked)
	r.db.Model(&models.EmailEvent{}).Where("event LIKE ? AND timestamp >= NOW() - (? * INTERVAL '1 day')", "%bounce%", days).Count(&totalBounced)

	openRate := 0.0
	clickRate := 0.0
	if totalSent > 0 {
		openRate = (float64(totalOpened) / float64(totalSent)) * 100
		clickRate = (float64(totalClicked) / float64(totalSent)) * 100
	}

	// Evolution data for the requested days
	var evolution []map[string]interface{}
	r.db.Raw(`
		SELECT DATE(timestamp) as date, event, COUNT(*) as count 
		FROM email_events 
		WHERE timestamp >= NOW() - (? * INTERVAL '1 day')
		GROUP BY DATE(timestamp), event
		ORDER BY DATE(timestamp) ASC
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
