package models

import (
	"time"

	"gorm.io/gorm"
)

type UserType string
type TaskStatus string
type TaskPriority string

const (
	UserTypeEmployer     UserType = "empleador"
	UserTypeProfessional UserType = "empleado"
	UserTypeSuperadmin   UserType = "empleador"

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
	UserType            UserType       `gorm:"column:tipo_usuario;type:varchar(20);not null;default:'profesional'" json:"user_type"`
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
	GoogleCalendarToken string         `gorm:"type:text" json:"-"`
	GoogleFormsToken    string         `gorm:"type:text" json:"-"`
	RememberToken       string         `gorm:"size:100" json:"-"`
	EmailVerifiedAt     *time.Time     `json:"email_verified_at,omitempty"`
	ManagerID           *uint          `gorm:"index" json:"manager_id,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}
