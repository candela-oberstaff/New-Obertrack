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
	Avatar       string    `json:"avatar"`
	JobTitle     string    `json:"job_title"`
	PhoneNumber  string    `json:"phone_number"`
	Company      string    `json:"company"`
	TenantID     uint      `json:"tenant_id"`
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

type AbsenceReportItem struct {
	ID            uint      `json:"id"`
	UserID        uint      `json:"user_id"`
	User          string    `json:"user"`
	Email         string    `json:"email"`
	PhoneNumber   string    `json:"phone_number"`
	Avatar        string    `json:"avatar"`
	TenantID      uint      `json:"tenant_id"`
	Company       string    `json:"company"`
	WorkDate      time.Time `json:"work_date"`
	HoursWorked   float64   `json:"hours_worked"`
	AbsenceHours  float64   `json:"absence_hours"`
	AbsenceReason string    `json:"absence_reason"`
	Approved      bool      `json:"approved"`
	Rejected      bool      `json:"rejected"`
	CreatedAt     time.Time `json:"created_at"`
}

type AbsenceReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type AbsenceReport struct {
	TotalAbsences int                  `json:"total_absences"`
	AbsenceHours  float64              `json:"absence_hours"`
	PendingReview int                  `json:"pending_review"`
	Approved      int                  `json:"approved"`
	Rejected      int                  `json:"rejected"`
	Reasons       []AbsenceReasonCount `gorm:"-" json:"reasons"`
	Items         []AbsenceReportItem  `gorm:"-" json:"items"`
}

type TenantSummary struct {
	ID             uint      `json:"id"`
	CompanyName    string    `json:"company_name"`
	OwnerName      string    `json:"owner_name"`
	OwnerEmail     string    `json:"owner_email"`
	Industry       string    `json:"industry"`
	Country        string    `json:"country"`
	State          string    `json:"state"`
	City           string    `json:"city"`
	Location       string    `json:"location"`
	Address        string    `json:"address"`
	PhoneNumber    string    `json:"phone_number"`
	IsActive       bool      `json:"is_active"`
	UserCount      int       `json:"user_count"`
	BoardCount     int       `json:"board_count"`
	TaskCount      int       `json:"task_count"`
	HoursThisMonth float64   `json:"hours_this_month"`
	PendingHours   float64   `json:"pending_hours"`
	PendingCount   int64     `json:"pending_count"`
	RejectedCount  int64     `json:"rejected_count"`
	OpenTickets    int       `json:"open_tickets"`
	CreatedAt      time.Time `json:"created_at"`
}

// FollowUpInfo es el estado vigente de gestión de un profesional (la entrada
// más reciente de la bitácora para un kind dado).
type FollowUpInfo struct {
	UserID    uint      `json:"user_id"`
	Status    string    `json:"status"`
	Note      string    `json:"note"`
	ByName    string    `json:"by_name"`
	CreatedAt time.Time `json:"created_at"`
}

// SeniorityItem es una fila del ranking de antigüedad de profesionales.
type SeniorityItem struct {
	UserID       uint      `json:"user_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Avatar       string    `json:"avatar"`
	JobTitle     string    `json:"job_title"`
	Company      string    `json:"company"`
	TenantID     uint      `json:"tenant_id"`
	StartedAt    time.Time `json:"started_at"`
	DaysEmployed int       `json:"days_employed"`
}

type EmployeeSummary struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Avatar         string     `json:"avatar"`
	UserType       string     `json:"user_type"`
	IsActive       bool       `json:"is_active"`
	IsManager      bool       `json:"is_manager"`
	HoursThisMonth float64    `json:"hours_this_month"`
	TasksAssigned  int        `json:"tasks_assigned"`
	TasksCompleted int        `json:"tasks_completed"`
	LastActive     *time.Time `json:"last_active"`
}

type EmployeeWorkHour struct {
	ID          uint      `json:"id"`
	WorkDate    time.Time `json:"work_date"`
	WorkType    string    `json:"work_type"`
	HoursWorked float64   `json:"hours_worked"`
	Approved    bool      `json:"approved"`
	Activities  string    `json:"activities"`
}

type EmployeeTask struct {
	ID        uint       `json:"id"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Completed bool       `json:"completed"`
	EndDate   *time.Time `json:"end_date"`
	BoardName string     `json:"board_name"`
}

