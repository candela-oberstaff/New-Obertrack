package models

import (
	"time"

	"gorm.io/gorm"
)

type EmergencyTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"size:255;not null" json:"title"`
	Subject   string         `gorm:"size:255;not null" json:"subject"`
	Body      string         `gorm:"type:text;not null" json:"body"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (EmergencyTemplate) TableName() string {
	return "emergency_templates"
}
