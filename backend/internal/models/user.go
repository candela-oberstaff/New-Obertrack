package models

import (
	"time"

	"gorm.io/gorm"
)

type UserType string
type TaskStatus string
type TaskPriority string

const (
	UserTypeEmployer        UserType = "empleador"
	UserTypeProfessional    UserType = "profesional"
	UserTypeSuperadmin      UserType = "superadmin"
	UserTypeCustomerSuccess UserType = "customer_success"

	TaskStatusTodo      TaskStatus = "por_hacer"
	TaskStatusInProcess TaskStatus = "en_proceso"
	TaskStatusDone      TaskStatus = "finalizado"

	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

type User struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	Name                string         `gorm:"size:255;not null" json:"name"`
	Email               string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password            string         `gorm:"size:255;not null" json:"-"`
	Avatar              string         `gorm:"size:500" json:"avatar"`
	UserType            UserType       `gorm:"type:varchar(20);not null;default:'profesional'" json:"user_type"`
	IsManager           bool           `gorm:"default:false" json:"is_manager"`
	IsSuperadmin        bool           `gorm:"default:false" json:"is_superadmin"`
	IsActive            bool           `gorm:"default:true" json:"is_active"`
	EmpleadorID         *uint          `gorm:"index" json:"empleador_id,omitempty"`
	CompanyName         string         `gorm:"size:255" json:"company_name"`
	JobTitle            string         `gorm:"size:255" json:"job_title"`
	PhoneNumber         string         `gorm:"size:50" json:"phone_number"`
	Country             string         `gorm:"size:100" json:"country"`
	City                string         `gorm:"size:100" json:"city"`
	Location            string         `gorm:"type:text" json:"location"`
	RememberToken       string         `gorm:"size:100" json:"-"`
	EmailVerifiedAt     *time.Time     `json:"email_verified_at,omitempty"`
	ManagerID           *uint          `gorm:"index" json:"manager_id,omitempty"`
	ResetToken          string         `gorm:"size:100;index" json:"-"`
	ResetTokenExpiry    *time.Time     `json:"-"`
	// TokenVersion is bumped to invalidate all previously issued access/refresh
	// tokens for this user (logout-all, password change, suspension) — audit A-04.
	TokenVersion        int            `gorm:"not null;default:0" json:"-"`
	ZohoAgentID         string         `gorm:"size:255" json:"zoho_agent_id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}

func TenantForUser(user *User) uint {
	if user == nil {
		return 0
	}
	if user.UserType == UserTypeEmployer {
		return user.ID
	}
	if user.EmpleadorID != nil {
		return *user.EmpleadorID
	}
	return 0
}
