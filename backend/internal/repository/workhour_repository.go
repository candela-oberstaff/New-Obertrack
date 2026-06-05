package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type WorkHourRepository interface {
	Create(workHour *models.WorkHour) error
	FindByID(id uint) (*models.WorkHour, error)
	FindManyByIDs(ids []uint) ([]models.WorkHour, error)
	FindManyByIDsAndTenant(ids []uint, tenantID uint) ([]models.WorkHour, error)
	FindByUserAndDate(userID uint, date time.Time) (*models.WorkHour, error)
	Update(workHour *models.WorkHour) error
	ApproveMultiple(ids []uint, approvedBy uint, approvedAt time.Time) error
	FindAll(filters map[string]interface{}, offset, limit int) ([]models.WorkHour, int64, error)
	GetSummary(filters map[string]interface{}) (map[string]float64, error)
	CountActiveToday() (int64, error)
	GetTotalHoursMonth() (float64, error)
}

type workHourRepository struct {
	db *gorm.DB
}

func NewWorkHourRepository(db *gorm.DB) WorkHourRepository {
	return &workHourRepository{db: db}
}

func (r *workHourRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *workHourRepository) Create(workHour *models.WorkHour) error {
	return r.db.Create(workHour).Error
}

func (r *workHourRepository) FindByID(id uint) (*models.WorkHour, error) {
	var workHour models.WorkHour
	err := r.db.First(&workHour, id).Error
	return &workHour, err
}

func (r *workHourRepository) FindManyByIDs(ids []uint) ([]models.WorkHour, error) {
	var workHours []models.WorkHour
	err := r.db.Preload("User").Where("id IN ?", ids).Find(&workHours).Error
	return workHours, err
}

func (r *workHourRepository) FindManyByIDsAndTenant(ids []uint, tenantID uint) ([]models.WorkHour, error) {
	var workHours []models.WorkHour
	q := r.db.Preload("User").Where("id IN ?", ids)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Find(&workHours).Error
	return workHours, err
}

func (r *workHourRepository) FindByUserAndDate(userID uint, date time.Time) (*models.WorkHour, error) {
	var workHour models.WorkHour
	err := r.db.Where("user_id = ? AND work_date = ?", userID, date).First(&workHour).Error
	return &workHour, err
}

func (r *workHourRepository) Update(workHour *models.WorkHour) error {
	return r.db.Save(workHour).Error
}

func (r *workHourRepository) ApproveMultiple(ids []uint, approvedBy uint, approvedAt time.Time) error {
	return r.db.Model(&models.WorkHour{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"approved":    true,
			"approved_by": approvedBy,
			"approved_at": approvedAt,
		}).Error
}

func (r *workHourRepository) FindAll(filters map[string]interface{}, offset, limit int) ([]models.WorkHour, int64, error) {
	query := r.db.Model(&models.WorkHour{})

	if tenantID, ok := filters["tenant_id"].(uint); ok {
		query = query.Where("work_hours.tenant_id = ?", tenantID)
	} else if employerID, ok := filters["employer_id"].(uint); ok {
		query = query.Where("work_hours.tenant_id = ?", employerID)
	}

	if managerID, ok := filters["manager_id"].(uint); ok {
		query = query.Joins("JOIN users ON users.id = work_hours.user_id").Where("users.manager_id = ?", managerID)
	}

	if userID, ok := filters["user_id"].(uint); ok {
		query = query.Where("work_hours.user_id = ?", userID)
	}

	if startDate, ok := filters["start_date"].(time.Time); ok {
		query = query.Where("work_date >= ?", startDate)
	}

	if endDate, ok := filters["end_date"].(time.Time); ok {
		query = query.Where("work_date <= ?", endDate)
	}

	if approved, ok := filters["approved"].(bool); ok {
		query = query.Where("approved = ?", approved)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var workHours []models.WorkHour
	err := query.Preload("User").
		Preload("ApprovedByUser").
		Offset(offset).
		Limit(limit).
		Order("work_date DESC").
		Find(&workHours).Error

	return workHours, total, err
}

func (r *workHourRepository) GetSummary(filters map[string]interface{}) (map[string]float64, error) {
	query := r.db.Model(&models.WorkHour{})

	if tenantID, ok := filters["tenant_id"].(uint); ok {
		query = query.Where("work_hours.tenant_id = ?", tenantID)
	} else if employerID, ok := filters["employer_id"].(uint); ok {
		query = query.Where("work_hours.tenant_id = ?", employerID)
	}

	if userID, ok := filters["user_id"].(uint); ok {
		query = query.Where("work_hours.user_id = ?", userID)
	}

	// Date range filters
	if startDate, ok := filters["start_date"].(time.Time); ok {
		query = query.Where("work_date >= ?", startDate)
	}
	if endDate, ok := filters["end_date"].(time.Time); ok {
		query = query.Where("work_date <= ?", endDate)
	}

	var totalHours float64
	var approvedHours float64

	// Total hours
	if err := query.Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours).Error; err != nil {
		return nil, err
	}

	// Approved hours
	if err := query.Where("approved = true").Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours).Error; err != nil {
		return nil, err
	}

	return map[string]float64{
		"total_hours":    totalHours,
		"approved_hours": approvedHours,
		"pending_hours":  totalHours - approvedHours,
	}, nil
}
func (r *workHourRepository) CountActiveToday() (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.Model(&models.WorkHour{}).Where("work_date >= ?", today).Distinct("user_id").Count(&count).Error
	return count, err
}

func (r *workHourRepository) GetTotalHoursMonth() (float64, error) {
	var total float64
	err := r.db.Model(&models.WorkHour{}).
		Where("work_date >= date_trunc('month', CURRENT_DATE)").
		Select("COALESCE(SUM(hours_worked), 0)").Scan(&total).Error
	return total, err
}
