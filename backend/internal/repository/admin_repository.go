package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type CompanyMetric struct {
	ID             uint    `json:"id"`
	Name           string  `json:"name"`
	Professionals  int     `json:"professionals"`
	HoursThisMonth float64 `json:"hours_this_month"`
	TasksCompleted int     `json:"tasks_completed"`
	ActiveUsers    int     `json:"active_users"`
}

type InactiveUser struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Company      string    `json:"company"`
	LastActive   time.Time `json:"last_active"`
	DaysInactive int       `json:"days_inactive"`
}

type Activity struct {
	Type      string    `json:"type"`
	User      string    `json:"user"`
	Company   string    `json:"company"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

type AdminRepository interface {
	GetCompaniesMetrics() ([]CompanyMetric, error)
	GetInactiveUsersList(since time.Time) ([]InactiveUser, error)
	GetRecentActivities() ([]Activity, error)
	CountInactiveWarning(since time.Time) (int64, error)
	DeleteSuperadmins() error
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) CountInactiveWarning(since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Where("user_type = ? AND id NOT IN (SELECT DISTINCT user_id FROM work_hours WHERE work_date >= ?)", "profesional", since).
		Count(&count).Error
	return count, err
}

func (r *adminRepository) GetCompaniesMetrics() ([]CompanyMetric, error) {
	var companies []CompanyMetric
	rows, err := r.db.Raw(`
		SELECT 
			u.id,
			u.company_name as name,
			COUNT(DISTINCT p.id) as professionals,
			COALESCE(SUM(wh.hours_worked), 0) as hours_this_month,
			COUNT(DISTINCT CASE WHEN t.completed = true THEN t.id END) as tasks_completed,
			COUNT(DISTINCT CASE WHEN wh.work_date >= CURRENT_DATE - INTERVAL '7 days' THEN wh.user_id END) as active_users
		FROM users u
		LEFT JOIN users p ON p.empleador_id = u.id AND p.user_type = 'profesional'
		LEFT JOIN work_hours wh ON wh.user_id = p.id AND wh.work_date >= date_trunc('month', CURRENT_DATE)
		LEFT JOIN tasks t ON t.created_by = p.id AND t.completed = true
		WHERE u.user_type = 'empleador'
		GROUP BY u.id, u.company_name
		ORDER BY hours_this_month DESC
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cm CompanyMetric
		var companyName interface{}
		rows.Scan(&cm.ID, &companyName, &cm.Professionals, &cm.HoursThisMonth, &cm.TasksCompleted, &cm.ActiveUsers)
		switch v := companyName.(type) {
		case []byte:
			cm.Name = string(v)
		case string:
			cm.Name = v
		}
		companies = append(companies, cm)
	}
	return companies, nil
}

func (r *adminRepository) GetInactiveUsersList(since time.Time) ([]InactiveUser, error) {
	var users []InactiveUser
	err := r.db.Raw(`
		SELECT 
			u.id,
			u.name,
			u.email,
			COALESCE(e.company_name, '-') as company,
			COALESCE(MAX(wh.work_date), u.created_at) as last_active,
			EXTRACT(DAY FROM CURRENT_DATE - COALESCE(MAX(wh.work_date), u.created_at)) as days_inactive
		FROM users u
		LEFT JOIN work_hours wh ON wh.user_id = u.id
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE u.user_type = 'profesional'
		GROUP BY u.id, u.name, u.email, e.company_name
		HAVING MAX(wh.work_date) IS NULL OR MAX(wh.work_date) < ?
		ORDER BY days_inactive DESC
		LIMIT 50
	`, since).Scan(&users).Error
	return users, err
}

func (r *adminRepository) GetRecentActivities() ([]Activity, error) {
	var activities []Activity
	err := r.db.Raw(`
		SELECT 
			'work_hour' as type,
			u.name as user,
			COALESCE(e.company_name, '-') as company,
			CASE 
				WHEN wh.work_type = 'complete' THEN 'Registró jornada completa'
				ELSE 'Registró ausencia'
			END as details,
			wh.created_at as timestamp
		FROM work_hours wh
		JOIN users u ON u.id = wh.user_id
		LEFT JOIN users e ON e.id = u.empleador_id
		ORDER BY wh.created_at DESC
		LIMIT 20
	`).Scan(&activities).Error
	return activities, err
}

func (r *adminRepository) DeleteSuperadmins() error {
	return r.db.Where("user_type = ?", "superadmin").Delete(&models.User{}).Error
}
