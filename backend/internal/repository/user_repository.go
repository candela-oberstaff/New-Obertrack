package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	GetAll(role, isManager string, offset, limit int) ([]models.User, int64, error)
	GetByID(id uint) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByResetToken(token string) (*models.User, error)
	Create(user *models.User) error
	Update(user *models.User, updates map[string]interface{}) error
	Delete(id uint) error
	GetEmployees(employerID uint) ([]models.User, error)
	GetTeam(managerID uint) ([]models.User, error)
	Save(user *models.User) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetAll(role, isManager string, offset, limit int) ([]models.User, int64, error) {
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

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := findQuery.Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
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
	return r.db.Unscoped().Delete(&models.User{}, id).Error
}

func (r *userRepository) GetEmployees(employerID uint) ([]models.User, error) {
	var employees []models.User
	err := r.db.Where("empleador_id = ?", employerID).Find(&employees).Error
	return employees, err
}

func (r *userRepository) GetTeam(managerID uint) ([]models.User, error) {
	var team []models.User
	err := r.db.Where("manager_id = ?", managerID).Find(&team).Error
	return team, err
}

func (r *userRepository) Save(user *models.User) error {
	return r.db.Save(user).Error
}
