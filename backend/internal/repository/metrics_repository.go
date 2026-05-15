package repository

import (
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
	var campaigns []models.EmailCampaign
	// Filter campaigns sent in the last X days
	if err := r.db.Where("sent_at >= NOW() - (? * INTERVAL '1 day')", days).Find(&campaigns).Error; err != nil {
		return nil, err
	}

	var totalOpened int64
	var totalClicked int64
	var totalBounced int64
	
	r.db.Model(&models.EmailEvent{}).Where("event = ? AND timestamp >= NOW() - (? * INTERVAL '1 day')", "opened", days).Count(&totalOpened)
	r.db.Model(&models.EmailEvent{}).Where("event = ? AND timestamp >= NOW() - (? * INTERVAL '1 day')", "click", days).Count(&totalClicked)
	r.db.Model(&models.EmailEvent{}).Where("event LIKE ? AND timestamp >= NOW() - (? * INTERVAL '1 day')", "%bounce%", days).Count(&totalBounced)

	totalSent := 0
	for _, c := range campaigns {
		totalSent += c.Recipients
	}

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
		"campaign_count": len(campaigns),
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

	return map[string]interface{}{
		"total_surveys": len(surveys),
		"total_responses": totalResponses,
		"avg_satisfaction": avgSat,
	}, nil
}
