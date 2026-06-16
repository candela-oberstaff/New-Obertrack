package models

import (
	"time"

	"gorm.io/gorm"
)

// Estados de un empleo (membresía de un profesional en una empresa).
const (
	EmploymentActive = "active" // Vínculo vigente
	EmploymentEnded  = "ended"  // El profesional ya no trabaja en esa empresa
)

// Employment es la membresía de un profesional (o customer success) en una
// empresa (tenant) y, a la vez, el núcleo del expediente laboral en esa empresa.
//
// FASE 0: la tabla refleja (dual-write) lo que hoy vive en users.empleador_id.
// El sistema sigue leyendo empleador_id como "empresa activa"; esta tabla es la
// fuente de verdad de TODAS las membresías (incluidas las múltiples y las
// terminadas) y la base sobre la que se construye el expediente.
type Employment struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index:idx_employment_user_company" json:"user_id"`
	CompanyID   uint           `gorm:"not null;index:idx_employment_user_company" json:"company_id"`
	JobTitle    string         `gorm:"size:255" json:"job_title"`
	ManagerID   *uint          `gorm:"index" json:"manager_id,omitempty"`
	Status      string         `gorm:"size:20;not null;default:'active';index" json:"status"`
	StartedAt   time.Time      `json:"started_at"`
	StartReason string         `gorm:"type:text" json:"start_reason,omitempty"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
	EndReason   string         `gorm:"type:text" json:"end_reason,omitempty"`
	// EndSummary guarda un snapshot inmutable al terminar el empleo (horas,
	// tareas, antigüedad...). Se llena en la fase 3; nullable hasta entonces.
	EndSummary string         `gorm:"type:text" json:"end_summary,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Employment) TableName() string {
	return "employments"
}
