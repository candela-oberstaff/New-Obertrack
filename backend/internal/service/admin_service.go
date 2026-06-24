package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type DashboardMetrics struct {
	TotalUsers         int     `json:"total_users"`
	ActiveUsers        int     `json:"active_users"`
	TotalCompanies     int     `json:"total_companies"`
	TotalProfessionals int     `json:"total_professionals"`
	TotalManagers      int     `json:"total_managers"`
	TotalHoursWorked   float64 `json:"total_hours_worked"`
	ApprovedHours      float64 `json:"approved_hours"`
	PendingHours       float64 `json:"pending_hours"`
	TotalTasks         int     `json:"total_tasks"`
	TotalBoards        int     `json:"total_boards"`
	CompletedTasks     int     `json:"completed_tasks"`
	PendingTasks       int     `json:"pending_tasks"`
	ActiveToday        int     `json:"active_today"`
	InactiveWarning    int     `json:"inactive_warning"`
}

type AdminService interface {
	GetDashboardMetrics() (*DashboardMetrics, error)
	GetCompanies() ([]repository.CompanyMetric, error)
	GetInactiveUsers(days int) ([]repository.InactiveUser, error)
	GetRecentActivities() ([]repository.Activity, error)
	GetAbsenceReport(month, year int) (*repository.AbsenceReport, error)
	GetStats() (map[string]interface{}, error)

	GetAllUsers(userType, isManager, isActive, search string, offset, limit int) ([]models.User, int64, error)
	// GetManagerReports lista los profesionales a cargo de un manager.
	GetManagerReports(managerID uint) ([]models.User, error)
	// BulkAssignManager asigna (o desasigna si managerID es nil) un manager a
	// varios profesionales a la vez. Devuelve (asignados, omitidos).
	BulkAssignManager(professionalIDs []uint, managerID *uint) (int, int, error)
	CreateUser(req map[string]interface{}) (*models.User, error)
	UpdateUser(id uint, updates map[string]interface{}) (*models.User, error)
	UpdateUserScoped(id uint, updates map[string]interface{}, tenantID uint) (*models.User, error)
	DeleteUser(id uint) error
	// DeleteUserScoped elimina (soft delete) un usuario solo si pertenece a la
	// empresa indicada (tenantID). Aplica el mismo guard de orphans que
	// DeleteUser. Para uso del EMPLEADOR (auto-acotado a su empresa).
	DeleteUserScoped(id, tenantID uint) error
	ResetPassword(id uint, newPassword string) error
	ResetPasswordScoped(id uint, newPassword string, tenantID uint) error
	FindUserByEmail(email string) (*models.User, error)

	GetSeniorityRanking() ([]repository.SeniorityItem, error)
	GetLatestFollowUps(kind string) ([]repository.FollowUpInfo, error)
	CreateFollowUp(userID, createdBy uint, kind, status, note string) (*models.FollowUp, error)
	GetTenants() ([]repository.TenantSummary, error)
	GetTenant(id uint) (*repository.TenantSummary, error)
	GetTenantEmployees(id uint) ([]repository.EmployeeSummary, error)
	GetTenantActivities(id uint) ([]repository.Activity, error)
	// GetArchived lista profesionales archivados (bajas + desactivados).
	// tenantID=0 = global; si no, los de esa empresa.
	GetArchived(tenantID uint) ([]repository.ArchivedEntry, error)
	SetTenantStatus(id uint, active bool, byUserID uint) (*models.User, error)
	CreateTenant(name, companyName, email, password string) (*models.User, error)
	AssignTenant(userID uint, companyName string) (*models.User, error)
	GetEmployeeTracking(userID uint) (map[string]interface{}, error)

	CreateSuperAdmin(name, email, password string, force bool) (*models.User, error)
	ResetSuperAdmin(name, email, password string) (*models.User, error)
	MakeSuperAdmin(email string) (*models.User, error)
}

type adminService struct {
	repo           repository.AdminRepository
	userRepo       repository.UserRepository
	taskRepo       repository.TaskRepository
	workHourRepo   repository.WorkHourRepository
	employmentRepo repository.EmploymentRepository
}

