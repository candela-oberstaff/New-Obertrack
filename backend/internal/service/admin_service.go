package service

import (
	"errors"
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type DashboardMetrics struct {
	TotalCompanies     int     `json:"total_companies"`
	TotalProfessionals int     `json:"total_professionals"`
	TotalManagers      int     `json:"total_managers"`
	TotalHoursWorked   float64 `json:"total_hours_worked"`
	ApprovedHours      float64 `json:"approved_hours"`
	PendingHours       float64 `json:"pending_hours"`
	TotalTasks         int     `json:"total_tasks"`
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
	GetStats() (map[string]interface{}, error)

	GetAllUsers(userType, isManager, isActive string, offset, limit int) ([]models.User, int64, error)
	CreateUser(req map[string]interface{}) (*models.User, error)
	UpdateUser(id uint, updates map[string]interface{}) (*models.User, error)
	DeleteUser(id uint) error
	ResetPassword(id uint, newPassword string) error

	CreateSuperAdmin(name, email, password string, force bool) (*models.User, error)
	ResetSuperAdmin(name, email, password string) (*models.User, error)
	MakeSuperAdmin(email string) (*models.User, error)
}

type adminService struct {
	repo repository.AdminRepository
}

func NewAdminService(repo repository.AdminRepository) AdminService {
	return &adminService{repo: repo}
}

func (s *adminService) GetDashboardMetrics() (*DashboardMetrics, error) {
	var m DashboardMetrics

	comp, _ := s.repo.CountUsersByType("empleador")
	m.TotalCompanies = int(comp)

	prof, _ := s.repo.CountUsersByType("profesional")
	m.TotalProfessionals = int(prof)

	man, _ := s.repo.CountManagers()
	m.TotalManagers = int(man)

	totalHours, _ := s.repo.GetTotalHoursWorked()
	m.TotalHoursWorked = totalHours

	appHours, _ := s.repo.GetApprovedHours()
	m.ApprovedHours = appHours
	m.PendingHours = totalHours - appHours

	tasks, _ := s.repo.CountTasks()
	m.TotalTasks = int(tasks)

	compTasks, _ := s.repo.CountCompletedTasks()
	m.CompletedTasks = int(compTasks)
	m.PendingTasks = m.TotalTasks - m.CompletedTasks

	actToday, _ := s.repo.CountActiveToday()
	m.ActiveToday = int(actToday)

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

func (s *adminService) GetStats() (map[string]interface{}, error) {
	totalUsersList, totalUsersCount, _ := s.repo.GetAllUsers("", "", "", 0, 1)
	_ = totalUsersList

	activeUsers, _ := s.repo.CountActiveUsers()
	comp, _ := s.repo.CountUsersByType("empleador")
	hours, _ := s.repo.GetTotalHoursMonth()

	tasks, _ := s.repo.CountTasks()
	compTasks, _ := s.repo.CountCompletedTasks()

	var cr float64
	if tasks > 0 {
		cr = float64(compTasks) / float64(tasks) * 100
	}

	return map[string]interface{}{
		"total_users":       totalUsersCount,
		"active_users":      activeUsers,
		"total_companies":   comp,
		"total_hours_month": hours,
		"completion_rate":   cr,
	}, nil
}

func (s *adminService) GetAllUsers(userType, isManager, isActive string, offset, limit int) ([]models.User, int64, error) {
	return s.repo.GetAllUsers(userType, isManager, isActive, offset, limit)
}

func (s *adminService) CreateUser(req map[string]interface{}) (*models.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req["password"].(string)), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}

	userType := req["user_type"].(string)
	if userType != "empleador" && userType != "profesional" && userType != "superadmin" {
		return nil, errors.New("Invalid user type")
	}

	user := &models.User{
		Name:         req["name"].(string),
		Email:        req["email"].(string),
		Password:     string(hashedPassword),
		UserType:     models.UserType(userType),
		IsSuperadmin: userType == "superadmin",
	}

	if val, ok := req["company_name"].(string); ok {
		user.CompanyName = val
	}
	if val, ok := req["job_title"].(string); ok {
		user.JobTitle = val
	}
	if val, ok := req["is_manager"].(bool); ok {
		user.IsManager = val
	}
	if val, ok := req["phone_number"].(string); ok {
		user.PhoneNumber = val
	}
	if val, ok := req["country"].(string); ok {
		user.Country = val
	}
	if val, ok := req["city"].(string); ok {
		user.City = val
	}
	if val, ok := req["empleador_id"].(uint); ok {
		uid := val
		user.EmpleadorID = &uid
	}
	if val, ok := req["manager_id"].(uint); ok {
		mid := val
		user.ManagerID = &mid
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) UpdateUser(id uint, updates map[string]interface{}) (*models.User, error) {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if name, ok := updates["name"].(string); ok {
		user.Name = name
	}
	if email, ok := updates["email"].(string); ok {
		user.Email = email
	}
	if jt, ok := updates["job_title"].(string); ok {
		user.JobTitle = jt
	}
	if pn, ok := updates["phone_number"].(string); ok {
		user.PhoneNumber = pn
	}
	if c, ok := updates["country"].(string); ok {
		user.Country = c
	}
	if c, ok := updates["city"].(string); ok {
		user.City = c
	}
	if act, ok := updates["is_active"].(bool); ok {
		user.IsActive = act
	}
	if isM, ok := updates["is_manager"].(bool); ok {
		user.IsManager = isM
	}
	if uType, ok := updates["user_type"].(string); ok {
		user.UserType = models.UserType(uType)
		user.IsSuperadmin = uType == "superadmin"
	}
	if eid, ok := updates["empleador_id"].(uint); ok {
		uid := eid
		user.EmpleadorID = &uid
	}
	if mid, ok := updates["manager_id"].(uint); ok {
		idm := mid
		user.ManagerID = &idm
	}

	if err := s.repo.UpdateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) DeleteUser(id uint) error {
	return s.repo.DeleteUserAllData(id)
}

func (s *adminService) ResetPassword(id uint, newPassword string) error {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return errors.New("User not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("Failed to hash password")
	}

	user.Password = string(hashedPassword)
	return s.repo.UpdateUser(user)
}

func (s *adminService) CreateSuperAdmin(name, email, password string, force bool) (*models.User, error) {
	count, _ := s.repo.CountUsersByType("superadmin")
	if count > 0 && !force {
		return nil, errors.New("Superadmin already exists. Use /api/seed/reset-superadmin to recreate.")
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

	if err := s.repo.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *adminService) ResetSuperAdmin(name, email, password string) (*models.User, error) {
	s.repo.DeleteSuperadmins()
	return s.CreateSuperAdmin(name, email, password, true)
}

func (s *adminService) MakeSuperAdmin(email string) (*models.User, error) {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return nil, errors.New("User not found with email")
	}

	user.IsSuperadmin = true
	user.UserType = "superadmin"

	if err := s.repo.UpdateUser(user); err != nil {
		return nil, err
	}

	log.Printf("User %s is now superadmin", email)
	return user, nil
}
