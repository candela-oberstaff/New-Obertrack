package models

import (
	"time"

	"gorm.io/gorm"
)

// Estado de la inducción de un usuario. Vive denormalizado en users para que el
// login pueda decidir sin joins.
//
// IMPORTANTE: el valor por defecto es OnboardingNotRequired. Todas las cuentas
// que ya existían (y las que no pasan por inducción: empresas, superadmin, CS)
// quedan en ese estado y entran con normalidad. Solo los profesionales que
// llegan por el puente de Obersuite nacen en OnboardingPending.
const (
	OnboardingNotRequired = "not_required" // No aplica: entra normal
	OnboardingPending     = "pending"      // Invitado, aún no aprueba: sin acceso
	OnboardingPassed      = "passed"       // Aprobó: acceso habilitado
	OnboardingBlocked     = "blocked"      // Agotó intentos: requiere contacto de soporte
)

// Estados de una invitación de inducción (espejo del estado del usuario).
const (
	InductionPending = "pending"
	InductionPassed  = "passed"
	InductionBlocked = "blocked"
)

// InductionConfig es la configuración GLOBAL de la inducción (fila única, id=1):
// qué video se muestra, qué cuestionario se aplica y con qué exigencia. Se
// modeló como tabla —y no como constantes— para que el panel de configuración
// pueda cambiarla sin desplegar.
type InductionConfig struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// TutorialID es el video de inducción (módulo Novedades/Tutoriales).
	TutorialID *uint `json:"tutorial_id,omitempty"`
	// SurveyID es el cuestionario con ponderación (módulo Encuestas).
	SurveyID *uint `json:"survey_id,omitempty"`
	// PassingScore es el mínimo aprobatorio en porcentaje (0-100).
	PassingScore int `gorm:"not null;default:70" json:"passing_score"`
	// MaxAttempts son los intentos permitidos antes de bloquear.
	MaxAttempts int `gorm:"not null;default:3" json:"max_attempts"`
	// InviteTTLDays es la vigencia del enlace de la landing, en días.
	InviteTTLDays int `gorm:"not null;default:30" json:"invite_ttl_days"`
	// IsActive apaga la inducción sin borrar la configuración. Si está apagada
	// (o incompleta), la contratación vuelve al flujo directo de acceso.
	IsActive  bool      `gorm:"not null;default:false" json:"is_active"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (InductionConfig) TableName() string {
	return "induction_configs"
}

// Ready indica si la inducción está configurada y encendida. Si no lo está, el
// alta de un profesional NO se bloquea: se le da acceso como antes.
func (c *InductionConfig) Ready() bool {
	return c != nil && c.IsActive && c.SurveyID != nil && *c.SurveyID > 0
}

// InductionInvite es la invitación personal de un profesional a la landing
// pública. El token es la única credencial para entrar (aún no tiene cuenta
// activa), por eso es aleatorio, único y con vencimiento.
type InductionInvite struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"not null;uniqueIndex" json:"user_id"`
	Token  string `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Status string `gorm:"size:20;not null;default:'pending';index" json:"status"`
	// Attempts consumidos y tope, congelado al invitar para que un cambio de
	// configuración no altere las reglas de una invitación ya emitida.
	Attempts     int     `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts  int     `gorm:"not null;default:3" json:"max_attempts"`
	PassingScore int     `gorm:"not null;default:70" json:"passing_score"`
	BestScore    float64 `gorm:"not null;default:0" json:"best_score"`
	// Contenido congelado de esta invitación.
	TutorialID *uint `json:"tutorial_id,omitempty"`
	SurveyID   *uint `json:"survey_id,omitempty"`

	ExpiresAt   time.Time      `json:"expires_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (InductionInvite) TableName() string {
	return "induction_invites"
}

// AttemptsLeft son los intentos que le quedan al profesional.
func (i *InductionInvite) AttemptsLeft() int {
	if i == nil {
		return 0
	}
	left := i.MaxAttempts - i.Attempts
	if left < 0 {
		return 0
	}
	return left
}

// InductionAttempt es el registro de un intento (aprobado o no). Sirve de
// evidencia para Soporte cuando tiene que contactar a alguien que no pasó.
type InductionAttempt struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	InviteID uint    `gorm:"not null;index" json:"invite_id"`
	UserID   uint    `gorm:"not null;index" json:"user_id"`
	Score    float64 `gorm:"not null;default:0" json:"score"`
	Passed   bool    `gorm:"not null;default:false" json:"passed"`
	// AnswersJSON guarda lo que respondió, para que Soporte pueda revisarlo.
	AnswersJSON string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

func (InductionAttempt) TableName() string {
	return "induction_attempts"
}
