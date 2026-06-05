package models

import (
	"time"

	"gorm.io/gorm"
)

type WorkType string

const (
	WorkTypeComplete WorkType = "complete"
	WorkTypeAbsence  WorkType = "absence"
	WorkTypeRecover  WorkType = "recover"
)

type WorkHour struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"not null;uniqueIndex:idx_user_date" json:"user_id"`
	TenantID        uint           `gorm:"index" json:"tenant_id"`
	User            User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	WorkDate        time.Time      `gorm:"type:date;not null;uniqueIndex:idx_user_date" json:"work_date"`
	WorkType        WorkType       `gorm:"type:varchar(20);not null;default:'complete'" json:"work_type"`
	HoursWorked     float64        `gorm:"type:decimal(5,2);not null" json:"hours_worked"`
	Activities      string         `gorm:"type:text" json:"activities"`
	StartTime       *time.Time     `gorm:"type:time" json:"start_time,omitempty"`
	EndTime         *time.Time     `gorm:"type:time" json:"end_time,omitempty"`
	Approved        bool           `gorm:"default:false;index:idx_user_approved" json:"approved"`
	ApprovedBy      *uint          `gorm:"index" json:"approved_by,omitempty"`
	ApprovedAt      *time.Time     `json:"approved_at,omitempty"`
	ApprovedByUser  User           `gorm:"foreignKey:ApprovedBy" json:"approved_by_user,omitempty"`
	Rejected        bool           `gorm:"default:false;index:idx_user_rejected" json:"rejected"`
	RejectedBy      *uint          `gorm:"index" json:"rejected_by,omitempty"`
	RejectedAt      *time.Time     `json:"rejected_at,omitempty"`
	RejectedByUser  User           `gorm:"foreignKey:RejectedBy" json:"rejected_by_user,omitempty"`
	RejectionReason string         `gorm:"type:text" json:"rejection_reason,omitempty"`
	Comments        string         `gorm:"type:text" json:"comments"`
	AbsenceReason   string         `gorm:"type:varchar(100)" json:"absence_reason,omitempty"`
	AbsenceHours    float64        `gorm:"type:decimal(5,2);default:0" json:"absence_hours,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (WorkHour) TableName() string {
	return "work_hours"
}

func (WorkHour) BeforeCreate(tx *gorm.DB) error {
	return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_work_hours_user_date ON work_hours (user_id, work_date)").Error
}