type AdminRepository interface {
	GetCompaniesMetrics() ([]CompanyMetric, error)
	GetInactiveUsersList(minDays int) ([]InactiveUser, error)
	GetRecentActivities() ([]Activity, error)
	GetAbsenceReport(startDate, endDate time.Time) (*AbsenceReport, error)
	CountInactiveWarning(since time.Time) (int64, error)
	CountBoards() (int64, error)
	DeleteSuperadmins() error

	GetTenants() ([]TenantSummary, error)
	GetTenantByID(id uint) (*TenantSummary, error)
	GetTenantActivities(tenantID uint) ([]Activity, error)

	GetTenantEmployees(tenantID uint) ([]EmployeeSummary, error)
	GetEmployeeSummary(userID uint) (*EmployeeSummary, error)
	GetEmployeeWorkHours(userID uint, limit int) ([]EmployeeWorkHour, error)
	GetEmployeeTasks(userID uint, limit int) ([]EmployeeTask, error)

	// Dedup de alertas de inactividad (watcher diario).
	GetRecentlyAlertedUserIDs(since time.Time) ([]uint, error)
	MarkUsersAlerted(alerts []models.InactivityAlert) error

	// Ranking de antigüedad de profesionales (métricas de customer success).
	GetSeniorityRanking() ([]SeniorityItem, error)

	// Bitácora de gestión de customer success.
	GetLatestFollowUps(kind string) ([]FollowUpInfo, error)
	CreateFollowUp(followUp *models.FollowUp) error
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

func (r *adminRepository) CountBoards() (int64, error) {
	var count int64
	err := r.db.Model(&models.Board{}).Count(&count).Error
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

// GetInactiveUsersList lista profesionales con minDays o más DÍAS HÁBILES
// completos sin registrar horas (los fines de semana no cuentan como
// inactividad, y el día de hoy tampoco: aún pueden registrar).
func (r *adminRepository) GetInactiveUsersList(minDays int) ([]InactiveUser, error) {
	var all []InactiveUser
	err := r.db.Raw(`
		SELECT
			u.id,
			u.name,
			u.email,
			COALESCE(u.avatar, '') as avatar,
			COALESCE(u.job_title, '') as job_title,
			COALESCE(u.phone_number, '') as phone_number,
			COALESCE(e.company_name, '-') as company,
			COALESCE(u.empleador_id, 0) as tenant_id,
			COALESCE(MAX(wh.work_date), u.created_at) as last_active,
			(SELECT COUNT(*)::int
			 FROM generate_series(
				(COALESCE(MAX(wh.work_date), u.created_at))::date + 1,
				CURRENT_DATE - 1,
				interval '1 day') AS d
			 WHERE EXTRACT(ISODOW FROM d) < 6) as days_inactive
		FROM users u
		LEFT JOIN work_hours wh ON wh.user_id = u.id AND wh.deleted_at IS NULL
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE u.user_type = 'profesional' AND u.deleted_at IS NULL AND u.is_active = true
		GROUP BY u.id, e.company_name
		ORDER BY days_inactive DESC
		LIMIT 1000
	`).Scan(&all).Error
	if err != nil {
		return nil, err
	}

	users := make([]InactiveUser, 0, len(all))
	for _, u := range all {
		if u.DaysInactive >= minDays {
			users = append(users, u)
		}
	}
	return users, nil
}

func (r *adminRepository) GetLatestFollowUps(kind string) ([]FollowUpInfo, error) {
	var items []FollowUpInfo
	err := r.db.Raw(`
		SELECT DISTINCT ON (f.user_id)
			f.user_id,
			f.status,
			f.note,
			COALESCE(u.name, '') as by_name,
			f.created_at
		FROM follow_ups f
		LEFT JOIN users u ON u.id = f.created_by
		WHERE f.kind = ?
		ORDER BY f.user_id, f.created_at DESC
	`, kind).Scan(&items).Error
	return items, err
}

func (r *adminRepository) CreateFollowUp(followUp *models.FollowUp) error {
	return r.db.Create(followUp).Error
}

func (r *adminRepository) GetSeniorityRanking() ([]SeniorityItem, error) {
	var items []SeniorityItem
	err := r.db.Raw(`
		SELECT
			u.id as user_id,
			u.name,
			u.email,
			COALESCE(u.avatar, '') as avatar,
			COALESCE(u.job_title, '') as job_title,
			COALESCE(e.company_name, '-') as company,
			COALESCE(u.empleador_id, 0) as tenant_id,
			u.created_at as started_at,
			EXTRACT(DAY FROM CURRENT_DATE - u.created_at)::int as days_employed
		FROM users u
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE u.user_type = 'profesional' AND u.deleted_at IS NULL AND u.is_active = true
		ORDER BY u.created_at ASC
		LIMIT 500
	`).Scan(&items).Error
	return items, err
}

func (r *adminRepository) GetRecentlyAlertedUserIDs(since time.Time) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.InactivityAlert{}).
		Where("last_alerted_at >= ?", since).
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *adminRepository) MarkUsersAlerted(alerts []models.InactivityAlert) error {
	if len(alerts) == 0 {
		return nil
	}
	return r.db.Save(&alerts).Error
}

