package repository

import (
	"database/sql"
	"strings"
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
	// ID es el de la fila de origen (jornada, tarea, tablero). Solo es único
	// junto al tipo: una misma jornada aparece como registro y como aprobación.
	ID        uint      `json:"id"`
	Type      string    `json:"type"`
	User      string    `json:"user"`
	Company   string    `json:"company"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// Tamaño de página del feed global. El máximo existe para que un limit inventado
// desde fuera no obligue a materializar el UNION entero.
const (
	defaultActivityPageSize = 25
	MaxActivityPageSize     = 100
)

// ActivityCursor marca el último evento ya entregado, para pedir los anteriores.
// Lleva tipo e id además de la fecha porque dos eventos pueden compartir
// timestamp al milisegundo (una carga masiva, un seed): ordenar solo por fecha
// haría que uno de los empatados se perdiera justo en el corte de página.
type ActivityCursor struct {
	Timestamp time.Time
	Type      string
	ID        uint
}

// Categorías del expediente de empresa. Agrupan los tipos de evento en las
// cuatro preguntas que se le hacen a la ficha: qué pasó con el acceso, quién
// entró o salió, cuánta actividad hay y qué ha hecho el equipo con la cuenta.
const (
	TenantActivityLifecycle  = "lifecycle"  // alta, suspensión, reactivación
	TenantActivityStaff      = "staff"      // altas y bajas de profesionales
	TenantActivityWork       = "work"       // registros de jornada y ausencias
	TenantActivityManagement = "management" // gestiones de customer success
	// TenantActivityNote va aparte de las gestiones: es lo único que escribe
	// una persona a mano, y quien busca "qué anotamos de esta empresa" no
	// quiere que se lo mezclen con los seguimientos automáticos de CS.
	TenantActivityNote = "note"
	// TenantActivityContact son los contactos del equipo HACIA la empresa
	// (correo, WhatsApp, llamada, reunión). Categoría propia porque responde a
	// la pregunta que más se hace soporte al abrir una ficha —"¿ya hablamos con
	// ellos?"— y mezclarla con las notas la volvería a esconder.
	TenantActivityContact = "contact"
)

// TenantActivityPerson es una persona que aparece en el expediente, para
// poder ofrecerla como filtro.
type TenantActivityPerson struct {
	UserID uint   `json:"user_id"`
	Name   string `json:"name"`
}

// TenantActivity es una entrada del expediente, ya clasificada.
type TenantActivity struct {
	Type      string    `json:"type"`
	Category  string    `json:"category"`
	User      string    `json:"user"`
	UserID    uint      `json:"user_id"`
	Company   string    `json:"company"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
	// EventID apunta a la fila de company_events cuando la entrada nace de una
	// (notas, suspensiones). Es 0 en las derivadas de otras tablas, que no se
	// gestionan desde el expediente.
	EventID uint `json:"event_id"`
	// Solo tienen sentido en las notas; en el resto son siempre false/nil.
	Pinned   bool       `json:"pinned"`
	EditedAt *time.Time `json:"edited_at,omitempty"`
	// Channel solo viene en los contactos (email, whatsapp, call, meeting) y
	// va aparte del texto para que la interfaz pueda distinguirlos de un
	// vistazo sin leer el detalle.
	Channel string `json:"channel,omitempty"`
}

// TenantActivityCount es cuántos movimientos hay de una categoría, con el
// filtro de persona ya aplicado.
type TenantActivityCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
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

