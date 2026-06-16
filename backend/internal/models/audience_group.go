package models

import (
	"time"

	"gorm.io/gorm"
)

type AudienceGroup struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `json:"name" gorm:"not null"`
	Description string         `json:"description"`
	CreatedBy   uint           `json:"created_by"`
	Members     []User         `gorm:"many2many:audience_group_members;" json:"members"`
}

type AudienceGroupMember struct {
	AudienceGroupID uint `gorm:"primaryKey"`
	UserID          uint `gorm:"primaryKey"`
}

func (AudienceGroup) TableName() string {
	return "audience_groups"
}

func (AudienceGroupMember) TableName() string {
	return "audience_group_members"
}
