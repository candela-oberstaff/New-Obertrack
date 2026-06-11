package models

import (
	"time"

	"gorm.io/gorm"
)

// Audiencia de un tutorial: controla qué tipo de usuario puede verlo.
const (
	TutorialAudienceAll          = "all"         // Visible para empresas y profesionales
	TutorialAudienceEmployer     = "empleador"   // Solo usuarios tipo empresa
	TutorialAudienceProfessional = "profesional" // Solo usuarios tipo profesional
)

func IsValidTutorialAudience(audience string) bool {
	return audience == TutorialAudienceAll ||
		audience == TutorialAudienceEmployer ||
		audience == TutorialAudienceProfessional
}

type Tutorial struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Title          string         `gorm:"size:255;not null" json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	GoogleDriveURL string         `gorm:"size:1000;not null" json:"google_drive_url"`
	IconName       string         `gorm:"size:50;not null;default:'PlayCircle'" json:"icon_name"`
	Category       string         `gorm:"size:80;default:'General';index" json:"category"`
	Audience       string         `gorm:"size:20;not null;default:'all';index" json:"audience"`
	DurationMin    int            `gorm:"default:0" json:"duration_min"`
	OrderIndex     int            `gorm:"default:0;index" json:"order_index"`
	IsActive       bool           `gorm:"default:true;index" json:"is_active"`
	CreatedBy      uint           `gorm:"not null;index" json:"created_by"`
	Creator        User           `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Tutorial) TableName() string {
	return "tutorials"
}

type TutorialView struct {
	TutorialID uint      `gorm:"primaryKey;autoIncrement:false" json:"tutorial_id"`
	UserID     uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	ViewedAt   time.Time `json:"viewed_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (TutorialView) TableName() string {
	return "tutorial_views"
}