func NewAdminService(
	repo repository.AdminRepository,
	userRepo repository.UserRepository,
	taskRepo repository.TaskRepository,
	workHourRepo repository.WorkHourRepository,
	employmentRepo repository.EmploymentRepository,
) AdminService {
	return &adminService{
		repo:           repo,
		userRepo:       userRepo,
		taskRepo:       taskRepo,
		workHourRepo:   workHourRepo,
		employmentRepo: employmentRepo,
	}
}

func (s *adminService) GetDashboardMetrics() (*DashboardMetrics, error) {
	var m DashboardMetrics

	totalUsers, _ := s.userRepo.Count("", "", "", 0)
	m.TotalUsers = int(totalUsers)

	activeUsers, _ := s.userRepo.Count("", "", "true", 0)
	m.ActiveUsers = int(activeUsers)

	_, comp, _ := s.userRepo.GetAll("empleador", "", "", 0, 0, 1)
	m.TotalCompanies = int(comp)

	_, prof, _ := s.userRepo.GetAll("profesional", "", "", 0, 0, 1)
	m.TotalProfessionals = int(prof)

	_, man, _ := s.userRepo.GetAll("", "true", "", 0, 0, 1)
	m.TotalManagers = int(man)

	summary, _ := s.workHourRepo.GetSummary(make(map[string]interface{}))
	m.TotalHoursWorked = summary["total_hours"]
	m.ApprovedHours = summary["approved_hours"]
	m.PendingHours = summary["pending_hours"]

	_, totalTasks, _ := s.taskRepo.FindAll(nil, 0, 1)
	m.TotalTasks = int(totalTasks)

	totalBoards, _ := s.repo.CountBoards()
	m.TotalBoards = int(totalBoards)

	// Since TaskRepo doesn't have a specific FilterByStatus yet, we'll keep using GetDB for specialized counts if necessary,
	// or we can add a method to TaskRepo later. For now, we use the existing GetAll with GetDB.

	compCount, _ := s.workHourRepo.CountActiveToday()
	m.ActiveToday = int(compCount)

	threeDaysAgo := time.Now().AddDate(0, 0, -3)
	inact, _ := s.repo.CountInactiveWarning(threeDaysAgo)
	m.InactiveWarning = int(inact)

	return &m, nil
}

func (s *adminService) GetCompanies() ([]repository.CompanyMetric, error) {
	return s.repo.GetCompaniesMetrics()
}

func (s *adminService) GetInactiveUsers(days int) ([]repository.InactiveUser, error) {
	// days se interpreta como días hábiles completos sin registrar horas.
	return s.repo.GetInactiveUsersList(days)
}

func (s *adminService) GetRecentActivities() ([]repository.Activity, error) {
	return s.repo.GetRecentActivities()
}

func (s *adminService) GetAbsenceReport(month, year int) (*repository.AbsenceReport, error) {
	now := time.Now()
	if month < 1 || month > 12 {
		month = int(now.Month())
	}
	if year < 2000 {
		year = now.Year()
	}

	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
	endDate := startDate.AddDate(0, 1, -1)
	return s.repo.GetAbsenceReport(startDate, endDate)
}

func (s *adminService) GetStats() (map[string]interface{}, error) {
	_, totalUsersCount, _ := s.userRepo.GetAll("", "", "", 0, 0, 1)
	_, activeUsers, _ := s.userRepo.GetAll("", "", "", 0, 0, 1) // Needs IsActive filter in UserRepository
	_, comp, _ := s.userRepo.GetAll("empleador", "", "", 0, 0, 1)
	hours, _ := s.workHourRepo.GetTotalHoursMonth()
	s.taskRepo.FindAll(nil, 0, 1)

	return map[string]interface{}{
		"total_users":       totalUsersCount,
		"active_users":      activeUsers,
		"total_companies":   comp,
		"total_hours_month": hours,
		"completion_rate":   0, // Simplified
	}, nil
}

func (s *adminService) GetAllUsers(userType, isManager, isActive, search string, offset, limit int) ([]models.User, int64, error) {
	return s.userRepo.GetAll(userType, isManager, search, 0, offset, limit)
}