func (r *adminRepository) GetRecentActivities() ([]Activity, error) {
	var activities []Activity
	// Cross-domain activity feed: work-hour registrations + approvals + task and
	// board creation, attributed to the acting user with their company. UNION ALL
	// keeps the actor's display name (vs. reading raw audit rows).
	err := r.db.Raw(`
		SELECT type, "user", company, details, timestamp FROM (
			-- Work hours registered
			SELECT
				'work_hour' as type,
				u.name as "user",
				COALESCE(NULLIF(e.company_name, ''), NULLIF(u.company_name, ''), '-') as company,
				CASE WHEN wh.work_type = 'complete' THEN 'Registró jornada completa' ELSE 'Registró una ausencia' END as details,
				wh.created_at as timestamp
			FROM work_hours wh
			JOIN users u ON u.id = wh.user_id
			LEFT JOIN users e ON e.id = u.empleador_id
			WHERE wh.deleted_at IS NULL

			UNION ALL

			-- Work hours approved (actor = approver)
			SELECT
				'approval' as type,
				ap.name as "user",
				COALESCE(NULLIF(e.company_name, ''), NULLIF(ap.company_name, ''), '-') as company,
				'Aprobó un registro de horas' as details,
				wh.approved_at as timestamp
			FROM work_hours wh
			JOIN users ap ON ap.id = wh.approved_by
			LEFT JOIN users e ON e.id = ap.empleador_id
			WHERE wh.approved = true AND wh.approved_by IS NOT NULL AND wh.approved_at IS NOT NULL AND wh.deleted_at IS NULL

			UNION ALL

			-- Tasks created
			SELECT
				'task' as type,
				u.name as "user",
				COALESCE(NULLIF(e.company_name, ''), NULLIF(u.company_name, ''), '-') as company,
				'Creó la tarea: ' || t.title as details,
				t.created_at as timestamp
			FROM tasks t
			JOIN users u ON u.id = t.created_by
			LEFT JOIN users e ON e.id = u.empleador_id
			WHERE t.deleted_at IS NULL

			UNION ALL

			-- Boards created
			SELECT
				'board' as type,
				u.name as "user",
				COALESCE(NULLIF(e.company_name, ''), NULLIF(u.company_name, ''), '-') as company,
				'Creó el tablero: ' || b.name as details,
				b.created_at as timestamp
			FROM boards b
			JOIN users u ON u.id = b.created_by
			LEFT JOIN users e ON e.id = u.empleador_id
			WHERE b.deleted_at IS NULL
		) feed
		ORDER BY timestamp DESC
		LIMIT 25
	`).Scan(&activities).Error
	return activities, err
}

