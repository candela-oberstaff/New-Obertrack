package models

import (
	"time"

	"gorm.io/gorm"
)

type WorkType string

const (
	WorkTypeComplete WorkType = "complete"
	WorkTypeAbsence  WorkType = "absence"
)

type WorkHour struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UserID         uint           `gorm:"not null;index" json:"user_id"`
	User           User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	WorkDate       time.Time      `gorm:"type:date;not null;index" json:"work_date"`
	WorkType       WorkType       `gorm:"type:varchar(20);not null;default:'complete'" json:"work_type"`
	HoursWorked    float64        `gorm:"type:decimal(5,2);not null" json:"hours_worked"`
	Activities     string         `gorm:"type:text" json:"activities"`
	StartTime      *time.Time     `gorm:"type:time" json:"start_time,omitempty"`
	EndTime        *time.Time     `gorm:"type:time" json:"end_time,omitempty"`
	Approved       bool           `gorm:"default:false" json:"approved"`
	ApprovedBy     *uint          `gorm:"index" json:"approved_by,omitempty"`
	ApprovedAt     *time.Time     `json:"approved_at,omitempty"`
	ApprovedByUser User           `gorm:"foreignKey:ApprovedBy" json:"approved_by_user,omitempty"`
	Comments       string         `gorm:"type:text" json:"comments"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (WorkHour) TableName() string {
	return "work_hours"
}
