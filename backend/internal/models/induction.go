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

// Estados de una invitación de inducción (espejo del estado del usuario). Los
// mismos valores describen cada bloque de la invitación.
const (
	InductionPending = "pending"
	InductionPassed  = "passed"
	InductionBlocked = "blocked"
)

// InductionConfig es el interruptor GLOBAL de la inducción (fila única, id=1):
// si está encendida y cuánto dura el enlace. Qué se ve y qué se pregunta ya no
// vive aquí sino en los programas (InductionProgram): una empresa puede tener
// una inducción distinta de otra, y cada programa se arma con bloques
// reutilizables.
//
// Se modeló como tabla —y no como constantes— para que el panel pueda cambiarla
// sin desplegar.
type InductionConfig struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// InviteTTLDays es la vigencia del enlace de la landing, en días.
	InviteTTLDays int `gorm:"not null;default:30" json:"invite_ttl_days"`
	// IsActive apaga la inducción sin borrar nada. Si está apagada (o no hay
	// un programa por defecto usable), la contratación vuelve al flujo directo
	// de acceso.
	IsActive  bool      `gorm:"not null;default:false" json:"is_active"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (InductionConfig) TableName() string {
	return "induction_configs"
}

// InductionBlock es la unidad reutilizable de la inducción: un video (de
// Novedades, opcional) más su propio cuestionario calificado. Vive en una
// biblioteca y un mismo bloque puede formar parte de varios programas: por eso
// es una tabla propia y no una fila dentro del programa.
type InductionBlock struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:160;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	// TutorialID es el video (módulo Novedades). Es opcional: un bloque puede
	// ser solo cuestionario.
	TutorialID *uint `gorm:"index" json:"tutorial_id,omitempty"`
	// SurveyID es el cuestionario calificado (módulo Encuestas, kind induction).
	SurveyID uint `gorm:"not null;index" json:"survey_id"`
	// PassingScore es el mínimo aprobatorio del bloque, en porcentaje. Nil = se
	// usa el del programa.
	PassingScore *int `json:"passing_score,omitempty"`
	// BadgeTitle, BadgeIcon y BadgeColor definen la insignia que se gana al
	// aprobar el bloque. Título vacío = el nombre del bloque.
	BadgeTitle string         `gorm:"size:160" json:"badge_title"`
	BadgeIcon  string         `gorm:"size:40;not null;default:'Award'" json:"badge_icon"`
	BadgeColor string         `gorm:"size:20;not null;default:'orchid'" json:"badge_color"`
	CreatedBy  uint           `gorm:"not null;index" json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Enriquecimientos de solo lectura para el panel. No son columnas: los
	// llena el repositorio al listar, con consultas aparte.
	TutorialTitle string   `gorm:"-" json:"tutorial_title,omitempty"`
	SurveyTitle   string   `gorm:"-" json:"survey_title,omitempty"`
	QuestionCount int      `gorm:"-" json:"question_count"`
	ProgramNames  []string `gorm:"-" json:"program_names"`
	// OrderIndex es la posición dentro de un programa. Solo tiene sentido
	// cuando el bloque viene cargado como parte de uno.
	OrderIndex int `gorm:"-" json:"order_index"`
}

func (InductionBlock) TableName() string {
	return "induction_blocks"
}

// EffectivePassingScore resuelve el mínimo del bloque: el propio si lo tiene,
// si no el del programa.
func (b *InductionBlock) EffectivePassingScore(programDefault int) int {
	if b != nil && b.PassingScore != nil {
		return *b.PassingScore
	}
	return programDefault
}