// ArchivedEntry es un profesional "archivado": con empleo finalizado (baja) o
// con la cuenta desactivada. Sirve para listarlos y poder reactivarlos.
type ArchivedEntry struct {
	Kind         string    `json:"kind"` // ended_employment | deactivated_user
	UserID       uint      `json:"user_id"`
	EmploymentID uint      `json:"employment_id"` // 0 si es solo cuenta desactivada
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Avatar       string    `json:"avatar"`
	Company      string    `json:"company"`
	CompanyID    uint      `json:"company_id"`
	JobTitle     string    `json:"job_title"`
	Reason       string    `json:"reason"`
	ArchivedAt   time.Time `json:"archived_at"`
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
	// Señales de salud de la cuenta. Nulos cuando nunca ha pasado: "todavía no
	// la hemos contactado" y "nunca la hemos contactado" se ven igual, y es la
	// respuesta honesta en ambos casos.
	LastContactAt  *time.Time `json:"last_contact_at,omitempty"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
}

// TenantTicket es un ticket visto desde la ficha de la empresa. Aplana lo justo
// para la tabla: de quién va y quién lo lleva, sin arrastrar el hilo entero.
type TenantTicket struct {
	ID     uint   `json:"id"`
	Origin string `json:"origin"` // internal | whatsapp | zoho
	Title  string `json:"title"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	// About es sobre quién va: el contacto de WhatsApp o el profesional de la
	// alerta interna. Vacío si el ticket no apunta a ninguno.
	About     string    `json:"about"`
	Assignee  string    `json:"assignee"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	// Rejected viaja aparte de Approved: sin él, la ficha del profesional
	// pintaba como "Pendiente" una jornada que en realidad se rechazó, que es
	// justo lo contrario de lo que hay que atender.
	Rejected        bool    `json:"rejected"`
	RejectionReason string  `json:"rejection_reason"`
	Comments        string  `json:"comments"`
	AbsenceReason   string  `json:"absence_reason"`
	AbsenceHours    float64 `json:"absence_hours"`
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
	GetInactiveUsersList(tenantID uint, minDays int) ([]InactiveUser, error)
	// GetRecentActivities devuelve una página del feed global, de la más
	// reciente hacia atrás. cursor nil = primera página.
	GetRecentActivities(cursor *ActivityCursor, limit int) ([]Activity, error)
	GetAbsenceReport(tenantID uint, startDate, endDate time.Time) (*AbsenceReport, error)
	CountInactiveWarning(since time.Time) (int64, error)
	CountBoards() (int64, error)
	DeleteSuperadmins() error

	GetTenants() ([]TenantSummary, error)
	GetTenantByID(id uint) (*TenantSummary, error)
	// GetTenantActivities devuelve una página del expediente y el total que
	// cumple el filtro (para poder paginar sin una segunda consulta).
	// category vacía = todas; userID 0 = todas las personas.
	GetTenantActivities(tenantID uint, category string, userID uint, offset, limit int) ([]TenantActivity, int64, error)
	// GetTenantActivityPeople lista las personas que aparecen en el expediente.
	GetTenantActivityPeople(tenantID uint) ([]TenantActivityPerson, error)
	// GetTenantActivityCounts cuenta por categoría (con el filtro de persona
	// aplicado), para los contadores de los chips.
	GetTenantActivityCounts(tenantID uint, userID uint) ([]TenantActivityCount, error)
	// GetTenantPinnedNotes trae las notas fijadas en la cabecera.
	GetTenantPinnedNotes(tenantID uint) ([]TenantActivity, error)

	GetTenantEmployees(tenantID uint) ([]EmployeeSummary, error)
	// GetTenantTickets lista los tickets de la empresa (alertas internas sobre
	// su gente + conversaciones de WhatsApp de su número).
	GetTenantTickets(tenantID uint) ([]TenantTicket, error)
	// GetEmployeeTickets lista los tickets que apuntan a UN profesional, sin el
	// rodeo de la empresa: los de su ficha son los suyos, no los de su tenant.
	GetEmployeeTickets(userID uint) ([]TenantTicket, error)
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

	// Eventos del ciclo de vida de una empresa (expediente).
	CreateCompanyEvent(event *models.CompanyEvent) error
	// DeleteCompanyNote borra una anotación manual. Acotado a la empresa y al
	// tipo "note": el resto del expediente es historial y no se toca. Devuelve
	// cuántas filas se borraron (0 = no existe o no era una nota).
	DeleteCompanyNote(companyID, noteID uint) (int64, error)
	// UpdateCompanyNote corrige el texto y marca la nota como editada.
	UpdateCompanyNote(companyID, noteID uint, detail string, editedAt time.Time) (int64, error)
	// SetCompanyNotePinned fija o desfija una nota en la cabecera.
	SetCompanyNotePinned(companyID, noteID uint, pinned bool) (int64, error)

	// Archivados: bajas de empleo + cuentas desactivadas. tenantID=0 = global.
	GetArchived(tenantID uint) ([]ArchivedEntry, error)
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
		WHERE u.user_type = 'empleador' AND u.deleted_at IS NULL
		  AND (COALESCE(TRIM(u.company_name), '') <> '' OR COALESCE(TRIM(u.name), '') <> '')
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
// GetInactiveUsersList lista profesionales sin registrar horas. tenantID 0 =
// todas las empresas (panel de admin); con valor, solo la de esa ficha.
func (r *adminRepository) GetInactiveUsersList(tenantID uint, minDays int) ([]InactiveUser, error) {
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
			AND (@tid = 0 OR u.empleador_id = @tid)
		GROUP BY u.id, e.company_name
		ORDER BY days_inactive DESC
		LIMIT 1000
	`, sql.Named("tid", tenantID)).Scan(&all).Error
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

