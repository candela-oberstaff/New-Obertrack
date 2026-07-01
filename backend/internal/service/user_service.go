package service

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type UserService interface {
	GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error)
	GetByID(id, requesterID, tenantID uint, isSuperadmin bool) (*models.User, error)
	Create(req map[string]interface{}) (*models.User, error)
	Update(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.User, error)
	Delete(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) error

	ToggleStatus(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	PromoteToManager(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, desired *bool) (*models.User, error)
	AssignToManager(professionalID, managerID, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	// ReassignTeam mueve TODOS los reportes activos de oldManagerID (en todas las
	// empresas) al nuevo manager, o los desasigna si newManagerID es nil. Devuelve
	// cuántas membresías se reasignaron.
	ReassignTeam(oldManagerID uint, newManagerID *uint, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (int64, error)

	GetEmployees(employerID uint) ([]models.User, error)
	GetMyTeam(userID uint) ([]models.User, error)

	ChangePassword(id uint, currentPassword, newPassword string) error
	GetByEmail(email string) (*models.User, error)
}

type userService struct {
	repo           repository.UserRepository
	employmentRepo repository.EmploymentRepository
}

func NewUserService(repo repository.UserRepository, employmentRepo repository.EmploymentRepository) UserService {
	return &userService{repo: repo, employmentRepo: employmentRepo}
}

func (s *userService) authorizeUserTenant(target *models.User, requesterID, tenantID uint, isSuperadmin bool, requireManage bool, role string, isManager bool) error {
	if isSuperadmin {
		return nil
	}
	if target == nil {
		return errors.New("User not found")
	}
	if target.ID == requesterID {
		return nil
	}
	if tenantID == 0 || tenantForUser(target) != tenantID {
		return errors.New("Access denied")
	}
	if requireManage && !(isEmployerRole(role) || isManager) {
		return errors.New("Access denied")
	}
	return nil
}

func (s *userService) authorizeAdminAction(target *models.User, tenantID uint, isSuperadmin bool, role string) error {
	if isSuperadmin {
		return nil
	}
	if target == nil {
		return errors.New("User not found")
	}
	if !isEmployerRole(role) {
		return errors.New("Access denied")
	}
	if tenantID == 0 || tenantForUser(target) != tenantID {
		return errors.New("Access denied")
	}
	return nil
}

func (s *userService) GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error) {
	return s.repo.GetAll(role, isManager, search, companyID, offset, limit)
}

func (s *userService) GetByID(id, requesterID, tenantID uint, isSuperadmin bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, false, "", false); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Create(req map[string]interface{}) (*models.User, error) {
	password, ok := req["password"].(string)
	if !ok {
		return nil, errors.New("Password is required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}

	userType, _ := req["user_type"].(string)

	user := &models.User{
		Name:         req["name"].(string),
		Email:        req["email"].(string),
		Password:     string(hashedPassword),
		UserType:     models.UserType(userType),
		IsSuperadmin: userType == "superadmin",
	}

	if companyName, ok := req["company_name"].(string); ok {
		user.CompanyName = companyName
	}
	if jobTitle, ok := req["job_title"].(string); ok {
		user.JobTitle = jobTitle
	}

	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) Update(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return nil, err
	}

	if id == requesterID && !isSuperadmin && role == string(models.UserTypeProfessional) {
		for k := range updates {
			if k != "avatar" {
				delete(updates, k)
			}
		}
	}

	// This validates fields manually like the original implementation
	if name, ok := updates["name"].(string); ok && name != "" {
		user.Name = name
	}
	if email, ok := updates["email"].(string); ok && email != "" {
		user.Email = email
	}
	if avatar, ok := updates["avatar"].(string); ok && avatar != "" {
		user.Avatar = avatar
	}
	if jt, ok := updates["job_title"].(string); ok && jt != "" {
		user.JobTitle = jt
	}
	if pn, ok := updates["phone_number"].(string); ok && pn != "" {
		user.PhoneNumber = pn
	}
	if country, ok := updates["country"].(string); ok && country != "" {
		user.Country = country
	}
	if state, ok := updates["state"].(string); ok && state != "" {
		user.State = state
	}
	if city, ok := updates["city"].(string); ok && city != "" {
		user.City = city
	}
	if location, ok := updates["location"].(string); ok && location != "" {
		user.Location = location
	}
	if idDoc, ok := updates["identity_document"].(string); ok && idDoc != "" {
		user.IdentityDocument = idDoc
	}

	if len(updates) > 0 {
		if err := s.repo.Update(user, updates); err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *userService) Delete(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return err
	}

	// No se puede eliminar un manager que aún tiene equipo a su cargo:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if user.IsManager {
		count, cerr := countManagerReports(s.repo, s.employmentRepo, id)
		if cerr != nil {
			return cerr // fail-closed
		}
		if count > 0 {
			return fmt.Errorf("No se puede eliminar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	return s.repo.Delete(id)
}

func (s *userService) ToggleStatus(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeAdminAction(user, tenantID, isSuperadmin, role); err != nil {
		return nil, err
	}

	// Si el manager va a quedar inactivo y aún tiene equipo, no se puede:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if user.IsActive && user.IsManager {
		count, cerr := countManagerReports(s.repo, s.employmentRepo, id)
		if cerr != nil {
			return nil, cerr // fail-closed
		}
		if count > 0 {
			return nil, fmt.Errorf("No se puede desactivar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	user.IsActive = !user.IsActive
	if !user.IsActive {
		user.TokenVersion++ // revoke sessions when suspending a user (audit A-04)
	}
	if err := s.repo.Save(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) PromoteToManager(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, desired *bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeAdminAction(user, tenantID, isSuperadmin, role); err != nil {
		return nil, err
	}

	var newVal bool
	if desired != nil {
		newVal = *desired
	} else {
		newVal = !user.IsManager // toggle de respaldo por compatibilidad
	}

	// Solo profesionales y customer success pueden ser manager.
	if newVal && user.UserType != models.UserTypeProfessional && user.UserType != models.UserTypeCustomerSuccess {
		return nil, errors.New("Manager inválido: solo profesionales o customer success pueden ser manager")
	}

	// No permitir quitar el rol de manager si todavía tiene equipo a su cargo:
	// dejaría a esos subordinados sin aprobador. Hay que reasignarlos primero.
	if !newVal && user.IsManager {
		count, cerr := countManagerReports(s.repo, s.employmentRepo, id)
		if cerr != nil {
			return nil, cerr // fail-closed
		}
		if count > 0 {
			return nil, fmt.Errorf("No se puede quitar el rol de manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	updates := map[string]interface{}{"is_manager": newVal}
	if err := s.repo.Update(user, updates); err != nil {
		return nil, err
	}

	user.IsManager = newVal
	return user, nil
}

func (s *userService) GetEmployees(employerID uint) ([]models.User, error) {
	_, err := s.repo.GetByID(employerID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	return s.repo.GetEmployees(employerID)
}

func (s *userService) AssignToManager(professionalID, managerID, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error) {
	professional, err := s.repo.GetByID(professionalID)
	if err != nil {
		return nil, errors.New("Professional not found")
	}
	if err := s.authorizeAdminAction(professional, tenantID, isSuperadmin, role); err != nil {
		return nil, err
	}

	if managerID == 0 {
		professional.ManagerID = nil
	} else {
		if managerID == professionalID {
			return nil, errors.New("Un profesional no puede ser su propio manager")
		}
		manager, err := s.repo.GetByID(managerID)
		if err != nil {
			return nil, errors.New("Manager not found")
		}
		if err := s.authorizeAdminAction(manager, tenantID, isSuperadmin, role); err != nil {
			return nil, err
		}
		companyID := uint(0)
		if professional.EmpleadorID != nil {
			companyID = *professional.EmpleadorID
		}
		if err := ensureValidManager(s.employmentRepo, manager, companyID); err != nil {
			return nil, err
		}
		professional.ManagerID = &managerID
	}

	if err := s.repo.Save(professional); err != nil {
		return nil, err
	}

	// Sincroniza el employment de la empresa que asigna (espejo per-empresa de
	// users.manager_id) para que la fuente de verdad por-empresa quede alineada.
	// Un empleador opera sobre su propio tenant; un superadmin no tiene tenant
	// (tenantID==0), así que se usa la empresa activa del propio profesional.
	syncCompanyID := tenantID
	if syncCompanyID == 0 && professional.EmpleadorID != nil {
		syncCompanyID = *professional.EmpleadorID
	}
	if syncCompanyID > 0 {
		if emp, err := s.employmentRepo.GetActive(professional.ID, syncCompanyID); err == nil && emp != nil {
			_ = s.employmentRepo.Update(emp, map[string]interface{}{"manager_id": professional.ManagerID})
			syncPrimaryManager(s.employmentRepo, emp.ID, professional.ManagerID)
		}
	}

	return professional, nil
}

func (s *userService) ReassignTeam(oldManagerID uint, newManagerID *uint, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (int64, error) {
	oldManager, err := s.repo.GetByID(oldManagerID)
	if err != nil {
		return 0, errors.New("User not found")
	}
	if err := s.authorizeAdminAction(oldManager, tenantID, isSuperadmin, role); err != nil {
		return 0, err
	}

	// Reasignación acotada a la empresa activa del manager: respeta el invariante
	// per-empresa (no mueve reportes de otras empresas hacia un manager que no
	// pertenece a ellas). Para managers multi-empresa se reasigna por empresa.
	companyID := uint(0)
	if oldManager.EmpleadorID != nil {
		companyID = *oldManager.EmpleadorID
	}
	if companyID == 0 {
		return 0, errors.New("El manager no tiene una empresa activa")
	}

	if newManagerID != nil {
		newManager, err := s.repo.GetByID(*newManagerID)
		if err != nil {
			return 0, errors.New("Manager inválido: manager no encontrado")
		}
		if err := ensureValidManager(s.employmentRepo, newManager, companyID); err != nil {
			return 0, err
		}
	}

	n, err := s.employmentRepo.ReassignManager(oldManagerID, newManagerID, companyID)
	if err != nil {
		return 0, err
	}
	if _, err := s.repo.ReassignManager(oldManagerID, newManagerID, companyID); err != nil {
		return 0, err
	}
	// Dual-write: mueve los vínculos principales del set (employment_managers)
	// del manager saliente al nuevo (o los quita si newManagerID es nil).
	old := oldManagerID
	_ = s.employmentRepo.ReassignManagerLinks(&old, newManagerID, companyID)
	return n, nil
}

func (s *userService) GetMyTeam(userID uint) ([]models.User, error) {
	user, err := s.repo.GetByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if !user.IsManager {
		return []models.User{}, nil
	}

	if MultiManagerReadsEnabled() {
		return s.repo.GetTeamViaLinks(userID)
	}
	return s.repo.GetTeam(userID)
}

func (s *userService) ChangePassword(id uint, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return errors.New("Current password is incorrect")
	}

	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("Failed to hash password")
	}

	user.Password = string(hashedPassword)
	user.TokenVersion++ // revoke existing sessions on password change (audit A-04)
	return s.repo.Save(user)
}
func (s *userService) GetByEmail(email string) (*models.User, error) {
	return s.repo.GetByEmail(email)
}
