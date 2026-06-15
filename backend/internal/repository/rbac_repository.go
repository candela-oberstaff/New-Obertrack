package repository

// RBACRepository gestiona roles personalizados y grupos (equipos) por tenant.
// Todas las lecturas/escrituras están acotadas por tenant_id; la resolución del
// tenant del solicitante ocurre en el handler (superadmin elige empresa,
// empleador usa la suya).

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RBACRepository interface {
	// Roles
	ListRoles(tenantID uint) ([]models.Role, error)
	GetRole(id uint) (*models.Role, error)
	CreateRole(role *models.Role) error
	UpdateRole(role *models.Role, updates map[string]interface{}) error
	DeleteRole(id uint) error
	GetRoleUsers(roleID uint) ([]models.User, error)
	AssignRole(roleID, userID uint) error
	UnassignRole(roleID, userID uint) error
	GetUserRoles(userID, tenantID uint) ([]models.Role, error)
	GetUserGroups(userID, tenantID uint) ([]models.Group, error)

	// Groups
	ListGroups(tenantID uint) ([]models.Group, error)
	GetGroup(id uint) (*models.Group, error)
	CreateGroup(group *models.Group) error
	UpdateGroup(group *models.Group, updates map[string]interface{}) error
	DeleteGroup(id uint) error
	GetGroupMembers(groupID uint) ([]models.User, error)
	AddGroupMember(groupID, userID uint) error
	RemoveGroupMember(groupID, userID uint) error
}

type rbacRepository struct {
	db *gorm.DB
}

func NewRBACRepository(db *gorm.DB) RBACRepository {
	return &rbacRepository{db: db}
}

// ── Roles ────────────────────────────────────────────────────────────────────

func (r *rbacRepository) ListRoles(tenantID uint) ([]models.Role, error) {
	var roles []models.Role
	err := r.db.Model(&models.Role{}).
		Select("roles.*, (SELECT COUNT(*) FROM user_roles ur WHERE ur.role_id = roles.id) as user_count").
		Where("roles.tenant_id = ?", tenantID).
		Order("LOWER(roles.name) ASC").
		Find(&roles).Error
	return roles, err
}

func (r *rbacRepository) GetRole(id uint) (*models.Role, error) {
	var role models.Role
	if err := r.db.First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *rbacRepository) CreateRole(role *models.Role) error {
	return r.db.Create(role).Error
}

func (r *rbacRepository) UpdateRole(role *models.Role, updates map[string]interface{}) error {
	return r.db.Model(role).Updates(updates).Error
}

func (r *rbacRepository) DeleteRole(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&models.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Role{}, id).Error
	})
}

func (r *rbacRepository) GetRoleUsers(roleID uint) ([]models.User, error) {
	var users []models.User
	err := r.db.Model(&models.User{}).
		Joins("JOIN user_roles ur ON ur.user_id = users.id").
		Where("ur.role_id = ?", roleID).
		Order("LOWER(users.name) ASC").
		Find(&users).Error
	return users, err
}

func (r *rbacRepository) AssignRole(roleID, userID uint) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserRole{
		RoleID:    roleID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}).Error
}

func (r *rbacRepository) UnassignRole(roleID, userID uint) error {
	return r.db.Where("role_id = ? AND user_id = ?", roleID, userID).Delete(&models.UserRole{}).Error
}

// GetUserRoles lista los roles de un usuario. tenantID 0 = sin filtro de tenant
// (uso interno para permisos efectivos).
func (r *rbacRepository) GetUserRoles(userID, tenantID uint) ([]models.Role, error) {
	var roles []models.Role
	query := r.db.Model(&models.Role{}).
		Joins("JOIN user_roles ur ON ur.role_id = roles.id").
		Where("ur.user_id = ?", userID)
	if tenantID > 0 {
		query = query.Where("roles.tenant_id = ?", tenantID)
	}
	err := query.Order("LOWER(roles.name) ASC").Find(&roles).Error
	return roles, err
}

func (r *rbacRepository) GetUserGroups(userID, tenantID uint) ([]models.Group, error) {
	var groups []models.Group
	query := r.db.Model(&models.Group{}).
		Joins("JOIN group_members gm ON gm.group_id = groups.id").
		Where("gm.user_id = ?", userID)
	if tenantID > 0 {
		query = query.Where("groups.tenant_id = ?", tenantID)
	}
	err := query.Order("LOWER(groups.name) ASC").Find(&groups).Error
	return groups, err
}

// ── Groups ───────────────────────────────────────────────────────────────────

func (r *rbacRepository) ListGroups(tenantID uint) ([]models.Group, error) {
	var groups []models.Group
	err := r.db.Model(&models.Group{}).
		Select("groups.*, (SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = groups.id) as member_count").
		Where("groups.tenant_id = ?", tenantID).
		Order("LOWER(groups.name) ASC").
		Find(&groups).Error
	return groups, err
}

func (r *rbacRepository) GetGroup(id uint) (*models.Group, error) {
	var group models.Group
	if err := r.db.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *rbacRepository) CreateGroup(group *models.Group) error {
	return r.db.Create(group).Error
}

func (r *rbacRepository) UpdateGroup(group *models.Group, updates map[string]interface{}) error {
	return r.db.Model(group).Updates(updates).Error
}

func (r *rbacRepository) DeleteGroup(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Group{}, id).Error
	})
}

func (r *rbacRepository) GetGroupMembers(groupID uint) ([]models.User, error) {
	var users []models.User
	err := r.db.Model(&models.User{}).
		Joins("JOIN group_members gm ON gm.user_id = users.id").
		Where("gm.group_id = ?", groupID).
		Order("LOWER(users.name) ASC").
		Find(&users).Error
	return users, err
}

func (r *rbacRepository) AddGroupMember(groupID, userID uint) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.GroupMember{
		GroupID:   groupID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}).Error
}

func (r *rbacRepository) RemoveGroupMember(groupID, userID uint) error {
	return r.db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&models.GroupMember{}).Error
}