// InductionProgram es la secuencia ordenada de bloques que se le asigna a un
// profesional, con las reglas del portero. Hay varios porque la inducción de
// una empresa no tiene por qué ser la de otra; uno de ellos es el programa por
// defecto, que recibe quien no tiene empresa asignada a ninguno.
type InductionProgram struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:160;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	// DefaultPassingScore es el mínimo aprobatorio de los bloques que no traen
	// uno propio.
	DefaultPassingScore int `gorm:"not null;default:70" json:"default_passing_score"`
	// MaxAttempts son los intentos permitidos POR BLOQUE antes de bloquear.
	MaxAttempts int `gorm:"not null;default:3" json:"max_attempts"`
	// IsDefault marca el programa que reciben las empresas sin asignación. Un
	// índice parcial único garantiza que sea uno solo.
	IsDefault bool `gorm:"not null;default:false" json:"is_default"`
	// IsActive apaga el programa sin borrarlo. Un programa apagado no se
	// asigna: quien lo tenía cae al programa por defecto.
	IsActive bool `gorm:"not null;default:true" json:"is_active"`
	// BadgeTitle, BadgeIcon y BadgeColor definen la insignia que se gana al
	// completar el programa entero. Título vacío = el nombre del programa.
	BadgeTitle string         `gorm:"size:160" json:"badge_title"`
	BadgeIcon  string         `gorm:"size:40;not null;default:'Trophy'" json:"badge_icon"`
	BadgeColor string         `gorm:"size:20;not null;default:'gold'" json:"badge_color"`
	// CertificateTemplateID es el diseño del certificado que se emite al
	// completar el programa. Nil = el programa no certifica.
	CertificateTemplateID *uint          `gorm:"index" json:"certificate_template_id,omitempty"`
	CreatedBy             uint           `gorm:"not null;index" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Blocks son los bloques en orden. Los carga el repositorio.
	Blocks []InductionBlock `gorm:"-" json:"blocks"`
	// CompanyIDs son las empresas asignadas a este programa.
	CompanyIDs []uint `gorm:"-" json:"company_ids"`
	// BlockCount y CompanyCount sirven al listado, que no carga el detalle.
	BlockCount   int `gorm:"-" json:"block_count"`
	CompanyCount int `gorm:"-" json:"company_count"`
}

func (InductionProgram) TableName() string {
	return "induction_programs"
}

// Usable indica si el programa puede emitirse: encendido y con al menos un
// bloque. Es la condición que hace que la inducción tenga algo que mostrar.
func (p *InductionProgram) Usable() bool {
	return p != nil && p.IsActive && len(p.Blocks) > 0
}

// InductionProgramBlock es el orden de un bloque dentro de un programa.
type InductionProgramBlock struct {
	ProgramID  uint `gorm:"primaryKey;autoIncrement:false" json:"program_id"`
	BlockID    uint `gorm:"primaryKey;autoIncrement:false" json:"block_id"`
	OrderIndex int  `gorm:"not null;default:0" json:"order_index"`
}

func (InductionProgramBlock) TableName() string {
	return "induction_program_blocks"
}

// InductionProgramCompany asigna un programa a una empresa. La clave es la
// empresa: una empresa tiene a lo sumo un programa.
type InductionProgramCompany struct {
	CompanyID uint `gorm:"primaryKey;autoIncrement:false" json:"company_id"`
	ProgramID uint `gorm:"not null;index" json:"program_id"`
}

func (InductionProgramCompany) TableName() string {
	return "induction_program_companies"
}

// InductionInvite es la invitación personal de un profesional a la landing
// pública. El token es la única credencial para entrar (aún no tiene cuenta
// activa), por eso es aleatorio, único y con vencimiento.
//
// Las reglas y el contenido se CONGELAN al invitar en las filas de
// InductionInviteBlock: editar el programa después no altera una invitación
// ya emitida.
type InductionInvite struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Una persona acumula invitaciones con el tiempo (ingreso, capacitaciones);
	// un índice parcial en la migración garantiza UNA sola pendiente a la vez.
	UserID uint `gorm:"not null;index" json:"user_id"`
	Token  string `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Status string `gorm:"size:20;not null;default:'pending';index" json:"status"`
	// ProgramID es el programa con el que se emitió. Nil en invitaciones
	// anteriores a los programas (la migración las adopta al programa por
	// defecto, así que en la práctica siempre viene).
	ProgramID   *uint  `gorm:"index" json:"program_id,omitempty"`
	ProgramName string `gorm:"size:160" json:"program_name"`
	// MaxAttempts es el tope de intentos POR BLOQUE, congelado al invitar.
	MaxAttempts int `gorm:"not null;default:3" json:"max_attempts"`
	// GatesAccess indica si esta invitación BLOQUEA el acceso hasta aprobar.
	// Es el caso del ingreso desde Obersuite. Una capacitación enviada desde
	// Soporte a alguien que ya trabaja no lo saca de la plataforma: gana sus
	// insignias sin que el acceso se toque. Vive en la invitación, y no en el
	// programa, porque el mismo programa puede ser ingreso para uno y repaso
	// para otro.
	GatesAccess bool `gorm:"not null;default:true" json:"gates_access"`

	ExpiresAt   time.Time      `json:"expires_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (InductionInvite) TableName() string {
	return "induction_invites"
}

// InductionInviteBlock es un bloque de una invitación: el snapshot de lo que
// se le pidió al profesional (video, cuestionario, mínimo) y su progreso en
// él. Snapshot y progreso van juntos porque tienen el mismo grano y la misma
// vida: nacen al invitar y mueren con la invitación.
type InductionInviteBlock struct {
	InviteID   uint `gorm:"primaryKey;autoIncrement:false" json:"invite_id"`
	BlockID    uint `gorm:"primaryKey;autoIncrement:false" json:"block_id"`
	OrderIndex int  `gorm:"not null;default:0" json:"order_index"`

	// Snapshot del bloque al momento de invitar.
	Name         string `gorm:"size:160;not null" json:"name"`
	TutorialID   *uint  `json:"tutorial_id,omitempty"`
	SurveyID     uint   `gorm:"not null" json:"survey_id"`
	PassingScore int    `gorm:"not null;default:70" json:"passing_score"`

	// Progreso.
	Status      string     `gorm:"size:20;not null;default:'pending'" json:"status"`
	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	BestScore   float64    `gorm:"not null;default:0" json:"best_score"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (InductionInviteBlock) TableName() string {
	return "induction_invite_blocks"
}

// AttemptsLeft son los intentos que le quedan al profesional en este bloque.
func (b *InductionInviteBlock) AttemptsLeft(maxAttempts int) int {
	if b == nil {
		return 0
	}
	left := maxAttempts - b.Attempts
	if left < 0 {
		return 0
	}
	return left
}

// InductionAttempt es el registro de un intento (aprobado o no) sobre un
// bloque. Sirve de evidencia para Soporte cuando tiene que contactar a alguien
// que no pasó.
type InductionAttempt struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	InviteID uint    `gorm:"not null;index" json:"invite_id"`
	UserID   uint    `gorm:"not null;index" json:"user_id"`
	BlockID  uint    `gorm:"not null;default:0;index" json:"block_id"`
	Score    float64 `gorm:"not null;default:0" json:"score"`
	Passed   bool    `gorm:"not null;default:false" json:"passed"`
	// AnswersJSON guarda lo que respondió, para que Soporte pueda revisarlo.
	AnswersJSON string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`

	// BlockName es el nombre del bloque en el que se hizo el intento. Lo llena
	// el servicio desde el snapshot de la invitación.
	BlockName string `gorm:"-" json:"block_name,omitempty"`
}

func (InductionAttempt) TableName() string {
	return "induction_attempts"
}