func (s *adminService) GetManagerReports(managerID uint) ([]models.User, error) {
	if MultiManagerReadsEnabled() {
		return s.userRepo.GetReportsByManagerViaLinks(managerID)
	}
	return s.userRepo.GetReportsByManager(managerID)
}

func (s *adminService) BulkAssignManager(professionalIDs []uint, managerID *uint) (int, int, error) {
	if managerID != nil {
		manager, err := s.userRepo.GetByID(*managerID)
		if err != nil {
			return 0, 0, errors.New("Manager inválido: manager no encontrado")
		}
		if !manager.IsManager {
			return 0, 0, errors.New("Manager inválido: el usuario seleccionado no es manager")
		}
		if !manager.IsActive {
			return 0, 0, errors.New("Manager inválido: el manager seleccionado está inactivo")
		}
	}
	assigned, skipped := 0, 0
	for _, pid := range professionalIDs {
		prof, err := s.userRepo.GetByID(pid)
		if err != nil || prof.UserType != models.UserTypeProfessional {
			skipped++
			continue
		}
		// Defensa: en el lote no se asigna un manager a otro manager (evita ciclos).
		if managerID != nil && prof.IsManager {
			skipped++
			continue
		}
		companyID := uint(0)
		if prof.EmpleadorID != nil {
			companyID = *prof.EmpleadorID
		}
		if managerID != nil {
			// El manager debe pertenecer a la empresa del profesional y no ser él mismo.
			if companyID == 0 || *managerID == pid {
				skipped++
				continue
			}
			if _, err := s.employmentRepo.GetActive(*managerID, companyID); err != nil {
				skipped++
				continue
			}
		}
		if err := s.userRepo.Update(prof, map[string]interface{}{"manager_id": managerID}); err != nil {
			skipped++
			continue
		}
		// Espeja el employment activo de la empresa del profesional (per-empresa).
		if companyID > 0 {
			if emp, err := s.employmentRepo.GetActive(pid, companyID); err == nil && emp != nil {
				_ = s.employmentRepo.Update(emp, map[string]interface{}{"manager_id": managerID})
				syncPrimaryManager(s.employmentRepo, emp.ID, managerID)
			}
		}
		assigned++
	}
	return assigned, skipped, nil
}

