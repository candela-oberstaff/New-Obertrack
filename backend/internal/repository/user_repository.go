package repository

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	GetObersuiteCompanies(updatedSince *time.Time) ([]ObersuiteCompanyRecord, error)
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
