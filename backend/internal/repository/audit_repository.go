package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// AuditRepository persists and queries the app-wide audit log.
type AuditRepository interface {
	Create(entry *models.AuditLog) error
	FindAll(filters map[string]interface{}, offset, limit int) ([]models.AuditLog, int64, error)
}

type auditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Create(entry *models.AuditLog) error {
	return r.db.Create(entry).Error
}

func (r *auditRepository) FindAll(filters map[string]interface{}, offset, limit int) ([]models.AuditLog, int64, error) {
	query := r.db.Model(&models.AuditLog{})

	if v, ok := filters["actor_id"].(uint); ok && v > 0 {
		query = query.Where("actor_id = ?", v)
	}
	if v, ok := filters["email"].(string); ok && v != "" {
		query = query.Where("actor_email ILIKE ?", "%"+v+"%")
	}
	if v, ok := filters["kind"].(string); ok && v != "" {
		query = query.Where("kind = ?", v)
	}
	if v, ok := filters["entity_type"].(string); ok && v != "" {
		query = query.Where("entity_type = ?", v)
	}
	if v, ok := filters["entity_id"].(string); ok && v != "" {
		query = query.Where("entity_id = ?", v)
	}
	if v, ok := filters["module"].(string); ok && v != "" {
		query = query.Where("module = ?", v)
	}
	if v, ok := filters["action"].(string); ok && v != "" {
		query = query.Where("action ILIKE ?", "%"+v+"%")
	}
	if v, ok := filters["success"].(bool); ok {
		query = query.Where("success = ?", v)
	}
	if v, ok := filters["start_date"].(time.Time); ok {
		query = query.Where("created_at >= ?", v)
	}
	if v, ok := filters["end_date"].(time.Time); ok {
		query = query.Where("created_at <= ?", v)
	}
	if v, ok := filters["q"].(string); ok && v != "" {
		like := "%" + v + "%"
		query = query.Where("actor_email ILIKE ? OR action ILIKE ? OR path ILIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []models.AuditLog
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}
