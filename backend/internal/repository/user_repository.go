package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error)
	Count(role, isManager, isActive string, companyID uint) (int64, error)
	GetByID(id uint) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByResetToken(token string) (*models.User, error)
	Create(user *models.User) error
	Update(user *models.User, updates map[string]interface{}) error
	Delete(id uint) error
	GetEmployees(employerID uint) ([]models.User, error)
	GetTeam(managerID uint) ([]models.User, error)
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
	countQuery := r.db.Model(&models.User{})
	findQuery := r.db.Model(&models.User{})

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

func (r *userRepository) Save(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) ReassignManager(oldManagerID uint, newManagerID *uint, companyID uint) (int64, error) {
	result := r.db.Model(&models.User{}).
		Where("manager_id = ? AND empleador_id = ?", oldManagerID, companyID).
		Update("manager_id", newManagerID)
	return result.RowsAffected, result.Error
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
