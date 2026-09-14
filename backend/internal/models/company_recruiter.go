package models

import "time"

// CompanyRecruiter es el reclutador de Obersuite que lleva a una empresa.
//
// Es un dato SUYO: se asigna en Obersuite y aquí solo se enseña. Por eso vive
// en su tabla y no como columnas más de users: quien lo escribe es el bridge,
// no una pantalla nuestra, y el reclutador no es un usuario de Obertrack (ni
// tiene por qué serlo). El espejo de esto es assigned_cs_id: nuestro analista,
// que Obersuite lee y no escribe.
type CompanyRecruiter struct {
	// CompanyID es la clave: una empresa tiene como mucho un reclutador.
	CompanyID uint `gorm:"primaryKey" json:"company_id"`
	// ExternalID es cómo Obersuite conoce a esa persona. Es lo que usan para
	// filtrar y lo que permite distinguir "cambió de reclutador" de "corrigieron
	// el nombre del mismo".
	ExternalID string `gorm:"size:120;not null" json:"external_id"`
	Name       string `gorm:"size:255;not null" json:"name"`
	Email      string `gorm:"size:255" json:"email"`
	// AssignedAt es cuándo se asignó ESTA persona; se conserva si solo se
	// corrige el nombre o el correo del mismo reclutador.
	AssignedAt time.Time `json:"assigned_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (CompanyRecruiter) TableName() string {
	return "company_recruiters"
}
