package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// reportScheduleID es la fila única de configuración global.
const reportScheduleID = 1

type ReportScheduleRepository interface {
	Get() (*models.ReportSchedule, error)
	Update(updates map[string]interface{}) (*models.ReportSchedule, error)
	// HasSuccessfulRun informa si ese período ya se envió con éxito a esa empresa.
	HasSuccessfulRun(tenantID uint, periodKey string) (bool, error)
	RecordRun(run *models.ReportRun) error
	ListRuns(limit int) ([]models.ReportRun, error)
}

type reportScheduleRepository struct {
	db *gorm.DB
}

func NewReportScheduleRepository(db *gorm.DB) ReportScheduleRepository {
	return &reportScheduleRepository{db: db}
}

// Get devuelve la configuración. Si la fila semilla no existiera (BD creada por
// AutoMigrate sin correr la migración), la crea con los valores por defecto.
func (r *reportScheduleRepository) Get() (*models.ReportSchedule, error) {
	var cfg models.ReportSchedule
	err := r.db.First(&cfg, reportScheduleID).Error
	if err == gorm.ErrRecordNotFound {
		cfg = models.ReportSchedule{
			ID:         reportScheduleID,
			Enabled:    false,
			Frequency:  models.ReportFreqMonthly,
			Hour:       8,
			Minute:     0,
			Timezone:   "UTC",
			Weekday:    1,
			DayOfMonth: 1,
		}
		if cerr := r.db.Create(&cfg).Error; cerr != nil {
			return nil, cerr
		}
		return &cfg, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *reportScheduleRepository) Update(updates map[string]interface{}) (*models.ReportSchedule, error) {
	if _, err := r.Get(); err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.ReportSchedule{}).
		Where("id = ?", reportScheduleID).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.Get()
}

func (r *reportScheduleRepository) HasSuccessfulRun(tenantID uint, periodKey string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ReportRun{}).
		Where("tenant_id = ? AND period_key = ? AND status = ?", tenantID, periodKey, models.ReportRunSent).
		Count(&count).Error
	return count > 0, err
}

func (r *reportScheduleRepository) RecordRun(run *models.ReportRun) error {
	return r.db.Create(run).Error
}

func (r *reportScheduleRepository) ListRuns(limit int) ([]models.ReportRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var runs []models.ReportRun
	err := r.db.Order("created_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}
