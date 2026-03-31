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
	GetDB() *gorm.DB
	CountUsersByType(userType string) (int64, error)
	CountManagers() (int64, error)
	GetTotalHoursWorked() (float64, error)
	GetApprovedHours() (float64, error)
	CountTasks() (int64, error)
	CountCompletedTasks() (int64, error)
	CountActiveToday() (int64, error)
	CountInactiveWarning(since time.Time) (int64, error)

	GetCompaniesMetrics() ([]CompanyMetric, error)
	GetInactiveUsersList(since time.Time) ([]InactiveUser, error)
	GetRecentActivities() ([]Activity, error)

	GetAllUsers(userType, isManager, isActive string, offset, limit int) ([]models.User, int64, error)
	GetUserByID(id uint) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	CreateUser(user *models.User) error
	UpdateUser(user *models.User) error
	DeleteUserAllData(id uint) error
	
	CountActiveUsers() (int64, error)
	GetTotalHoursMonth() (float64, error)
	DeleteSuperadmins() error
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *adminRepository) CountUsersByType(userType string) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("user_type = ?", userType).Count(&count).Error
	return count, err
}

func (r *adminRepository) CountManagers() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("is_manager = ?", true).Count(&count).Error
	return count, err
}

func (r *adminRepository) GetTotalHoursWorked() (float64, error) {
	var total float64
	err := r.db.Model(&models.WorkHour{}).Select("COALESCE(SUM(hours_worked), 0)").Scan(&total).Error
	return total, err
}

func (r *adminRepository) GetApprovedHours() (float64, error) {
	var approved float64
	err := r.db.Model(&models.WorkHour{}).Where("approved = ?", true).Select("COALESCE(SUM(hours_worked), 0)").Scan(&approved).Error
	return approved, err
}

func (r *adminRepository) CountTasks() (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Count(&count).Error
	return count, err
}

func (r *adminRepository) CountCompletedTasks() (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Where("completed = ?", true).Count(&count).Error
	return count, err
}

func (r *adminRepository) CountActiveToday() (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.Model(&models.WorkHour{}).Where("work_date >= ?", today).Distinct("user_id").Count(&count).Error
	return count, err
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

func (r *adminRepository) GetAllUsers(userType, isManager, isActive string, offset, limit int) ([]models.User, int64, error) {
	var users []models.User
	query := r.db.Model(&models.User{})

	if userType != "" {
		query = query.Where("user_type = ?", userType)
	}
	if isManager != "" {
		query = query.Where("is_manager = ?", isManager == "true")
	}
	if isActive != "" {
		query = query.Where("is_active = ?", isActive == "true")
	}

	var total int64
	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&users).Error
	return users, total, err
}

func (r *adminRepository) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	return &user, err
}

func (r *adminRepository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *adminRepository) CreateUser(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *adminRepository) UpdateUser(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *adminRepository) CountActiveUsers() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("is_active = ?", true).Count(&count).Error
	return count, err
}

func (r *adminRepository) GetTotalHoursMonth() (float64, error) {
	var total float64
	err := r.db.Model(&models.WorkHour{}).
		Where("work_date >= date_trunc('month', CURRENT_DATE)").
		Select("COALESCE(SUM(hours_worked), 0)").Scan(&total).Error
	return total, err
}

func (r *adminRepository) DeleteUserAllData(id uint) error {
	tx := r.db.Begin()

	if err := tx.Where("user_id = ?", id).Delete(&models.WorkHour{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("approved_by = ?", id).Update("approved_by", nil).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Unscoped().Delete(&models.User{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *adminRepository) DeleteSuperadmins() error {
	return r.db.Where("user_type = ?", "superadmin").Delete(&models.User{}).Error
}
