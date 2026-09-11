package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	GetObersuiteCompanies(updatedSince *time.Time) ([]ObersuiteCompanyRecord, error)
	GetObersuiteProfessionals(companyID uint) ([]ObersuiteProfessional, error)
	GetObersuiteProfessional(companyID, userID uint) (*ObersuiteProfessional, error)
	GetObersuiteWorkdays(userID, companyID uint, offset, limit int) ([]ObersuiteWorkday, int64, error)
	GetObersuiteTasks(userID, companyID uint, offset, limit int) ([]ObersuiteTask, int64, error)
	GetBoardPhases(boardIDs []uint) ([]BoardPhaseRow, error)
	GetObersuiteCounts(userID, companyID uint) (ObersuiteCounts, error)
	GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error)
	Count(role, isManager, isActive string, companyID uint) (int64, error)
	CountCompanies() (int64, error)
	GetByID(id uint) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	// FindAnyByEmail busca el correo SIN el filtro de borrado lógico.
	//
	// Existe para explicar el choque del índice único: users.email es un índice
	// único a secas, no parcial, así que una cuenta en la Papelera sigue
	// ocupando su correo. GetByEmail no la ve —GORM le añade
	// "deleted_at IS NULL"—, y sin esta consulta el panel solo podría decir
	// "ya existe" sin poder decir dónde ni qué hacer.
	FindAnyByEmail(email string) (*models.User, error)
	// FindActiveByPhoneDigits busca un usuario ACTIVO cuyo teléfono coincida con
	// los dígitos dados (se comparan los últimos 10, para tolerar prefijos de
	// país: WhatsApp entrega "58414..." y la ficha puede tener "0414...").
	// Sustenta la excepción de contacto en frío: escribirle primero a nuestra
	// propia gente no es cold outreach.
	FindActiveByPhoneDigits(digits string) (*models.User, error)
	GetByObersuiteID(obersuiteID string) (*models.User, error)
	GetByResetToken(token string) (*models.User, error)
	Create(user *models.User) error
	Update(user *models.User, updates map[string]interface{}) error
	Delete(id uint) error
	// RevokeSessionsByEmployer sube el token_version de todos los profesionales
	// de una empresa, lo que invalida sus sesiones vivas en el siguiente
	// request (el middleware compara la versión del JWT con la de la BD).
	// Se usa al suspender la empresa: el portero del login solo actúa en la
	// siguiente entrada, así que sin esto quien ya estaba dentro seguía
	// operando con normalidad. Devuelve cuántos usuarios se vieron afectados.
	RevokeSessionsByEmployer(employerID uint) (int64, error)
	GetEmployees(employerID uint) ([]models.User, error)
	GetTeam(managerID uint) ([]models.User, error)
	CountActiveSuperadminsExcluding(excludeID uint) (int64, error)
	// CountReportsByManager cuenta los usuarios activos que tienen a managerID
	// como manager (relación canónica users.manager_id, que escribe toda
	// asignación). Es la fuente principal para impedir degradar/eliminar a un
	// manager con equipo a su cargo, independiente de la sincronización de
	// employments (que depende del login del subordinado).
	CountReportsByManager(managerID uint) (int64, error)
	// GetReportsByManager lista los usuarios activos a cargo de managerID
	// (users.manager_id), para mostrar el equipo que hay que reasignar.
	GetReportsByManager(managerID uint) ([]models.User, error)

	// --- Lecturas via-links (FASE 2, semántica "cualquier manager") ---
	// GetTeamViaLinks lista los usuarios activos cuyo empleo ACTIVO en la empresa
	// activa del manager tiene un vínculo vivo a managerID en employment_managers.
	// Equivalente via-links de GetTeam.
	GetTeamViaLinks(managerID uint) ([]models.User, error)
	// CountReportsByManagerViaLinks cuenta los usuarios activos con un vínculo
	// vivo a managerID (cualquier empresa). Equivalente via-links de
	// CountReportsByManager.
	CountReportsByManagerViaLinks(managerID uint) (int64, error)
	// GetReportsByManagerViaLinks lista esos usuarios (orden por nombre).
	GetReportsByManagerViaLinks(managerID uint) ([]models.User, error)
	// GetByIDs lista los usuarios activos de la lista, ordenados por nombre. Lo
	// usa el alcance del supervisor, que resuelve su árbol como un conjunto de
	// IDs y después lo materializa.
	GetByIDs(ids []uint) ([]models.User, error)
	// ListActiveByTypes devuelve los usuarios ACTIVOS de los tipos dados. Lo usa
	// el público objetivo de las novedades, que necesita la ficha entera
	// (empresa, país, si tiene equipo a cargo) para decidir a quién alcanza.
	ListActiveByTypes(types []models.UserType) ([]models.User, error)
	// ListActiveSupervisors lista los supervisores activos con empresa asignada.
	// Lo recorre el watcher de escalado, que trabaja supervisor por supervisor.
	ListActiveSupervisors() ([]models.User, error)
	Save(user *models.User) error
	// ReassignManager mueve todos los usuarios que tienen a oldManagerID como
	// manager hacia newManagerID, o los desasigna si newManagerID es nil.
	// Devuelve cuántas filas se afectaron.
	ReassignManager(oldManagerID uint, newManagerID *uint, companyID uint) (int64, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	// Build two separate queries to avoid session pollution in GORM v2
	//
	// Las cuentas de sistema quedan fuera de TODO listado: no son personas, y
	// aparecían como seleccionables en el selector de destinatarios de campañas
	// (el bot es superadmin activo). Filtrarlo aquí lo saca de una vez de todas
	// las pantallas que se alimentan de esta consulta.
	countQuery := r.db.Model(&models.User{}).Where("is_system = false")
	findQuery := r.db.Model(&models.User{}).Where("is_system = false")

	if role != "" {
		countQuery = countQuery.Where("user_type = ?", role)
		findQuery = findQuery.Where("user_type = ?", role)
	}

	if isManager != "" {
		countQuery = countQuery.Where("is_manager = ?", isManager == "true")
		findQuery = findQuery.Where("is_manager = ?", isManager == "true")
	}

	if companyID > 0 {
		countQuery = countQuery.Where("empleador_id = ? OR id = ?", companyID, companyID)
		findQuery = findQuery.Where("empleador_id = ? OR id = ?", companyID, companyID)
	}

	if search != "" {
		like := "%" + search + "%"
		countQuery = countQuery.Where("name ILIKE ? OR email ILIKE ?", like, like)
		findQuery = findQuery.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := findQuery.Order("LOWER(name) ASC").Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

func (r *userRepository) Count(role, isManager, isActive string, companyID uint) (int64, error) {
	var total int64
	query := r.db.Model(&models.User{})

	if role != "" {
		query = query.Where("user_type = ?", role)
	}

	if isManager != "" {
		query = query.Where("is_manager = ?", isManager == "true")
	}

	if isActive != "" {
		query = query.Where("is_active = ?", isActive == "true")
	}

	if companyID > 0 {
		query = query.Where("empleador_id = ? OR id = ?", companyID, companyID)
	}

	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *userRepository) CountCompanies() (int64, error) {
	var total int64
	err := r.db.Model(&models.User{}).
		Where("user_type = ?", models.UserTypeEmployer).
		Where("COALESCE(TRIM(company_name), '') <> '' OR COALESCE(TRIM(name), '') <> ''").
		Count(&total).Error
	return total, err
}

func (r *userRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindAnyByEmail(email string) (*models.User, error) {
	var user models.User
	// Unscoped: incluye las filas con deleted_at, que son justo las que hay que
	// encontrar. LOWER a los dos lados porque el índice es sensible a mayúsculas
	// pero quien escribe el correo en el formulario no lo es.
	if err := r.db.Unscoped().Where("LOWER(email) = LOWER(?)", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindActiveByPhoneDigits(digits string) (*models.User, error) {
	var user models.User
	if err := r.db.
		Where(`is_active = ? AND right(regexp_replace(COALESCE(phone_number, ''), '\D', '', 'g'), 10) = right(?, 10)
			AND phone_number IS NOT NULL AND phone_number <> ''`, true, digits).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByObersuiteID(obersuiteID string) (*models.User, error) {
	var user models.User
	// El filtro de vacío es indispensable: sin él, un id ausente engancharía a
	// cualquiera de los usuarios que no vienen del puente.
	if err := r.db.Where("obersuite_id = ? AND obersuite_id != ''", obersuiteID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByResetToken(token string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("reset_token = ? AND reset_token != ''", token).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) Update(user *models.User, updates map[string]interface{}) error {
	return r.db.Model(user).Updates(updates).Error
}

// RevokeSessionsByEmployer usa UpdateColumn (no Update) a propósito: el bump de
// token_version es contabilidad de sesión, no un cambio de datos del perfil, y
// no debe mover el updated_at de media plantilla. El soft delete lo filtra GORM.
func (r *userRepository) RevokeSessionsByEmployer(employerID uint) (int64, error) {
	res := r.db.Model(&models.User{}).
		Where("empleador_id = ?", employerID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1"))
	return res.RowsAffected, res.Error
}

func (r *userRepository) Delete(id uint) error {
	// Soft delete: sets deleted_at and keeps the row so foreign keys
	// (work_hours, tickets, audit_logs, etc.) stay valid and history is
	// preserved. The user disappears from all normal queries.
	return r.db.Delete(&models.User{}, id).Error
}

func (r *userRepository) GetEmployees(employerID uint) ([]models.User, error) {
	var employees []models.User
	err := r.db.Where("empleador_id = ?", employerID).Find(&employees).Error
	return employees, err
}

func (r *userRepository) GetTeam(managerID uint) ([]models.User, error) {
	var team []models.User
	err := r.db.
		Joins("JOIN employments ON employments.user_id = users.id AND employments.status = ?", models.EmploymentActive).
		Where("employments.manager_id = ?", managerID).
		Where("employments.company_id = (SELECT empleador_id FROM users WHERE id = ?)", managerID).
		Where("users.is_active = ?", true).
		Find(&team).Error
	return team, err
}

// --- Lecturas via-links (FASE 2) ---

func (r *userRepository) GetTeamViaLinks(managerID uint) ([]models.User, error) {
	var team []models.User
	err := r.db.
		Joins("JOIN employments ON employments.user_id = users.id AND employments.status = ?", models.EmploymentActive).
		Joins("JOIN employment_managers ON employment_managers.employment_id = employments.id AND employment_managers.deleted_at IS NULL").
		Where("employment_managers.manager_id = ?", managerID).
		Where("employments.company_id = (SELECT empleador_id FROM users WHERE id = ?)", managerID).
		Where("users.is_active = ?", true).
		Distinct().
		Find(&team).Error
	return team, err
}

func (r *userRepository) CountReportsByManagerViaLinks(managerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Joins("JOIN employments ON employments.user_id = users.id AND employments.status = ?", models.EmploymentActive).
		Joins("JOIN employment_managers ON employment_managers.employment_id = employments.id AND employment_managers.deleted_at IS NULL").
		Where("employment_managers.manager_id = ?", managerID).
		Where("users.is_active = ?", true).
		Distinct("users.id").
		Count(&count).Error
	return count, err
}

func (r *userRepository) GetReportsByManagerViaLinks(managerID uint) ([]models.User, error) {
	var reports []models.User
	err := r.db.
		Joins("JOIN employments ON employments.user_id = users.id AND employments.status = ?", models.EmploymentActive).
		Joins("JOIN employment_managers ON employment_managers.employment_id = employments.id AND employment_managers.deleted_at IS NULL").
		Where("employment_managers.manager_id = ?", managerID).
		Where("users.is_active = ?", true).
		Distinct().
		Order("users.name ASC").
		Find(&reports).Error
	return reports, err
}

func (r *userRepository) GetByIDs(ids []uint) ([]models.User, error) {
	if len(ids) == 0 {
		return []models.User{}, nil
	}
	var users []models.User
	err := r.db.
		Where("id IN ? AND is_active = ?", ids, true).
		Order("name ASC").
		Find(&users).Error
	return users, err
}

func (r *userRepository) ListActiveSupervisors() ([]models.User, error) {
	var users []models.User
	err := r.db.
		Where("is_supervisor = ? AND is_active = ? AND empleador_id IS NOT NULL", true, true).
		Order("id ASC").
		Find(&users).Error
	return users, err
}

func (r *userRepository) Save(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) ReassignManager(oldManagerID uint, newManagerID *uint, companyID uint) (int64, error) {
	result := r.db.Model(&models.User{}).
		Where("manager_id = ? AND empleador_id = ?", oldManagerID, companyID).
		Update("manager_id", newManagerID)
	return result.RowsAffected, result.Error
}

func (r *userRepository) CountActiveSuperadminsExcluding(excludeID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Where("is_superadmin = ? AND is_active = ? AND id <> ?", true, true, excludeID).
		Count(&count).Error
	return count, err
}

func (r *userRepository) CountReportsByManager(managerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Where("manager_id = ? AND is_active = ?", managerID, true).
		Count(&count).Error
	return count, err
}

func (r *userRepository) GetReportsByManager(managerID uint) ([]models.User, error) {
	var reports []models.User
	err := r.db.
		Where("manager_id = ? AND is_active = ?", managerID, true).
		Order("name ASC").
		Find(&reports).Error
	return reports, err
}

// ListActiveByTypes lista los usuarios activos de los tipos indicados. Se
// seleccionan solo las columnas que necesita el público objetivo: traer la
// ficha completa de toda la plataforma para repartir un aviso sería caro.
func (r *userRepository) ListActiveByTypes(types []models.UserType) ([]models.User, error) {
	if len(types) == 0 {
		return []models.User{}, nil
	}
	raw := make([]string, 0, len(types))
	for _, t := range types {
		raw = append(raw, string(t))
	}
	var users []models.User
	if err := r.db.Model(&models.User{}).
		Select("id", "name", "email", "user_type", "empleador_id", "country", "is_manager", "is_supervisor").
		Where("is_active = ? AND user_type IN ?", true, raw).
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// ObersuiteCompanyRecord es la ficha completa de una empresa tal y como la
// consume Obersuite.
//
// Refleja los mismos campos que ve el equipo en el panel de Empresas
// (repository.TenantSummary) y los CALCULA IGUAL: los subselects se copian de
// tenantSelect a propósito. Si los dos sitios contaran distinto, el número que
// mira Customer Success y el que ve Obersuite se contradirían sin que nadie
// pudiera decir cuál está mal.
type ObersuiteCompanyRecord struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	IsActive         bool   `json:"is_active"`
	ResponsibleName  string `json:"responsible_name"`
	ResponsibleEmail string `json:"responsible_email"`
	Industry         string `json:"industry"`

	// Ubicación. Location es texto libre ("Las Lomas"); no sustituye a
	// country/state/city, los acompaña.
	Country string `json:"country"`
	State   string `json:"state"`
	City    string `json:"city"`
	Address string `json:"address"`

	ProfessionalsCount int `json:"professionals_count"`
	BoardsCount        int `json:"boards_count"`
	TasksCount         int `json:"tasks_count"`

	// Dos señales que NO se mezclan: cuándo contactamos nosotros, y cuándo dio
	// señales de vida la empresa. Una a la que llamamos ayer pero que no entra
	// hace dos meses es justo el caso que hay que ver, y un solo campo lo
	// escondería. Nulos cuando nunca ha pasado.
	LastContactAt *time.Time `json:"last_contact_at"`

	// UpdatedAt permite a Obersuite pedir solo lo que cambió en vez de
	// recorrer el padrón entero en cada sincronización.
	UpdatedAt time.Time `json:"updated_at"`
}

// GetObersuiteCompanies devuelve el padrón completo de empresas con su ficha.
//
// updatedSince acota a lo que cambió después de ese instante, para que Obersuite
// pueda sincronizar en incremental en vez de recorrer el padrón entero cada vez.
// Nulo = todas.
//
// El corte va como marcador posicional REPETIDO y no como parámetro con nombre.
// Con `@since::timestamptz`, GORM no sustituye el nombre y Postgres acaba
// leyendo `@` como el operador de valor absoluto sobre una columna `since` que
// no existe: la consulta entera revienta con 42703. Los sql.Named que sí
// funcionan en este repo van siempre sueltos, sin un cast pegado detrás.
//
// Los contadores se copian de tenantSelect en admin_repository.go, que es lo que
// ve el equipo en el panel de Empresas. Es duplicación consciente: si los dos
// sitios contaran distinto, el número del panel y el de Obersuite se
// contradirían sin forma de saber cuál está mal. Al tocar uno hay que tocar el
// otro.
//
// Lo que ya NO se calcula aquí son los campos de operación (horas del mes,
// jornadas pendientes, tickets abiertos) ni last_activity_at: nadie los consumía
// del otro lado y cambiaban cada pocos segundos, con lo que el ETag del handler
// no acertaba nunca. El de tickets además costaba una subconsulta con
// normalización de teléfonos. Ver docs/integracion-obersuite.md.
func (r *userRepository) GetObersuiteCompanies(updatedSince *time.Time) ([]ObersuiteCompanyRecord, error) {
	var records []ObersuiteCompanyRecord
	err := r.db.Raw(`
		SELECT
			u.id,
			COALESCE(NULLIF(u.company_name, ''), u.name) as name,
			u.is_active,
			u.name as responsible_name,
			u.email as responsible_email,
			COALESCE(u.industry, '') as industry,
			COALESCE(u.country, '') as country,
			COALESCE(u.state, '') as state,
			COALESCE(u.city, '') as city,
			COALESCE(u.address, '') as address,
			u.updated_at,
			-- MISMO criterio que GetObersuiteProfessionals, y tiene que
			-- seguir siéndolo: este número y aquella lista se pintan juntos
			-- en la ficha de Obersuite. Ya se separaron una vez —la lista
			-- filtraba solo por empresa principal— y devolvía menos gente que
			-- el contador de al lado, sin que fallara nada.
			(SELECT COUNT(DISTINCT p.id) FROM users p
			 WHERE (p.empleador_id = u.id OR EXISTS (SELECT 1 FROM employments e WHERE e.user_id = p.id AND e.company_id = u.id AND e.status = 'active' AND e.deleted_at IS NULL))
			   AND p.user_type = 'profesional' AND p.deleted_at IS NULL) as professionals_count,
			(SELECT COUNT(*) FROM boards b WHERE b.tenant_id = u.id AND b.deleted_at IS NULL) as boards_count,
			(SELECT COUNT(*) FROM tasks t WHERE t.tenant_id = u.id AND t.deleted_at IS NULL) as tasks_count,
			-- Última vez que NOSOTROS contactamos con la empresa.
			(SELECT MAX(ce.created_at) FROM company_events ce
				WHERE ce.company_id = u.id AND ce.type = 'contact') as last_contact_at
		FROM users u
		WHERE u.user_type = 'empleador' AND u.deleted_at IS NULL
		  AND (?::timestamptz IS NULL OR u.updated_at > ?::timestamptz)
		ORDER BY LOWER(COALESCE(NULLIF(u.company_name, ''), u.name)) ASC
	`, updatedSince, updatedSince).Scan(&records).Error
	return records, err
}

// ObersuiteProfessional es una persona de la plantilla de UNA empresa, con todo
// referido a ESA empresa y no a la que el usuario tenga como principal.
type ObersuiteProfessional struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
	// AvatarURL es la misma foto como URL absoluta y accesible sin sesión. La
	// relativa de arriba la pide el navegador de Obersuite a SU dominio y sale
	// rota; y /api/uploads exige sesión, así que tampoco se puede proxear.
	// Vacía si no hay foto. La rellena el handler, que es quien sabe el host.
	AvatarURL string `json:"avatar_url"`
	UserType  string `json:"user_type"`
	IsActive      bool   `json:"is_active"`
	IsReplacement bool   `json:"is_replacement"`
	IsManager     bool   `json:"is_manager"`
	IsSupervisor  bool   `json:"is_supervisor"`
	JobTitle     string `json:"job_title"`

	// Los dos identificadores de Obersuite NO significan lo mismo: ObersuiteID
	// es la persona (su candidato) y HireObersuiteID es ESTA contratación.
	// Alguien puede venir de Obersuite y que este empleo concreto lo abriéramos
	// nosotros a mano. Vacíos = no hay vínculo, y se omiten del JSON.
	ObersuiteID     string `json:"obersuite_id,omitempty"`
	HireObersuiteID string `json:"hire_obersuite_id,omitempty"`

	// StartedAt y el horario salen del empleo EN ESTA EMPRESA. Nulos o vacíos
	// cuando la persona la tiene como empresa principal pero sin empleo escrito.
	StartedAt         *time.Time `json:"started_at"`
	ScheduleType      string     `json:"schedule_type"`
	ScheduleDays      string     `json:"schedule_days"`
	ScheduleStartTime string     `json:"schedule_start_time"`
	ScheduleEndTime   string     `json:"schedule_end_time"`

	HoursThisMonth float64    `json:"hours_this_month"`
	TasksAssigned  int        `json:"tasks_assigned"`
	TasksCompleted int        `json:"tasks_completed"`
	LastActive     *time.Time `json:"last_active"`

	// IsPrimaryCompany dice si esta empresa es la que el usuario tiene activa.
	// Quien tiene empleo aquí pero otra como principal ve la otra al entrar en
	// la app: explica diferencias que si no parecen un fallo.
	IsPrimaryCompany bool `json:"is_primary_company"`

	// Lo que enseña nuestra ficha y Obersuite pinta en la suya.
	//
	// ManagerName va resuelto porque el nombre es lo único que se pinta;
	// mandar manager_id obligaría a pedir el organigrama para un solo dato.
	// Vacío si no tiene manager.
	ManagerName string `json:"manager_name"`
	// Location va ya compuesta ("Carabobo, Venezuela") con el mismo criterio de
	// la ficha: ciudad, provincia y país, los que haya; y si no hay ninguno,
	// el campo libre. Vacía si no se sabe nada.
	Location    string `json:"location"`
	PhoneNumber string `json:"phone_number"`
	// AccessState es la clave del portero de inducción (not_required, pending,
	// passed, blocked) y AccessLabel su texto en español, el mismo que pinta
	// nuestra ficha. Van los dos: la clave para colorear sin comparar textos, la
	// etiqueta para no repetir el diccionario del otro lado.
	AccessState   string `json:"access_state"`
	AccessLabel   string `json:"access_label"`
	EmailVerified bool   `json:"email_verified"`
}

// accessLabels es el diccionario del portero de inducción, el MISMO que
// ONBOARDING_STATE en frontend/src/pages/Tenants/EmployeeFicha.tsx. Al tocar
// uno hay que tocar el otro.
var accessLabels = map[string]string{
	"not_required": "Acceso directo",
	"pending":      "Inducción pendiente",
	"passed":       "Inducción aprobada",
	"blocked":      "Bloqueado por intentos",
}

// AccessLabelFor traduce la clave del portero a su etiqueta. Una clave que no
// se conoce se devuelve tal cual antes que en blanco: un texto raro se ve, un
// hueco no.
func AccessLabelFor(state string) string {
	if state == "" {
		state = "not_required"
	}
	if l, ok := accessLabels[state]; ok {
		return l
	}
	return state
}

// GetObersuiteProfessionals devuelve la plantilla de una empresa con el MISMO
// criterio que professionals_count del padrón: quien la tiene como empresa
// principal Y quien tiene un empleo activo en ella.
//
// Existe en vez de reutilizar GetTenantEmployees porque aquella filtra solo por
// empleador_id, y eso deja fuera a los recontratados: alguien que ya trabajaba
// en otra empresa conserva su empleador_id y aquí solo gana un empleo. El padrón
// sí los cuenta, así que la lista devolvía menos gente que el número de al lado
// —y son justo los que llegan por el puente, que es la vía que más los produce—.
//
// El empleo se busca lateralmente por la empresa PREGUNTADA. Con el criterio de
// employeeMetrics, que ancla a u.empleador_id, un recontratado habría salido con
// la fecha de ingreso y el horario de su OTRA empresa: un dato equivocado que
// parece bueno, que es la peor clase.
func (r *userRepository) GetObersuiteProfessionals(companyID uint) ([]ObersuiteProfessional, error) {
	return r.obersuiteProfessionals(companyID, 0)
}

// GetObersuiteProfessional es UNA fila de la plantilla: la cabecera de la ficha
// de la persona. Sale de la misma consulta que la lista a propósito, para que
// lo que se ve al abrir la ficha sea exactamente lo que se veía en la fila.
// Devuelve nil, nil si la persona no está vinculada a esa empresa.
func (r *userRepository) GetObersuiteProfessional(companyID, userID uint) (*ObersuiteProfessional, error) {
	rows, err := r.obersuiteProfessionals(companyID, userID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (r *userRepository) obersuiteProfessionals(companyID, onlyUserID uint) ([]ObersuiteProfessional, error) {
	var rows []ObersuiteProfessional
	err := r.db.Raw(`
		SELECT
			u.id, u.name, u.email,
			COALESCE(u.avatar, '') as avatar,
			u.user_type, u.is_active, COALESCE(u.is_replacement, false) as is_replacement, u.is_manager, u.is_supervisor,
			-- El cargo del empleo manda sobre el del perfil: es el que tiene en
			-- ESTA empresa, y el del perfil puede haberse quedado del anterior.
			COALESCE(NULLIF(e.job_title, ''), u.job_title, '') as job_title,
			COALESCE(u.obersuite_id, '') as obersuite_id,
			COALESCE(e.obersuite_id, '') as hire_obersuite_id,
			e.started_at,
			COALESCE(e.schedule_type, '')       as schedule_type,
			COALESCE(e.schedule_days, '')       as schedule_days,
			COALESCE(e.schedule_start_time, '') as schedule_start_time,
			COALESCE(e.schedule_end_time, '')   as schedule_end_time,
			COALESCE((SELECT SUM(wh.hours_worked) FROM work_hours wh
				WHERE wh.user_id = u.id AND wh.tenant_id = ? AND wh.deleted_at IS NULL
				  AND wh.work_date >= date_trunc('month', CURRENT_DATE)), 0) as hours_this_month,
			(SELECT COUNT(*) FROM task_users tu
				JOIN tasks t ON t.id = tu.task_id AND t.deleted_at IS NULL
				WHERE tu.user_id = u.id AND t.tenant_id = ?) as tasks_assigned,
			(SELECT COUNT(*) FROM task_users tu
				JOIN tasks t ON t.id = tu.task_id AND t.deleted_at IS NULL
				WHERE tu.user_id = u.id AND t.tenant_id = ? AND t.completed = true) as tasks_completed,
			(SELECT MAX(wh.work_date) FROM work_hours wh
				WHERE wh.user_id = u.id AND wh.tenant_id = ? AND wh.deleted_at IS NULL) as last_active,
			(u.empleador_id = ?) as is_primary_company,
			-- El manager del EMPLEO en esta empresa, y solo si no lo hay el del
			-- perfil. users.manager_id es de la persona: a un recontratado le
			-- saldría el jefe de su otra empresa, como ya pasó con el cargo.
			COALESCE(me.name, m.name, '') as manager_name,
			-- Ubicación compuesta como en la ficha: de lo concreto a lo general,
			-- y el texto libre solo cuando no hay nada estructurado.
			COALESCE(NULLIF(
				CONCAT_WS(', ', NULLIF(u.city, ''), NULLIF(u.state, ''), NULLIF(u.country, '')),
			''), u.location, '') as location,
			COALESCE(u.phone_number, '') as phone_number,
			COALESCE(NULLIF(u.onboarding_status, ''), 'not_required') as access_state,
			(u.email_verified_at IS NOT NULL) as email_verified
		FROM users u
		LEFT JOIN LATERAL (
			SELECT em.job_title, em.obersuite_id, em.started_at, em.manager_id,
			       em.schedule_type, em.schedule_days,
			       em.schedule_start_time, em.schedule_end_time
			FROM employments em
			WHERE em.user_id = u.id AND em.company_id = ?
			  AND em.status = 'active' AND em.deleted_at IS NULL
			ORDER BY em.started_at DESC
			LIMIT 1
		) e ON TRUE
		LEFT JOIN users me ON me.id = e.manager_id AND me.deleted_at IS NULL
		LEFT JOIN users m  ON m.id  = u.manager_id AND m.deleted_at  IS NULL
		WHERE u.deleted_at IS NULL
		  AND u.user_type = 'profesional'
		  AND (u.empleador_id = ? OR e.started_at IS NOT NULL)
		  AND (? = 0 OR u.id = ?)
		ORDER BY u.name
	`, companyID, companyID, companyID, companyID, companyID,
		companyID, companyID, onlyUserID, onlyUserID).Scan(&rows).Error
	for i := range rows {
		rows[i].AccessLabel = AccessLabelFor(rows[i].AccessState)
	}
	return rows, err
}

// ObersuiteWorkday es una jornada de la ficha de persona hacia Obersuite.
//
// El estado va resuelto en UNA clave (approved / rejected / pending) en vez de
// los dos booleanos de la tabla: con approved=false y rejected=false la ficha
// nuestra tuvo que aprender a distinguir "pendiente" de "rechazada", y ese
// aprendizaje no tiene por qué repetirse del otro lado.
type ObersuiteWorkday struct {
	ID              uint      `json:"id"`
	WorkDate        time.Time `json:"work_date"`
	WorkType        string    `json:"work_type"`
	HoursWorked     float64   `json:"hours_worked"`
	State           string    `json:"state"`
	Activities      string    `json:"activities"`
	Comments        string    `json:"comments"`
	RejectionReason string    `json:"rejection_reason"`
	AbsenceReason   string    `json:"absence_reason"`
	AbsenceHours    float64   `json:"absence_hours"`
}

// GetObersuiteWorkdays devuelve las jornadas de una persona EN UNA EMPRESA,
// paginadas y de la más reciente hacia atrás.
//
// Acotar por tenant_id no es opcional: work_hours lleva la empresa, y alguien
// con dos empleos tiene jornadas de las dos. Sin el filtro, la ficha de la
// empresa A enseñaría lo que la persona trabajó para la B.
func (r *userRepository) GetObersuiteWorkdays(userID, companyID uint, offset, limit int) ([]ObersuiteWorkday, int64, error) {
	var total int64
	if err := r.db.Raw(`
		SELECT COUNT(*) FROM work_hours
		WHERE user_id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, userID, companyID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ObersuiteWorkday
	err := r.db.Raw(`
		SELECT id, work_date, work_type, hours_worked,
			CASE WHEN approved THEN 'approved'
			     WHEN rejected THEN 'rejected'
			     ELSE 'pending' END as state,
			COALESCE(activities, '')       as activities,
			COALESCE(comments, '')         as comments,
			COALESCE(rejection_reason, '') as rejection_reason,
			COALESCE(absence_reason, '')   as absence_reason,
			COALESCE(absence_hours, 0)     as absence_hours
		FROM work_hours
		WHERE user_id = ? AND tenant_id = ? AND deleted_at IS NULL
		ORDER BY work_date DESC, id DESC
		LIMIT ? OFFSET ?
	`, userID, companyID, limit, offset).Scan(&rows).Error
	return rows, total, err
}

// ObersuiteTask es una tarea de la ficha de persona hacia Obersuite.
type ObersuiteTask struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	BoardID   uint   `json:"board_id"`
	BoardName string `json:"board_name"`
	Status    string `json:"status"`
	// StatusLabel es el nombre de la columna del tablero en la que está la
	// tarea. Un tablero puede tener columnas propias ("En revisión"), y
	// entonces status trae un id que no está en ningún diccionario fijo: la
	// etiqueta existe, pero en la definición del tablero. Se resuelve de ahí.
	// Vacía si la columna ya no existe en el tablero.
	StatusLabel string     `json:"status_label"`
	Completed   bool       `json:"completed"`
	EndDate     *time.Time `json:"end_date"`
}

// BoardPhaseRow es una fase (columna) de un tablero, lo justo para resolver la
// etiqueta de un status.
type BoardPhaseRow struct {
	BoardID uint
	Name    string
	Status  string
}

// GetBoardPhases devuelve las fases de varios tableros de una vez, para poner
// etiqueta a los status de una página de tareas sin una consulta por tarea.
func (r *userRepository) GetBoardPhases(boardIDs []uint) ([]BoardPhaseRow, error) {
	if len(boardIDs) == 0 {
		return nil, nil
	}
	var rows []BoardPhaseRow
	err := r.db.Raw(`
		SELECT bp.board_id, p.name, COALESCE(p.status, '') as status
		FROM board_phases bp
		JOIN phases p ON p.id = bp.phase_id
		WHERE bp.board_id IN ?
	`, boardIDs).Scan(&rows).Error
	return rows, err
}

// GetObersuiteTasks devuelve las tareas asignadas a una persona EN UNA EMPRESA,
// paginadas. Mismo motivo que las jornadas para acotar por tenant_id.
func (r *userRepository) GetObersuiteTasks(userID, companyID uint, offset, limit int) ([]ObersuiteTask, int64, error) {
	var total int64
	if err := r.db.Raw(`
		SELECT COUNT(*) FROM task_users tu
		JOIN tasks t ON t.id = tu.task_id AND t.deleted_at IS NULL
		WHERE tu.user_id = ? AND t.tenant_id = ?
	`, userID, companyID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ObersuiteTask
	err := r.db.Raw(`
		SELECT t.id, t.title, t.board_id,
			COALESCE(b.name, '') as board_name,
			t.status, t.completed, t.end_date
		FROM task_users tu
		JOIN tasks t ON t.id = tu.task_id AND t.deleted_at IS NULL
		LEFT JOIN boards b ON b.id = t.board_id AND b.deleted_at IS NULL
		WHERE tu.user_id = ? AND t.tenant_id = ?
		ORDER BY t.completed ASC, t.end_date ASC NULLS LAST, t.id DESC
		LIMIT ? OFFSET ?
	`, userID, companyID, limit, offset).Scan(&rows).Error
	return rows, total, err
}

// ObersuiteCounts son los tres números de las pestañas de la ficha de persona,
// para pintarlos al abrirla sin pedir los bloques enteros.
type ObersuiteCounts struct {
	Workdays int64 `json:"workdays"`
	Tasks    int64 `json:"tasks"`
}

// GetObersuiteCounts cuenta jornadas y tareas de la persona EN LA EMPRESA. Con
// los mismos filtros que las listas, o el número de la pestaña y el total de
// dentro no cuadrarían — que es exactamente lo que se acaba de arreglar en
// nuestra propia ficha.
func (r *userRepository) GetObersuiteCounts(userID, companyID uint) (ObersuiteCounts, error) {
	var c ObersuiteCounts
	err := r.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM work_hours
			  WHERE user_id = ? AND tenant_id = ? AND deleted_at IS NULL) as workdays,
			(SELECT COUNT(*) FROM task_users tu
			  JOIN tasks t ON t.id = tu.task_id AND t.deleted_at IS NULL
			  WHERE tu.user_id = ? AND t.tenant_id = ?) as tasks
	`, userID, companyID, userID, companyID).Scan(&c).Error
	return c, err
}