func (s *adminService) CreateUser(req map[string]interface{}) (*models.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req["password"].(string)), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}

	userType := models.UserType(req["user_type"].(string))
	user := &models.User{
		Name:         req["name"].(string),
		Email:        req["email"].(string),
		Password:     string(hashedPassword),
		UserType:     userType,
		IsSuperadmin: userType == models.UserTypeSuperadmin,
		IsActive:     true,
	}
	if v, ok := req["company_name"].(string); ok {
		user.CompanyName = v
	}
	if v, ok := req["job_title"].(string); ok {
		user.JobTitle = v
	}
	if v, ok := req["phone_number"].(string); ok {
		user.PhoneNumber = v
	}
	if v, ok := req["country"].(string); ok {
		user.Country = v
	}
	if v, ok := req["state"].(string); ok {
		user.State = v
	}
	if v, ok := req["city"].(string); ok {
		user.City = v
	}
	if v, ok := req["location"].(string); ok {
		user.Location = v
	}
	if v, ok := req["industry"].(string); ok {
		user.Industry = v
	}
	if v, ok := req["is_manager"].(bool); ok {
		user.IsManager = v
	}

	// Solo profesionales y customer success pueden quedar vinculados a una empresa.
	if userType == models.UserTypeProfessional || userType == models.UserTypeCustomerSuccess {
		if v, ok := req["empleador_id"].(uint); ok && v > 0 {
			employer, err := s.userRepo.GetByID(v)
			if err != nil || employer.UserType != models.UserTypeEmployer {
				return nil, errors.New("La empresa seleccionada no es válida")
			}
			empID := v
			user.EmpleadorID = &empID
		}
	}
	if userType == models.UserTypeProfessional {
		if v, ok := req["manager_id"].(uint); ok && v > 0 {
			manager, merr := s.userRepo.GetByID(v)
			if merr != nil {
				return nil, errors.New("Manager inválido: manager no encontrado")
			}
			// Empresa del nuevo profesional: la que ya quedó fijada en el user,
			// si no, el empleador_id que trae la petición.
			companyID := uint(0)
			if user.EmpleadorID != nil {
				companyID = *user.EmpleadorID
			} else if e, ok := req["empleador_id"].(uint); ok {
				companyID = e
			}
			if err := ensureValidManager(s.employmentRepo, manager, companyID); err != nil {
				return nil, err
			}
			managerID := v
			user.ManagerID = &managerID
		}
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) UpdateUser(id uint, updates map[string]interface{}) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}

	// Solo profesionales y customer success pueden quedar vinculados a una
	// empresa; solo los profesionales tienen manager. Al cambiar a un rol que
	// no aplica, se limpia activamente el vínculo (no basta con no actualizarlo).
	targetType := user.UserType
	if t, ok := updates["user_type"].(string); ok && t != "" {
		targetType = models.UserType(t)
	}

	// Solo profesionales o customer success pueden ser manager.
	if v, ok := updates["is_manager"].(bool); ok && v &&
		targetType != models.UserTypeProfessional && targetType != models.UserTypeCustomerSuccess {
		return nil, errors.New("Manager inválido: solo profesionales o customer success pueden ser manager")
	}

	// No permitir quitar el rol de manager si todavía tiene equipo a su cargo:
	// dejaría a esos subordinados sin aprobador. Hay que reasignarlos primero.
	if v, ok := updates["is_manager"].(bool); ok && !v && user.IsManager {
		count, cerr := countManagerReports(s.userRepo, s.employmentRepo, id)
		if cerr != nil {
			return nil, cerr // fail-closed: ante error de DB no permitimos dejar huérfanos
		}
		if count > 0 {
			return nil, fmt.Errorf("No se puede quitar el rol de manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	// No permitir desactivar un manager que aún tiene equipo a su cargo:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if v, ok := updates["is_active"].(bool); ok && !v && user.IsManager {
		count, cerr := countManagerReports(s.userRepo, s.employmentRepo, id)
		if cerr != nil {
			return nil, cerr // fail-closed
		}
		if count > 0 {
			return nil, fmt.Errorf("No se puede desactivar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}
	if targetType != models.UserTypeProfessional && targetType != models.UserTypeCustomerSuccess {
		updates["empleador_id"] = nil
	}
	if targetType != models.UserTypeProfessional {
		updates["manager_id"] = nil
	}

	// Al cambiar el tipo de usuario hay que sincronizar el flag is_superadmin
	// e invalidar la sesión vigente (el rol viaja en el JWT), para que el
	// cambio tome efecto en el próximo login/refresh.
	if targetType != user.UserType {
		updates["is_superadmin"] = targetType == models.UserTypeSuperadmin
		updates["token_version"] = user.TokenVersion + 1
	}

	if newEmpIDVal, ok := updates["empleador_id"]; ok {
		var newEmpID uint
		switch v := newEmpIDVal.(type) {
		case uint:
			newEmpID = v
		case *uint:
			if v != nil {
				newEmpID = *v
			}
		case int:
			newEmpID = uint(v)
		case float64:
			newEmpID = uint(v)
		}
		if newEmpID > 0 && user.EmpleadorID != nil && *user.EmpleadorID != newEmpID {
			return nil, errors.New("Este usuario ya está vinculado a una empresa y no se puede cambiar a otra")
		}
		// When empleador_id changes, invalidate the user's existing JWT so the
		// next login/refresh picks up the new tenant_id in the token claims.
		if newEmpID > 0 && (user.EmpleadorID == nil || *user.EmpleadorID != newEmpID) {
			updates["token_version"] = user.TokenVersion + 1
		}
	}

	// Si se asigna un manager (manager_id > 0), validar que el destino sea apto:
	// que sea manager, esté activo y pertenezca a la empresa del profesional.
	if mgrIDVal, ok := updates["manager_id"]; ok {
		var mgrID uint
		switch v := mgrIDVal.(type) {
		case uint:
			mgrID = v
		case *uint:
			if v != nil {
				mgrID = *v
			}
		case int:
			mgrID = uint(v)
		case float64:
			mgrID = uint(v)
		}
		if mgrID > 0 {
			manager, merr := s.userRepo.GetByID(mgrID)
			if merr != nil {
				return nil, errors.New("Manager inválido: manager no encontrado")
			}
			// Empresa resultante del target: la que traiga el update si hay,
			// si no la actual del usuario.
			companyID := uint(0)
			if user.EmpleadorID != nil {
				companyID = *user.EmpleadorID
			}
			if newEmpIDVal, ok := updates["empleador_id"]; ok {
				switch v := newEmpIDVal.(type) {
				case uint:
					companyID = v
				case *uint:
					if v != nil {
						companyID = *v
					}
				case int:
					companyID = uint(v)
				case float64:
					companyID = uint(v)
				}
			}
			if err := ensureValidManager(s.employmentRepo, manager, companyID); err != nil {
				return nil, err
			}
		}
	}

	if err := s.userRepo.Update(user, updates); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) DeleteUser(id uint) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		// No se pudo cargar: conserva el comportamiento de borrado directo.
		return s.userRepo.Delete(id)
	}
	// No se puede eliminar un manager que aún tiene equipo a su cargo:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if user.IsManager {
		count, cerr := countManagerReports(s.userRepo, s.employmentRepo, id)
		if cerr != nil {
			return cerr // fail-closed
		}
		if count > 0 {
			return fmt.Errorf("No se puede eliminar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}
	return s.userRepo.Delete(id)
}

// DeleteUserScoped elimina un usuario solo si pertenece a la empresa del
// solicitante (tenantID). Reusa el mismo guard de orphans que DeleteUser.
func (s *adminService) DeleteUserScoped(id, tenantID uint) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}
	if tenantForUser(user) != tenantID {
		return errors.New("Access denied")
	}
	// No se puede eliminar un manager que aún tiene equipo a su cargo:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if user.IsManager {
		count, cerr := countManagerReports(s.userRepo, s.employmentRepo, id)
		if cerr != nil {
			return cerr // fail-closed
		}
		if count > 0 {
			return fmt.Errorf("No se puede eliminar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}
	return s.userRepo.Delete(id)
}

func (s *adminService) UpdateUserScoped(id uint, updates map[string]interface{}, tenantID uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if tenantForUser(user) != tenantID {
		return nil, errors.New("Access denied")
	}
	if user.UserType != models.UserTypeProfessional {
		return nil, errors.New("Access denied")
	}
	return s.UpdateUser(id, updates)
}

func (s *adminService) ResetPasswordScoped(id uint, newPassword string, tenantID uint) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}
	if tenantForUser(user) != tenantID {
		return errors.New("Access denied")
	}
	if user.UserType != models.UserTypeProfessional {
		return errors.New("Access denied")
	}
	return s.ResetPassword(id, newPassword)
}

func (s *adminService) FindUserByEmail(email string) (*models.User, error) {
	return s.userRepo.GetByEmail(email)
}

func (s *adminService) ResetPassword(id uint, newPassword string) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("Failed to hash password")
	}

	return s.userRepo.Update(user, map[string]interface{}{"password": string(hashedPassword)})
}