func (r *adminRepository) CreateCompanyEvent(event *models.CompanyEvent) error {
	return r.db.Create(event).Error
}

// GetArchived lista profesionales archivados: empleos finalizados (baja) y
// cuentas desactivadas. Con tenantID=0 devuelve el global; si no, los de esa
// empresa. Un mismo usuario puede aparecer en ambas categorías.
func (r *adminRepository) GetArchived(tenantID uint) ([]ArchivedEntry, error) {
	var entries []ArchivedEntry
	err := r.db.Raw(`
		SELECT 'ended_employment' as kind, emp.id as employment_id, u.id as user_id,
			u.name, u.email, COALESCE(u.avatar, '') as avatar,
			COALESCE(owner.company_name, '-') as company, emp.company_id,
			COALESCE(emp.job_title, '') as job_title,
			COALESCE(emp.end_reason, '') as reason, emp.ended_at as archived_at
		FROM employments emp
		JOIN users u ON u.id = emp.user_id
		JOIN users owner ON owner.id = emp.company_id
		WHERE emp.status = 'ended' AND emp.deleted_at IS NULL
			AND (@tid = 0 OR emp.company_id = @tid)

		UNION ALL

		SELECT 'deactivated_user' as kind, 0 as employment_id, u.id as user_id,
			u.name, u.email, COALESCE(u.avatar, '') as avatar,
			COALESCE(owner.company_name, '-') as company, COALESCE(u.empleador_id, 0) as company_id,
			COALESCE(u.job_title, '') as job_title,
			'' as reason, u.updated_at as archived_at
		FROM users u
		LEFT JOIN users owner ON owner.id = u.empleador_id
		WHERE u.is_active = false AND u.deleted_at IS NULL
			AND u.user_type IN ('profesional', 'customer_success')
			AND (@tid = 0 OR u.empleador_id = @tid)

		ORDER BY archived_at DESC
	`, sql.Named("tid", tenantID)).Scan(&entries).Error
	return entries, err
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

func (r *adminRepository) GetRecentActivities(cursor *ActivityCursor, limit int) ([]Activity, error) {
	var activities []Activity
	if limit <= 0 {
		limit = defaultActivityPageSize
	}

	// El corte de página va por keyset y no por OFFSET: el feed crece por arriba
	// mientras se navega, y con OFFSET cada evento nuevo empuja una fila ya vista
	// a la página siguiente (se ve repetida y la de abajo se pierde).
	where := ""
	args := []interface{}{}
	if cursor != nil {
		where = "WHERE (feed.timestamp, feed.type, feed.id) < (?::timestamptz, ?::text, ?::bigint)"
		args = append(args, cursor.Timestamp, cursor.Type, int64(cursor.ID))
	}
	args = append(args, limit)

	// Cross-domain activity feed: work-hour registrations + approvals + task and
	// board creation, attributed to the acting user with their company. UNION ALL
	// keeps the actor's display name (vs. reading raw audit rows).
	err := r.db.Raw(`
		SELECT id, type, "user", company, details, timestamp FROM (
			-- Work hours registered
			SELECT
				wh.id as id,
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
				wh.id as id,
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
				t.id as id,
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
				b.id as id,
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
		`+where+`
		ORDER BY feed.timestamp DESC, feed.type DESC, feed.id DESC
		LIMIT ?
	`, args...).Scan(&activities).Error
	return activities, err
}

// GetAbsenceReport resume las ausencias del periodo. tenantID 0 = todas las
// empresas; con valor, solo las jornadas registradas para esa empresa (se filtra
// por el tenant de la jornada, no por el empleador actual de la persona: si
// cambió de empresa, sus ausencias siguen contando donde ocurrieron).
func (r *adminRepository) GetAbsenceReport(tenantID uint, startDate, endDate time.Time) (*AbsenceReport, error) {
	report := &AbsenceReport{
		Reasons: []AbsenceReasonCount{},
		Items:   []AbsenceReportItem{},
	}
	scope := []interface{}{
		sql.Named("tid", tenantID),
		sql.Named("start", startDate),
		sql.Named("end", endDate),
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
			AND wh.work_date BETWEEN @start AND @end
			AND (@tid = 0 OR wh.tenant_id = @tid)
	`, scope...).Scan(report).Error
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
			AND wh.work_date BETWEEN @start AND @end
			AND (@tid = 0 OR wh.tenant_id = @tid)
		GROUP BY reason
		ORDER BY count DESC, reason ASC
		LIMIT 5
	`, scope...).Scan(&report.Reasons).Error
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
			AND wh.work_date BETWEEN @start AND @end
			AND (@tid = 0 OR wh.tenant_id = @tid)
		ORDER BY wh.work_date DESC, wh.created_at DESC
		LIMIT 25
	`, scope...).Scan(&report.Items).Error
	if err != nil {
		return nil, err
	}

	return report, nil
}

func (r *adminRepository) DeleteSuperadmins() error {
	return r.db.Where("user_type = ?", "superadmin").Delete(&models.User{}).Error
}

// companyPhoneDigits normaliza dentro de SQL el teléfono de la empresa igual que
// utils.NormalizePhoneDigits en Go: solo dígitos y sin el prefijo internacional
// "00". Espera el usuario-empresa con el alias `u`.
const companyPhoneDigits = `regexp_replace(regexp_replace(COALESCE(u.phone_number, ''), '\D', '', 'g'), '^00(.)', '\1')`

// tenantTicketScope acota qué tickets son de una empresa. Espera los alias `tk`
// (tickets) y `u` (el usuario-empresa).
//
// Son tres fuentes distintas, y hasta ahora solo se contaba la primera —por eso
// el KPI "Tickets abiertos" se quedaba corto—:
//
//  1. Alertas internas sobre sus profesionales (rechazos de horas), que apuntan
//     al profesional por user_id.
//  2. Alertas internas sobre el propio responsable de la empresa.
//  3. Conversaciones de WhatsApp/Zoho, que NO tienen user_id: van por contacto,
//     así que se cruzan por teléfono. El contacto guarda el número como lo da
//     WAHA y la ficha como lo tecleó una persona, de ahí la normalización a
//     ambos lados.
//
// El guardia del NULLIF es lo que evita el fallo silencioso: sin él, una empresa
// sin teléfono normaliza a cadena vacía y se adjudicaría todos los contactos que
// tampoco tienen número.
const tenantTicketScope = `
	tk.deleted_at IS NULL
	AND (
		tk.user_id IN (SELECT pu.id FROM users pu WHERE pu.empleador_id = u.id AND pu.deleted_at IS NULL)
		OR tk.user_id = u.id
		OR (
			NULLIF(` + companyPhoneDigits + `, '') IS NOT NULL
			AND EXISTS (
				SELECT 1 FROM contacts ct
				WHERE ct.id = tk.contact_id AND ct.deleted_at IS NULL
				AND (
					regexp_replace(COALESCE(ct.phone, ''), '\D', '', 'g') = ` + companyPhoneDigits + `
					OR regexp_replace(COALESCE(ct.wa_id, ''), '\D', '', 'g') = ` + companyPhoneDigits + `
				)
			)
		)
	)
`

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
			WHERE tk.status = 'open' AND ` + tenantTicketScope + `) as open_tickets,
		-- Última vez que NOSOTROS contactamos con la empresa. Es la pregunta que
		-- más se hace soporte al abrir una ficha, y hasta ahora había que
		-- deducirla leyendo el expediente entero.
		(SELECT MAX(ce.created_at) FROM company_events ce
			WHERE ce.company_id = u.id AND ce.type = 'contact') as last_contact_at,
		-- Última señal de vida de ELLOS: la jornada o la tarea más reciente. No
		-- se mezcla con lo anterior a propósito —una empresa a la que llamamos
		-- ayer pero que no entra hace dos meses es justo el caso que hay que
		-- ver, y un solo campo lo escondería—.
		GREATEST(
			(SELECT MAX(wh.created_at) FROM work_hours wh
				WHERE wh.tenant_id = u.id AND wh.deleted_at IS NULL),
			(SELECT MAX(t2.created_at) FROM tasks t2
				WHERE t2.tenant_id = u.id AND t2.deleted_at IS NULL)
		) as last_activity_at
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

// GetTenantTickets lista los tickets de una empresa: las alertas internas sobre
// su gente y las conversaciones de WhatsApp de su número, en la misma tabla.
// Usa el mismo criterio que el KPI (tenantTicketScope), así que el contador de
// la cabecera y la lista nunca se contradicen.
//
// Los abiertos van primero: quien abre esta pestaña viene a ver qué hay
// pendiente, no a leer el archivo.
func (r *adminRepository) GetTenantTickets(tenantID uint) ([]TenantTicket, error) {
	var tickets []TenantTicket
	err := r.db.Raw(`
		SELECT
			tk.id,
			tk.origin,
			COALESCE(tk.title, '') as title,
			COALESCE(tk.stage, '') as stage,
			COALESCE(tk.status, '') as status,
			COALESCE(NULLIF(ct.name, ''), NULLIF(pu.name, ''), '') as about,
			COALESCE(ag.name, '') as assignee,
			tk.created_at,
			tk.updated_at
		FROM tickets tk
		JOIN users u ON u.id = ?
		LEFT JOIN contacts ct ON ct.id = tk.contact_id AND ct.deleted_at IS NULL
		LEFT JOIN users pu ON pu.id = tk.user_id
		LEFT JOIN users ag ON ag.id = tk.assigned_to
		WHERE `+tenantTicketScope+`
		ORDER BY CASE WHEN tk.status = 'open' THEN 0 ELSE 1 END, tk.updated_at DESC
		LIMIT 200
	`, tenantID).Scan(&tickets).Error
	return tickets, err
}

// GetEmployeeTickets lista los tickets que van SOBRE un profesional. Solo por
// user_id: las conversaciones de WhatsApp se cruzan por teléfono contra la
// empresa (ver tenantTicketScope) y adjudicárselas a una persona por su número
// mezclaría los tickets de quien comparte el móvil de la oficina.
func (r *adminRepository) GetEmployeeTickets(userID uint) ([]TenantTicket, error) {
	var tickets []TenantTicket
	err := r.db.Raw(`
		SELECT
			tk.id,
			tk.origin,
			COALESCE(tk.title, '') as title,
			COALESCE(tk.stage, '') as stage,
			COALESCE(tk.status, '') as status,
			COALESCE(NULLIF(pu.name, ''), '') as about,
			COALESCE(ag.name, '') as assignee,
			tk.created_at,
			tk.updated_at
		FROM tickets tk
		LEFT JOIN users pu ON pu.id = tk.user_id
		LEFT JOIN users ag ON ag.id = tk.assigned_to
		WHERE tk.deleted_at IS NULL AND tk.user_id = ?
		ORDER BY CASE WHEN tk.status = 'open' THEN 0 ELSE 1 END, tk.updated_at DESC
		LIMIT 100
	`, userID).Scan(&tickets).Error
	return tickets, err
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
		SELECT id, work_date, work_type, hours_worked, approved, activities,
			rejected, COALESCE(rejection_reason, '') as rejection_reason,
			COALESCE(comments, '') as comments,
			COALESCE(absence_reason, '') as absence_reason,
			COALESCE(absence_hours, 0) as absence_hours
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

// GetTenantActivities arma el EXPEDIENTE de la empresa: una línea de tiempo del
// ciclo de vida completo, desde el alta hasta la baja. Une seis fuentes:
//  1) Alta de la empresa (created_at del empleador).
//  2) Altas de empleados (employments iniciados).
//  3) Bajas de empleados (employments finalizados).
//  4) Registros de horas (work_hours).
//  5) Gestiones de CS (follow_ups, acotadas vía employments).
//  6) Suspensiones, reactivaciones y notas del equipo (company_events).
//
// Cada rama declara su categoría, para poder filtrar la línea de tiempo sin
// que el frontend tenga que saberse la lista de tipos. El total sale con una
// window function sobre el conjunto ya filtrado: se pagina sin una segunda
// consulta y sin traerse el expediente entero para contarlo.
//
// La columna se llama "user" (entre comillas) porque es palabra reservada en
// Postgres; dentro del CTE viaja como "actor" para no tener que escaparla en
// cada rama de la unión.
// tenantEventsCTE es la unión que arma el expediente. Vive en una constante
// porque la consumen dos consultas —la línea de tiempo y la lista de personas
// que aparecen en ella—, y deben ver exactamente los mismos movimientos: si se
// duplicara, un filtro por persona podría ofrecer a alguien sin resultados.
//
// Espera el parámetro @tid. Cada rama aporta también quién protagoniza el
// movimiento (actor_id) para poder filtrar por persona sin cruzar por nombre,
// que ni identifica ni sobrevive a dos empleados homónimos.
var tenantEventsCTE = `
	SELECT 'company_created' as type, ` + quoted(TenantActivityLifecycle) + ` as category,
		owner.name as actor, owner.id as actor_id,
		COALESCE(owner.company_name, '-') as company,
		'Empresa registrada en la plataforma' as details,
		owner.created_at as timestamp, 0 as event_id,
		false as pinned, NULL::timestamptz as edited_at, '' as channel
	FROM users owner WHERE owner.id = @tid

	UNION ALL

	SELECT 'employee_joined' as type, ` + quoted(TenantActivityStaff) + ` as category,
		u.name as actor, u.id as actor_id,
		COALESCE(owner.company_name, '-') as company,
		'Se incorporó' ||
			(CASE WHEN COALESCE(emp.job_title, '') <> '' THEN ' como ' || emp.job_title ELSE '' END) ||
			(CASE WHEN COALESCE(emp.start_reason, '') <> '' THEN ' — ' || emp.start_reason ELSE '' END) as details,
		emp.started_at as timestamp, 0 as event_id,
		false as pinned, NULL::timestamptz as edited_at, '' as channel
	FROM employments emp
	JOIN users u ON u.id = emp.user_id
	JOIN users owner ON owner.id = @tid
	WHERE emp.company_id = @tid AND emp.deleted_at IS NULL

	UNION ALL

	SELECT 'employee_left' as type, ` + quoted(TenantActivityStaff) + ` as category,
		u.name as actor, u.id as actor_id,
		COALESCE(owner.company_name, '-') as company,
		'Finalizó su empleo' ||
			(CASE WHEN COALESCE(emp.end_reason, '') <> '' THEN ' — ' || emp.end_reason ELSE '' END) as details,
		emp.ended_at as timestamp, 0 as event_id,
		false as pinned, NULL::timestamptz as edited_at, '' as channel
	FROM employments emp
	JOIN users u ON u.id = emp.user_id
	JOIN users owner ON owner.id = @tid
	WHERE emp.company_id = @tid AND emp.deleted_at IS NULL
		AND emp.status = 'ended' AND emp.ended_at IS NOT NULL

	UNION ALL

	SELECT 'work_hour' as type, ` + quoted(TenantActivityWork) + ` as category,
		u.name as actor, u.id as actor_id,
		COALESCE(e.company_name, '-') as company,
		CASE WHEN wh.work_type = 'complete' THEN 'Registró jornada completa' ELSE 'Registró ausencia' END as details,
		wh.created_at as timestamp, 0 as event_id,
		false as pinned, NULL::timestamptz as edited_at, '' as channel
	FROM work_hours wh
	JOIN users u ON u.id = wh.user_id
	LEFT JOIN users e ON e.id = u.empleador_id
	WHERE wh.tenant_id = @tid

	UNION ALL

	SELECT 'follow_up' as type, ` + quoted(TenantActivityManagement) + ` as category,
		u.name as actor, u.id as actor_id,
		COALESCE(c.company_name, '-') as company,
		'Gestión de ' ||
			(CASE f.kind WHEN 'inactivity' THEN 'inactividad' WHEN 'absence' THEN 'ausencia' ELSE f.kind END) ||
			': ' ||
			(CASE f.status WHEN 'contacted' THEN 'Contactado' WHEN 'justified' THEN 'Justificado' WHEN 'escalated' THEN 'Escalado' ELSE f.status END) ||
			(CASE WHEN COALESCE(f.note, '') <> '' THEN ' — ' || f.note ELSE '' END) as details,
		f.created_at as timestamp, 0 as event_id,
		false as pinned, NULL::timestamptz as edited_at, '' as channel
	FROM follow_ups f
	JOIN users u ON u.id = f.user_id
	LEFT JOIN users c ON c.id = @tid
	WHERE EXISTS (
		SELECT 1 FROM employments emp
		WHERE emp.user_id = f.user_id AND emp.company_id = @tid AND emp.deleted_at IS NULL
	)

	UNION ALL

	SELECT 'company_' || ce.type as type,
		(CASE ce.type
			WHEN 'note' THEN ` + quoted(TenantActivityNote) + `
			WHEN 'contact' THEN ` + quoted(TenantActivityContact) + `
			ELSE ` + quoted(TenantActivityLifecycle) + ` END) as category,
		COALESCE(actor.name, '') as actor, COALESCE(actor.id, 0) as actor_id,
		COALESCE(owner.company_name, '-') as company,
		(CASE ce.type
			WHEN 'suspended' THEN 'Acceso suspendido'
			WHEN 'reactivated' THEN 'Acceso reactivado'
			WHEN 'note' THEN COALESCE(NULLIF(ce.detail, ''), 'Nota sin contenido')
			WHEN 'contact' THEN
				(CASE ce.channel
					WHEN 'email' THEN 'Correo enviado a la empresa'
					WHEN 'whatsapp' THEN 'WhatsApp enviado a la empresa'
					WHEN 'call' THEN 'Llamada telefónica'
					WHEN 'meeting' THEN 'Reunión con la empresa'
					ELSE 'Contacto con la empresa' END)
				|| (CASE WHEN COALESCE(ce.detail, '') <> '' THEN ' — ' || ce.detail ELSE '' END)
			ELSE ce.type END) as details,
		ce.created_at as timestamp, ce.id as event_id,
		ce.pinned, ce.edited_at, COALESCE(ce.channel, '') as channel
	FROM company_events ce
	JOIN users owner ON owner.id = ce.company_id
	LEFT JOIN users actor ON actor.id = ce.by_user_id
	WHERE ce.company_id = @tid
`

func (r *adminRepository) GetTenantActivities(tenantID uint, category string, userID uint, offset, limit int) ([]TenantActivity, int64, error) {
	// Fila plana (y no un struct con TenantActivity embebido) para no depender
	// de cómo promociona GORM los campos anónimos al escanear un Raw.
	rows := []struct {
		Type      string
		Category  string
		User      string
		UserID    uint
		Company   string
		Details   string
		Timestamp time.Time
		EventID   uint
		Pinned    bool
		EditedAt  *time.Time
		Channel   string
		Total     int64
	}{}

	err := r.db.Raw(`
		WITH events AS (`+tenantEventsCTE+`)
		SELECT type, category, actor AS "user", actor_id AS user_id, company, details, timestamp, event_id,
			pinned, edited_at, channel,
			COUNT(*) OVER() AS total
		FROM events
		WHERE (@cat = '' OR category = @cat)
			AND (@uid = 0 OR actor_id = @uid)
		ORDER BY timestamp DESC
		LIMIT @limit OFFSET @offset
	`,
		sql.Named("tid", tenantID),
		sql.Named("cat", category),
		sql.Named("uid", userID),
		sql.Named("limit", limit),
		sql.Named("offset", offset),
	).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	activities := make([]TenantActivity, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, TenantActivity{
			Type:      row.Type,
			Category:  row.Category,
			User:      row.User,
			UserID:    row.UserID,
			Company:   row.Company,
			Details:   row.Details,
			Timestamp: row.Timestamp,
			EventID:   row.EventID,
			Pinned:    row.Pinned,
			EditedAt:  row.EditedAt,
			Channel:   row.Channel,
		})
	}
	// Sin filas no hay ventana de la que leer el total; en la última página
	// vacía (p. ej. tras borrar una nota) el total real es el que diga el
	// siguiente refresco, así que 0 es la respuesta honesta aquí.
	var total int64
	if len(rows) > 0 {
		total = rows[0].Total
	}
	return activities, total, nil
}

// quoted embebe un literal de texto en el SQL. Solo se usa con las constantes
// de categoría de este mismo archivo (nunca con entrada del usuario): las
// ramas de la unión necesitan el literal en el SELECT y repetir un parámetro
// con nombre por rama enturbiaría la consulta.
func quoted(literal string) string {
	return "'" + strings.ReplaceAll(literal, "'", "''") + "'"
}

// GetTenantActivityPeople lista quién aparece en el expediente. Sale de la
// MISMA unión que la línea de tiempo (y no de la plantilla actual) para que el
// desplegable ofrezca exactamente a quien tiene algo que enseñar: incluye a
// quien ya causó baja y deja fuera a quien acaba de entrar y todavía no ha
// hecho nada.
func (r *adminRepository) GetTenantActivityPeople(tenantID uint) ([]TenantActivityPerson, error) {
	var people []TenantActivityPerson
	err := r.db.Raw(`
		WITH events AS (`+tenantEventsCTE+`)
		SELECT actor_id AS user_id, MAX(actor) AS name
		FROM events
		WHERE actor_id > 0 AND COALESCE(actor, '') <> ''
		GROUP BY actor_id
		ORDER BY LOWER(MAX(actor)) ASC
	`, sql.Named("tid", tenantID)).Scan(&people).Error
	return people, err
}

// GetTenantActivityCounts cuenta los movimientos por categoría con el filtro de
// persona ya aplicado, para que los contadores de los chips coincidan siempre
// con lo que se ve al pulsarlos.
func (r *adminRepository) GetTenantActivityCounts(tenantID uint, userID uint) ([]TenantActivityCount, error) {
	var counts []TenantActivityCount
	err := r.db.Raw(`
		WITH events AS (`+tenantEventsCTE+`)
		SELECT category, COUNT(*) AS count
		FROM events
		WHERE (@uid = 0 OR actor_id = @uid)
		GROUP BY category
	`, sql.Named("tid", tenantID), sql.Named("uid", userID)).Scan(&counts).Error
	return counts, err
}

// GetTenantPinnedNotes trae las notas fijadas, que van fuera de la cronología
// (arriba del expediente) porque son avisos vigentes, no historia.
func (r *adminRepository) GetTenantPinnedNotes(tenantID uint) ([]TenantActivity, error) {
	rows := []struct {
		User      string
		UserID    uint
		Details   string
		Timestamp time.Time
		EventID   uint
		EditedAt  *time.Time
	}{}
	err := r.db.Raw(`
		SELECT COALESCE(actor.name, '') AS "user", COALESCE(actor.id, 0) AS user_id,
			COALESCE(NULLIF(ce.detail, ''), 'Nota sin contenido') AS details,
			ce.created_at AS timestamp, ce.id AS event_id, ce.edited_at
		FROM company_events ce
		LEFT JOIN users actor ON actor.id = ce.by_user_id
		WHERE ce.company_id = ? AND ce.type = ? AND ce.pinned = true
		ORDER BY ce.created_at DESC
	`, tenantID, models.CompanyEventNote).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	notes := make([]TenantActivity, 0, len(rows))
	for _, row := range rows {
		notes = append(notes, TenantActivity{
			Type:      "company_" + models.CompanyEventNote,
			Category:  TenantActivityNote,
			User:      row.User,
			UserID:    row.UserID,
			Details:   row.Details,
			Timestamp: row.Timestamp,
			EventID:   row.EventID,
			Pinned:    true,
			EditedAt:  row.EditedAt,
		})
	}
	return notes, nil
}

func (r *adminRepository) DeleteCompanyNote(companyID, noteID uint) (int64, error) {
	res := r.db.Where("id = ? AND company_id = ? AND type = ?", noteID, companyID, models.CompanyEventNote).
		Delete(&models.CompanyEvent{})
	return res.RowsAffected, res.Error
}

// UpdateCompanyNote corrige el texto de una nota y deja constancia de que se
// editó. Acotado a la empresa y al tipo "note", igual que el borrado.
func (r *adminRepository) UpdateCompanyNote(companyID, noteID uint, detail string, editedAt time.Time) (int64, error) {
	res := r.db.Model(&models.CompanyEvent{}).
		Where("id = ? AND company_id = ? AND type = ?", noteID, companyID, models.CompanyEventNote).
		Updates(map[string]interface{}{"detail": detail, "edited_at": editedAt})
	return res.RowsAffected, res.Error
}

// SetCompanyNotePinned fija o desfija una nota. No toca edited_at: cambiar de
// sitio una nota no es reescribirla.
func (r *adminRepository) SetCompanyNotePinned(companyID, noteID uint, pinned bool) (int64, error) {
	res := r.db.Model(&models.CompanyEvent{}).
		Where("id = ? AND company_id = ? AND type = ?", noteID, companyID, models.CompanyEventNote).
		Update("pinned", pinned)
	return res.RowsAffected, res.Error
}
