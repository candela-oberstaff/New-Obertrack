package models

import (
	"time"

	"gorm.io/gorm"
)

// Niveles de permiso por módulo para un rol.
const (
	PermissionNone = "none"
	PermissionView = "view"
	PermissionEdit = "edit"
)

func IsValidPermissionLevel(level string) bool {
	return level == PermissionNone || level == PermissionView || level == PermissionEdit
}

// Role es un rol personalizado de una empresa (tenant) con permisos por módulo.
// Permissions guarda un objeto JSON {"tasks":"edit","hours":"view",...}; la
// aplicación de estos permisos en cada módulo se conecta gradualmente.
type Role struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	TenantID    uint           `gorm:"not null;index" json:"tenant_id"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Permissions string         `gorm:"type:text;not null;default:'{}'" json:"permissions"`
	CreatedBy   uint           `gorm:"not null" json:"created_by"`
	// Columna calculada (COUNT de user_roles): read-only y fuera de migración,
	// para que GORM la escanee del SELECT sin crear una columna real.
	UserCount int64 `gorm:"-" json:"user_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Role) TableName() string {
	return "roles"
}

// UserRole asigna un rol a un usuario (un usuario puede tener varios roles).
type UserRole struct {
	UserID    uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	RoleID    uint      `gorm:"primaryKey;autoIncrement:false" json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (UserRole) TableName() string {
	return "user_roles"
}

// Group es un equipo de usuarios dentro de una empresa (tenant).
type Group struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	TenantID    uint           `gorm:"not null;index" json:"tenant_id"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedBy   uint           `gorm:"not null" json:"created_by"`
	// Columna calculada (COUNT de group_members): read-only y fuera de migración.
	MemberCount int64 `gorm:"-" json:"member_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Group) TableName() string {
	return "groups"
}

// GroupMember asigna un usuario a un grupo.
type GroupMember struct {
	GroupID   uint      `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	UserID    uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (GroupMember) TableName() string {
	return "group_members"
}
