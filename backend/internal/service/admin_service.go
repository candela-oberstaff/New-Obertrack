package service

import (
	"errors"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
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
	CreateUser(req map[string]interface{}) (*models.User, error)
	UpdateUser(id uint, updates map[string]interface{}) (*models.User, error)
	DeleteUser(id uint) error
	ResetPassword(id uint, newPassword string) error

	GetTenants() ([]repository.TenantSummary, error)
	GetTenant(id uint) (*repository.TenantSummary, error)
	GetTenantEmployees(id uint) ([]repository.EmployeeSummary, error)
	GetTenantActivities(id uint) ([]repository.Activity, error)
	SetTenantStatus(id uint, active bool) (*models.User, error)
	CreateTenant(name, companyName, email, password string) (*models.User, error)
	AssignTenant(userID uint, companyName string) (*models.User, error)
	GetEmployeeTracking(userID uint) (map[string]interface{}, error)

	CreateSuperAdmin(name, email, password string, force bool) (*models.User, error)
	ResetSuperAdmin(name, email, password string) (*models.User, error)
	MakeSuperAdmin(email string) (*models.User, error)
}

type adminService struct {
	repo         repository.AdminRepository
	userRepo     repository.UserRepository
	taskRepo     repository.TaskRepository
	workHourRepo repository.WorkHourRepository
}

func NewAdminService(
	repo repository.AdminRepository,
	userRepo repository.UserRepository,
	taskRepo repository.TaskRepository,
	workHourRepo repository.WorkHourRepository,
) AdminService {
	return &adminService{
		repo:         repo,
		userRepo:     userRepo,
		taskRepo:     taskRepo,
		workHourRepo: workHourRepo,
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
	since := time.Now().AddDate(0, 0, -days)
	return s.repo.GetInactiveUsersList(since)
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
	if v, ok := req["city"].(string); ok {
		user.City = v
	}
	if v, ok := req["location"].(string); ok {
		user.Location = v
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

	// Solo profesionales y customer success pueden quedar vinculados a una empresa.
	targetType := user.UserType
	if t, ok := updates["user_type"].(string); ok && t != "" {
		targetType = models.UserType(t)
	}
	if targetType != models.UserTypeProfessional && targetType != models.UserTypeCustomerSuccess {
		delete(updates, "empleador_id")
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

	if err := s.userRepo.Update(user, updates); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) DeleteUser(id uint) error {
	return s.userRepo.Delete(id)
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

func (s *adminService) SetTenantStatus(id uint, active bool) (*models.User, error) {
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