func (r *adminRepository) GetAbsenceReport(startDate, endDate time.Time) (*AbsenceReport, error) {
	report := &AbsenceReport{
		Reasons: []AbsenceReasonCount{},
		Items:   []AbsenceReportItem{},
	}

	err := r.db.Raw(`
		SELECT
			COUNT(*) as total_absences,
			COALESCE(SUM(wh.absence_hours), 0) as absence_hours,
			COUNT(CASE WHEN wh.approved = false AND wh.rejected = false THEN 1 END) as pending_review,
			COUNT(CASE WHEN wh.approved = true THEN 1 END) as approved,
			COUNT(CASE WHEN wh.rejected = true THEN 1 END) as rejected
		FROM work_hours wh
		WHERE wh.work_type = 'absence'
			AND wh.deleted_at IS NULL
			AND wh.work_date BETWEEN ? AND ?
	`, startDate, endDate).Scan(report).Error
	if err != nil {
		return nil, err
	}

	err = r.db.Raw(`
		SELECT
			COALESCE(NULLIF(wh.absence_reason, ''), 'Sin motivo') as reason,
			COUNT(*) as count
		FROM work_hours wh
		WHERE wh.work_type = 'absence'
			AND wh.deleted_at IS NULL
			AND wh.work_date BETWEEN ? AND ?
		GROUP BY reason
		ORDER BY count DESC, reason ASC
		LIMIT 5
	`, startDate, endDate).Scan(&report.Reasons).Error
	if err != nil {
		return nil, err
	}

	err = r.db.Raw(`
		SELECT
			wh.id,
			wh.user_id,
			u.name as user,
			u.email,
			COALESCE(u.phone_number, '') as phone_number,
			COALESCE(u.avatar, '') as avatar,
			COALESCE(u.empleador_id, 0) as tenant_id,
			COALESCE(e.company_name, '-') as company,
			wh.work_date,
			wh.hours_worked,
			wh.absence_hours,
			COALESCE(NULLIF(wh.absence_reason, ''), 'Sin motivo') as absence_reason,
			wh.approved,
			wh.rejected,
			wh.created_at
		FROM work_hours wh
		JOIN users u ON u.id = wh.user_id
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE wh.work_type = 'absence'
			AND wh.deleted_at IS NULL
			AND wh.work_date BETWEEN ? AND ?
		ORDER BY wh.work_date DESC, wh.created_at DESC
		LIMIT 25
	`, startDate, endDate).Scan(&report.Items).Error
	if err != nil {
		return nil, err
	}

	return report, nil
}

func (r *adminRepository) DeleteSuperadmins() error {
	return r.db.Where("user_type = ?", "superadmin").Delete(&models.User{}).Error
}

const tenantSelect = `
	SELECT
		u.id,
		COALESCE(NULLIF(u.company_name, ''), u.name) as company_name,
		u.name as owner_name,
		u.email as owner_email,
		u.industry,
		u.country,
		u.state,
		u.city,
		u.location,
		u.address,
		u.phone_number,
		u.is_active,
		u.created_at,
		COUNT(DISTINCT m.id) as user_count,
		COUNT(DISTINCT b.id) as board_count,
		COUNT(DISTINCT t.id) as task_count,
		COALESCE((SELECT SUM(wh.hours_worked) FROM work_hours wh
			WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL
			AND wh.work_date >= date_trunc('month', CURRENT_DATE)), 0) as hours_this_month,
		COALESCE((SELECT SUM(wh.hours_worked) FROM work_hours wh
			WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL
			AND wh.approved = false AND wh.rejected = false), 0) as pending_hours,
		(SELECT COUNT(*) FROM work_hours wh
			WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL
			AND wh.approved = false AND wh.rejected = false) as pending_count,
		(SELECT COUNT(*) FROM work_hours wh
			WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL
			AND wh.rejected = true) as rejected_count,
		(SELECT COUNT(*) FROM tickets tk
			WHERE tk.deleted_at IS NULL AND tk.status = 'open'
			AND tk.user_id IN (SELECT pu.id FROM users pu WHERE pu.empleador_id = u.id AND pu.deleted_at IS NULL)) as open_tickets
	FROM users u
	LEFT JOIN users m ON m.empleador_id = u.id AND m.deleted_at IS NULL
	LEFT JOIN boards b ON b.tenant_id = u.id AND b.deleted_at IS NULL
	LEFT JOIN tasks t ON t.tenant_id = u.id AND t.deleted_at IS NULL
	WHERE u.user_type = 'empleador' AND u.deleted_at IS NULL
`

