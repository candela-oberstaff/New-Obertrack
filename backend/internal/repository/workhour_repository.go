package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type WorkHourRepository interface {
	Create(workHour *models.WorkHour) error
	FindByID(id uint) (*models.WorkHour, error)
	FindByIDAndTenant(id, tenantID uint) (*models.WorkHour, error)
	FindManyByIDs(ids []uint) ([]models.WorkHour, error)
	FindManyByIDsAndTenant(ids []uint, tenantID uint) ([]models.WorkHour, error)
	FindByUserAndDate(userID uint, date time.Time, tenantID uint) (*models.WorkHour, error)
	Update(workHour *models.WorkHour) error
	ApproveMultiple(ids []uint, approvedBy uint, approvedAt time.Time) error
	ApproveMultipleAndTenant(ids []uint, approvedBy uint, approvedAt time.Time, tenantID uint) error
	RejectMultiple(ids []uint, rejectedBy uint, rejectedAt time.Time, reason string) error
	RejectMultipleAndTenant(ids []uint, rejectedBy uint, rejectedAt time.Time, reason string, tenantID uint) error
	FindAll(filters map[string]interface{}, offset, limit int) ([]models.WorkHour, int64, error)
	GetSummary(filters map[string]interface{}) (map[string]float64, error)
	// ListAbsences lista las ausencias (work_type='absence') de un usuario en un
	// tenant dentro de un rango de fechas, más recientes primero.
	ListAbsences(userID, tenantID uint, start, end time.Time) ([]models.WorkHour, error)
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

func (r *workHourRepository) FindByIDAndTenant(id, tenantID uint) (*models.WorkHour, error) {
	var workHour models.WorkHour
	err := r.db.Where("tenant_id = ?", tenantID).First(&workHour, id).Error
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

// FindByUserAndDate busca la jornada de un usuario en una fecha dentro de una
// empresa (tenant): un registro por día POR empresa (multi-empresa).
func (r *workHourRepository) FindByUserAndDate(userID uint, date time.Time, tenantID uint) (*models.WorkHour, error) {
	var workHour models.WorkHour
	err := r.db.Where("user_id = ? AND work_date = ? AND tenant_id = ?", userID, date, tenantID).First(&workHour).Error
	return &workHour, err
}

func (r *workHourRepository) Update(workHour *models.WorkHour) error {
	return r.db.Save(workHour).Error
}

func (r *workHourRepository) ApproveMultiple(ids []uint, approvedBy uint, approvedAt time.Time) error {
	return r.db.Model(&models.WorkHour{}).
		Where("id IN ?", ids).
		Select("approved", "approved_by", "approved_at", "rejected", "rejected_by", "rejected_at", "rejection_reason").
		Updates(models.WorkHour{
			Approved:        true,
			ApprovedBy:      &approvedBy,
			ApprovedAt:      &approvedAt,
			Rejected:        false,
			RejectedBy:      nil,
			RejectedAt:      nil,
			RejectionReason: "",
		}).Error
}

func (r *workHourRepository) ApproveMultipleAndTenant(ids []uint, approvedBy uint, approvedAt time.Time, tenantID uint) error {
	return r.db.Model(&models.WorkHour{}).
		Where("id IN ?", ids).
		Where("tenant_id = ?", tenantID).
		Select("approved", "approved_by", "approved_at", "rejected", "rejected_by", "rejected_at", "rejection_reason").
		Updates(models.WorkHour{
			Approved:        true,
			ApprovedBy:      &approvedBy,
			ApprovedAt:      &approvedAt,
			Rejected:        false,
			RejectedBy:      nil,
			RejectedAt:      nil,
			RejectionReason: "",
		}).Error
}

func (r *workHourRepository) RejectMultiple(ids []uint, rejectedBy uint, rejectedAt time.Time, reason string) error {
	return r.db.Model(&models.WorkHour{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"approved":         false,
			"approved_by":      nil,
			"approved_at":      nil,
			"rejected":         true,
			"rejected_by":      rejectedBy,
			"rejected_at":      rejectedAt,
			"rejection_reason": reason,
		}).Error
}

func (r *workHourRepository) RejectMultipleAndTenant(ids []uint, rejectedBy uint, rejectedAt time.Time, reason string, tenantID uint) error {
	return r.db.Model(&models.WorkHour{}).
		Where("id IN ?", ids).
		Where("tenant_id = ?", tenantID).
		Updates(map[string]interface{}{
			"approved":         false,
			"approved_by":      nil,
			"approved_at":      nil,
			"rejected":         true,
			"rejected_by":      rejectedBy,
			"rejected_at":      rejectedAt,
			"rejection_reason": reason,
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

	if managerOrUserID, ok := filters["manager_or_user_id"].(uint); ok {
		query = query.Joins("JOIN users manager_scope ON manager_scope.id = work_hours.user_id").
			Where("work_hours.user_id = ? OR manager_scope.manager_id = ?", managerOrUserID, managerOrUserID)
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

	if rejected, ok := filters["rejected"].(bool); ok {
		query = query.Where("rejected = ?", rejected)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var workHours []models.WorkHour
	err := query.Preload("User").
		Preload("ApprovedByUser").
		Preload("RejectedByUser").
		Offset(offset).
		Limit(limit).
		Order("work_date DESC").
		Find(&workHours).Error

	return workHours, total, err
}

func (r *workHourRepository) GetSummary(filters map[string]interface{}) (map[string]float64, error) {
	baseQuery := func() *gorm.DB {
		query := r.db.Model(&models.WorkHour{})

		if tenantID, ok := filters["tenant_id"].(uint); ok {
			query = query.Where("work_hours.tenant_id = ?", tenantID)
		} else if employerID, ok := filters["employer_id"].(uint); ok {
			query = query.Where("work_hours.tenant_id = ?", employerID)
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

		return query
	}

	var totalHours float64
	var approvedHours float64
	var rejectedHours float64

	if err := baseQuery().Select("COALESCE(SUM(hours_worked), 0)").Scan(&totalHours).Error; err != nil {
		return nil, err
	}

	if err := baseQuery().Where("approved = true").Select("COALESCE(SUM(hours_worked), 0)").Scan(&approvedHours).Error; err != nil {
		return nil, err
	}

	if err := baseQuery().Where("rejected = true").Select("COALESCE(SUM(hours_worked), 0)").Scan(&rejectedHours).Error; err != nil {
		return nil, err
	}

	pendingHours := totalHours - approvedHours - rejectedHours
	if pendingHours < 0 {
		pendingHours = 0
	}

	return map[string]float64{
		"total_hours":    totalHours,
		"approved_hours": approvedHours,
		"pending_hours":  pendingHours,
		"rejected_hours": rejectedHours,
	}, nil
}
func (r *workHourRepository) ListAbsences(userID, tenantID uint, start, end time.Time) ([]models.WorkHour, error) {
	var absences []models.WorkHour
	err := r.db.
		Where("user_id = ? AND tenant_id = ? AND work_type = ?", userID, tenantID, models.WorkTypeAbsence).
		Where("work_date >= ? AND work_date <= ?", start, end).
		Order("work_date DESC").
		Find(&absences).Error
	return absences, err
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