func (s *adminService) GetSeniorityRanking() ([]repository.SeniorityItem, error) {
	return s.repo.GetSeniorityRanking()
}

func (s *adminService) GetArchived(tenantID uint) ([]repository.ArchivedEntry, error) {
	return s.repo.GetArchived(tenantID)
}

func (s *adminService) GetLatestFollowUps(kind string) ([]repository.FollowUpInfo, error) {
	if !models.IsValidFollowUpKind(kind) {
		return nil, errors.New("Tipo de seguimiento inválido: usa 'inactivity' o 'absence'")
	}
	return s.repo.GetLatestFollowUps(kind)
}

func (s *adminService) CreateFollowUp(userID, createdBy uint, kind, status, note string) (*models.FollowUp, error) {
	if !models.IsValidFollowUpKind(kind) {
		return nil, errors.New("Tipo de seguimiento inválido: usa 'inactivity' o 'absence'")
	}
	if !models.IsValidFollowUpStatus(status) {
		return nil, errors.New("Estado inválido: usa 'contacted', 'justified' o 'escalated'")
	}
	if _, err := s.userRepo.GetByID(userID); err != nil {
		return nil, errors.New("Usuario no encontrado")
	}

	followUp := &models.FollowUp{
		UserID:    userID,
		Kind:      kind,
		Status:    status,
		Note:      utils.SanitizeHTML(note),
		CreatedBy: createdBy,
	}
	if err := s.repo.CreateFollowUp(followUp); err != nil {
		return nil, err
	}
	return followUp, nil
}