func (r *adminRepository) GetTenants() ([]TenantSummary, error) {
	var tenants []TenantSummary
	err := r.db.Raw(tenantSelect + `
		GROUP BY u.id
		ORDER BY LOWER(COALESCE(NULLIF(u.company_name, ''), u.name)) ASC
	`).Scan(&tenants).Error
	return tenants, err
}

func (r *adminRepository) GetTenantByID(id uint) (*TenantSummary, error) {
	var tenant TenantSummary
	err := r.db.Raw(tenantSelect+`
		AND u.id = ?
		GROUP BY u.id
	`, id).Scan(&tenant).Error
	if err != nil {
		return nil, err
	}
	if tenant.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &tenant, nil
}

const employeeMetrics = `
	u.id, u.name, u.email, u.avatar, u.user_type, u.is_active, u.is_manager,
	COALESCE((SELECT SUM(wh.hours_worked) FROM work_hours wh WHERE wh.user_id = u.id AND wh.deleted_at IS NULL AND wh.work_date >= date_trunc('month', CURRENT_DATE)), 0) as hours_this_month,
	(SELECT COUNT(*) FROM task_users tu WHERE tu.user_id = u.id) as tasks_assigned,
	(SELECT COUNT(*) FROM task_users tu JOIN tasks t ON t.id = tu.task_id AND t.deleted_at IS NULL WHERE tu.user_id = u.id AND t.completed = true) as tasks_completed,
	(SELECT MAX(wh.work_date) FROM work_hours wh WHERE wh.user_id = u.id AND wh.deleted_at IS NULL) as last_active
`

func (r *adminRepository) GetTenantEmployees(tenantID uint) ([]EmployeeSummary, error) {
	var employees []EmployeeSummary
	err := r.db.Raw(`
		SELECT `+employeeMetrics+`
		FROM users u
		WHERE u.empleador_id = ? AND u.deleted_at IS NULL
		ORDER BY u.name
	`, tenantID).Scan(&employees).Error
	return employees, err
}

func (r *adminRepository) GetEmployeeSummary(userID uint) (*EmployeeSummary, error) {
	var employee EmployeeSummary
	err := r.db.Raw(`
		SELECT `+employeeMetrics+`
		FROM users u
		WHERE u.id = ? AND u.deleted_at IS NULL
	`, userID).Scan(&employee).Error
	if err != nil {
		return nil, err
	}
	if employee.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &employee, nil
}

func (r *adminRepository) GetEmployeeWorkHours(userID uint, limit int) ([]EmployeeWorkHour, error) {
	var hours []EmployeeWorkHour
	err := r.db.Raw(`
		SELECT id, work_date, work_type, hours_worked, approved, activities
		FROM work_hours
		WHERE user_id = ? AND deleted_at IS NULL
		ORDER BY work_date DESC
		LIMIT ?
	`, userID, limit).Scan(&hours).Error
	return hours, err
}

func (r *adminRepository) GetEmployeeTasks(userID uint, limit int) ([]EmployeeTask, error) {
	var tasks []EmployeeTask
	err := r.db.Raw(`
		SELECT t.id, t.title, t.status, t.completed, t.end_date, COALESCE(b.name, '') as board_name
		FROM tasks t
		JOIN task_users tu ON tu.task_id = t.id
		LEFT JOIN boards b ON b.id = t.board_id
		WHERE tu.user_id = ? AND t.deleted_at IS NULL
		ORDER BY t.created_at DESC
		LIMIT ?
	`, userID, limit).Scan(&tasks).Error
	return tasks, err
}

func (r *adminRepository) GetTenantActivities(tenantID uint) ([]Activity, error) {
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
		WHERE wh.tenant_id = ?
		ORDER BY wh.created_at DESC
		LIMIT 20
	`, tenantID).Scan(&activities).Error
	return activities, err
}
