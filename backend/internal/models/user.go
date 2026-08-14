package models

import (
	"time"

	"gorm.io/gorm"
)

type UserType string
type TaskStatus string
type TaskPriority string

const (
	UserTypeEmployer     UserType = "empleador"
	UserTypeProfessional UserType = "profesional"
	UserTypeSuperadmin   UserType = "superadmin"
	// Analista de IT: soporte técnico de plataforma — acceso a Tools, Métricas
	// y Auditoría, sin gestión de usuarios/empresas ni datos operativos de clientes.
	UserTypeITAnalyst       UserType = "analista_it"
	UserTypeCustomerSuccess UserType = "customer_success"

	TaskStatusTodo      TaskStatus = "por_hacer"
	TaskStatusInProcess TaskStatus = "en_proceso"
	TaskStatusDone      TaskStatus = "finalizado"

	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

type User struct {
	ID        uint     `gorm:"primaryKey" json:"id"`
	Name      string   `gorm:"size:255;not null" json:"name"`
	Email     string   `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string   `gorm:"size:255;not null" json:"-"`
	Avatar    string   `gorm:"size:500" json:"avatar"`
	UserType  UserType `gorm:"type:varchar(20);not null;default:'profesional'" json:"user_type"`
	IsManager bool     `gorm:"default:false" json:"is_manager"`
	// IsSupervisor marca al manager que además tiene OTROS managers a su cargo.
	// Es un flag encima de is_manager, nunca solo: así todo lo que hoy pregunta
	// por is_manager (tareas, tableros, horas) sigue valiendo para un supervisor
	// sin tocarse. Lo propio del rol es el alcance —ve el árbol completo bajo su
	// mando y no solo sus reportes directos— y se resuelve aparte, recorriendo
	// employment_managers.
	IsSupervisor bool   `gorm:"default:false" json:"is_supervisor"`
	IsSuperadmin bool   `gorm:"default:false" json:"is_superadmin"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`
	EmpleadorID  *uint  `gorm:"index" json:"empleador_id,omitempty"`
	Empleador    *User  `gorm:"foreignKey:EmpleadorID" json:"-"`
	CompanyName  string `gorm:"size:255" json:"company_name"`
	Industry     string `gorm:"size:255" json:"industry"`
	// ClientSince es la fecha de alta REAL de la empresa como cliente. Solo
	// aplica a las cuentas empleador. Existe aparte de created_at porque ese es
	// cuándo se creó el registro en Obertrack, que no coincide con el alta
	// cuando la empresa se carga meses después de empezar a trabajar con
	// nosotros. Nula = no se ha corregido y se muestra created_at.
	ClientSince      *time.Time `gorm:"type:date" json:"client_since,omitempty"`
	JobTitle         string     `gorm:"size:255" json:"job_title"`
	PhoneNumber      string     `gorm:"size:50" json:"phone_number"`
	Country          string     `gorm:"size:100" json:"country"`
	State            string     `gorm:"size:100" json:"state"`
	City             string     `gorm:"size:100" json:"city"`
	Location         string     `gorm:"type:text" json:"location"`
	IdentityDocument string     `gorm:"size:500" json:"identity_document"`
	Address          string     `gorm:"type:text" json:"address"`
	ObersuiteID      string     `gorm:"size:64" json:"obersuite_id,omitempty"`
	RememberToken    string     `gorm:"size:100" json:"-"`
	EmailVerifiedAt  *time.Time `json:"email_verified_at,omitempty"`
	ManagerID        *uint      `gorm:"index" json:"manager_id,omitempty"`
	ResetToken       string     `gorm:"size:100;index" json:"-"`
	ResetTokenExpiry *time.Time `json:"-"`
	// TokenVersion is bumped to invalidate all previously issued access/refresh
	// tokens for this user (logout-all, password change, suspension) — audit A-04.
	TokenVersion int    `gorm:"not null;default:0" json:"-"`
	ZohoAgentID  string `gorm:"size:255" json:"zoho_agent_id"`
	// OnboardingStatus controla el portero de inducción en el login. Por defecto
	// 'not_required': toda cuenta existente (y las que no pasan por inducción)
	// entra con normalidad. Ver models/induction.go.
	OnboardingStatus string `gorm:"size:20;not null;default:'not_required';index" json:"onboarding_status"`
	// Permissions son los permisos efectivos por módulo derivados de los roles
	// asignados (campo transitorio: solo viaja en /auth/me y /auth/login).
	// Ausente = sin roles = comportamiento histórico del tipo de cuenta.
	Permissions map[string]string `gorm:"-" json:"permissions,omitempty"`
	// Companies son las empresas donde el profesional tiene un empleo ACTIVO
	// (campo transitorio para el switcher multi-empresa). La activa es la que
	// apunta empleador_id. Vacío/único = sin switcher.
	Companies []CompanyRef   `gorm:"-" json:"companies,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) CompanyDisplayName() string {
	if u == nil {
		return ""
	}
	if u.CompanyName != "" {
		return u.CompanyName
	}
	if u.Empleador != nil {
		if u.Empleador.CompanyName != "" {
			return u.Empleador.CompanyName
		}
		return u.Empleador.Name
	}
	return ""
}

// CompanyRef es una referencia ligera a una empresa (para el switcher).
type CompanyRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func TenantForUser(user *User) uint {
	if user == nil {
		return 0
	}
	if user.UserType == UserTypeEmployer {
		return user.ID
	}
	if user.EmpleadorID != nil {
		return *user.EmpleadorID
	}
	return 0
}