func (s *adminService) GetTenants() ([]repository.TenantSummary, error) {
	return s.repo.GetTenants()
}

func (s *adminService) GetTenant(id uint) (*repository.TenantSummary, error) {
	return s.repo.GetTenantByID(id)
}

func (s *adminService) GetTenantEmployees(id uint) ([]repository.EmployeeSummary, error) {
	return s.repo.GetTenantEmployees(id)
}

func (s *adminService) GetEmployeeTracking(userID uint) (map[string]interface{}, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("Employee not found")
	}

	summary, err := s.repo.GetEmployeeSummary(userID)
	if err != nil {
		return nil, err
	}

	workHours, err := s.repo.GetEmployeeWorkHours(userID, 60)
	if err != nil {
		return nil, err
	}

	tasks, err := s.repo.GetEmployeeTasks(userID, 60)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"user":       user,
		"summary":    summary,
		"work_hours": workHours,
		"tasks":      tasks,
	}, nil
}

func (s *adminService) GetTenantActivities(id uint) ([]repository.Activity, error) {
	return s.repo.GetTenantActivities(id)
}

func (s *adminService) SetTenantStatus(id uint, active bool, byUserID uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("Tenant not found")
	}
	if user.UserType != models.UserTypeEmployer {
		return nil, errors.New("El usuario indicado no es una empresa")
	}
	if err := s.userRepo.Update(user, map[string]interface{}{"is_active": active}); err != nil {
		return nil, err
	}
	// Registra el hito en el expediente de la empresa (best-effort).
	eventType := models.CompanyEventSuspended
	if active {
		eventType = models.CompanyEventReactivated
	}
	_ = s.repo.CreateCompanyEvent(&models.CompanyEvent{
		CompanyID: id,
		Type:      eventType,
		ByUserID:  byUserID,
	})
	return user, nil
}

func (s *adminService) AssignTenant(userID uint, companyName string) (*models.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("Usuario no encontrado")
	}
	if user.IsSuperadmin {
		return nil, errors.New("No se puede asignar un superadmin como responsable")
	}
	if user.UserType == models.UserTypeEmployer {
		return nil, errors.New("El usuario ya es responsable de una empresa")
	}

	updates := map[string]interface{}{
		"user_type":    models.UserTypeEmployer,
		"company_name": companyName,
		"is_active":    true,
		"is_manager":   false,
		"empleador_id": nil,
		"manager_id":   nil,
	}
	if err := s.userRepo.Update(user, updates); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) CreateTenant(name, companyName, email, password string) (*models.User, error) {
	if _, err := s.userRepo.GetByEmail(email); err == nil {
		return nil, errors.New("Email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}

	user := &models.User{
		Name:        name,
		Email:       email,
		Password:    string(hashedPassword),
		UserType:    models.UserTypeEmployer,
		CompanyName: companyName,
		IsActive:    true,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) CreateSuperAdmin(name, email, password string, force bool) (*models.User, error) {
	_, count, _ := s.userRepo.GetAll("superadmin", "", "", 0, 0, 1)
	if count > 0 && !force {
		return nil, errors.New("Superadmin already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}

	user := &models.User{
		Name:         name,
		Email:        email,
		Password:     string(hashedPassword),
		UserType:     "superadmin",
		IsSuperadmin: true,
		IsActive:     true,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) ResetSuperAdmin(name, email, password string) (*models.User, error) {
	// Specialized delete for superadmins still in AdminRepo
	s.repo.DeleteSuperadmins()
	return s.CreateSuperAdmin(name, email, password, true)
}

func (s *adminService) MakeSuperAdmin(email string) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if err := s.userRepo.Update(user, map[string]interface{}{
		"is_superadmin": true,
		"user_type":     "superadmin",
	}); err != nil {
		return nil, err
	}

	return user, nil
}
