package service

import (
	"errors"
	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type UserService interface {
	GetAll(role, isManager string, companyID uint, offset, limit int) ([]models.User, int64, error)
	GetByID(id, requesterID, tenantID uint, isSuperadmin bool) (*models.User, error)
	Create(req map[string]interface{}) (*models.User, error)
	Update(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.User, error)
	Delete(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) error

	ToggleStatus(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	PromoteToManager(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	AssignToManager(professionalID, managerID, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	
	GetEmployees(employerID uint) ([]models.User, error)
	GetMyTeam(userID uint) ([]models.User, error)
	
	ChangePassword(id uint, currentPassword, newPassword string) error
	GetByEmail(email string) (*models.User, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
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

func (s *userService) GetAll(role, isManager string, companyID uint, offset, limit int) ([]models.User, int64, error) {
	return s.repo.GetAll(role, isManager, companyID, offset, limit)
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
		Name:        req["name"].(string),
		Email:       req["email"].(string),
		Password:    string(hashedPassword),
		UserType:    models.UserType(userType),
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
	if city, ok := updates["city"].(string); ok && city != "" {
		user.City = city
	}
	if location, ok := updates["location"].(string); ok && location != "" {
		user.Location = location
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

	return s.repo.Delete(id)
}

func (s *userService) ToggleStatus(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return nil, err
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

func (s *userService) PromoteToManager(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{"is_manager": !user.IsManager}
	if err := s.repo.Update(user, updates); err != nil {
		return nil, err
	}
	
	user.IsManager = !user.IsManager
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
	if err := s.authorizeUserTenant(professional, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return nil, err
	}

	if managerID == 0 {
		professional.ManagerID = nil
	} else {
		manager, err := s.repo.GetByID(managerID)
		if err != nil {
			return nil, errors.New("Manager not found")
		}
		if err := s.authorizeUserTenant(manager, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
			return nil, err
		}
		if !manager.IsManager {
			return nil, errors.New("User is not a manager")
		}
		professional.ManagerID = &managerID
	}

	if err := s.repo.Save(professional); err != nil {
		return nil, err
	}

	return professional, nil
}

func (s *userService) GetMyTeam(userID uint) ([]models.User, error) {
	user, err := s.repo.GetByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if !user.IsManager {
		return []models.User{}, nil
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
