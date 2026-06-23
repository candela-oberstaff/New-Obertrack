package models

import (
	"time"

	"gorm.io/gorm"
)

// EmploymentManager es el vínculo N-a-N entre un empleo (employment) y sus
// managers. Reemplaza progresivamente al puntero único employments.manager_id.
//
// FASE 0-1: esta tabla guarda TODOS los managers de un empleo; el principal se
// marca con IsPrimary=true y se mantiene en espejo con employments.manager_id
// (dual-write). Las lecturas (aprobación, scope de horas, equipo, guards) aún
// usan el puntero, así que introducir esta tabla no cambia el comportamiento.
type EmploymentManager struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	EmploymentID uint           `gorm:"index;not null" json:"employment_id"`
	ManagerID    uint           `gorm:"index;not null" json:"manager_id"`
	IsPrimary    bool           `gorm:"default:false" json:"is_primary"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (EmploymentManager) TableName() string {
	return "employment_managers"
}
