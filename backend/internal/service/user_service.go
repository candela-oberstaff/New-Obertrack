package service

import (
	"errors"
	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type UserService interface {
	GetAll(role, isManager string, offset, limit int) ([]models.User, int64, error)
	GetByID(id uint) (*models.User, error)
	Create(req map[string]interface{}) (*models.User, error)
	Update(id uint, updates map[string]interface{}) (*models.User, error)
	Delete(id uint) error
	
	ToggleStatus(id uint) (*models.User, error)
	PromoteToManager(id uint) (*models.User, error)
	AssignToManager(professionalID, managerID uint) (*models.User, error)
	
	GetEmployees(employerID uint) ([]models.User, error)
	GetMyTeam(userID uint) ([]models.User, error)
	
	ChangePassword(id uint, currentPassword, newPassword string) error
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) GetAll(role, isManager string, offset, limit int) ([]models.User, int64, error) {
	return s.repo.GetAll(role, isManager, offset, limit)
}

func (s *userService) GetByID(id uint) (*models.User, error) {
	return s.repo.GetByID(id)
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

func (s *userService) Update(id uint, updates map[string]interface{}) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
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

func (s *userService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *userService) ToggleStatus(id uint) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}

	user.IsActive = !user.IsActive
	if err := s.repo.Save(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) PromoteToManager(id uint) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
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

func (s *userService) AssignToManager(professionalID, managerID uint) (*models.User, error) {
	professional, err := s.repo.GetByID(professionalID)
	if err != nil {
		return nil, errors.New("Professional not found")
	}

	if managerID == 0 {
		professional.ManagerID = nil
	} else {
		manager, err := s.repo.GetByID(managerID)
		if err != nil {
			return nil, errors.New("Manager not found")
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

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("Failed to hash password")
	}

	user.Password = string(hashedPassword)
	return s.repo.Save(user)
}
