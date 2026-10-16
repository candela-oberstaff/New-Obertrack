package mcp

import (
	"fmt"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// Alcance por fila.
//
// Cada filtro de aquí replica la regla que aplica la web en su service (ver
// las referencias en cada función). El asistente nunca ve más que el usuario
// en pantalla; donde la web y una regla más estricta difieren, gana la
// estricta.

// Claves de rol para cuotas y permisos del MCP. No son user_type: manager y
// supervisor son flags sobre un profesional y tienen alcance propio.
const (
	RoleSuperadmin      = "superadmin"
	RoleCustomerSuccess = "customer_success"
	RoleIT              = "analista_it"
	RoleEmployer        = "empleador"
	RoleSupervisor      = "supervisor"
	RoleManager         = "manager"
	RoleProfessional    = "profesional"
)

// RoleOrder es el orden en que el panel muestra los roles.
var RoleOrder = []string{RoleSuperadmin, RoleCustomerSuccess, RoleIT, RoleEmployer, RoleSupervisor, RoleManager, RoleProfessional}

// RoleLabels en español para la UI y para whoami.
var RoleLabels = map[string]string{
	RoleSuperadmin:      "SuperAdmin",
	RoleCustomerSuccess: "Customer Success",
	RoleIT:              "Analista de IT",
	RoleEmployer:        "Empresa",
	RoleSupervisor:      "Supervisor",
	RoleManager:         "Manager",
	RoleProfessional:    "Profesional",
}

// RoleKey clasifica a un usuario en una de las claves de rol.
func RoleKey(u *models.User) string {
	switch {
	case u.IsSuperadmin || u.UserType == models.UserTypeSuperadmin:
		return RoleSuperadmin
	case string(u.UserType) == RoleCustomerSuccess:
		return RoleCustomerSuccess
	case string(u.UserType) == RoleIT:
		return RoleIT
	case u.UserType == models.UserTypeEmployer:
		return RoleEmployer
	case u.IsManager && u.IsSupervisor:
		return RoleSupervisor
	case u.IsManager:
		return RoleManager
	}
	return RoleProfessional
}

// Scope es lo que el usuario del token puede ver.
type Scope struct {
	UserID       uint
	Name         string
	Email        string
	RoleKey      string
	IsManager    bool
	CrossCompany bool // superadmin y CS: leen cualquier empresa eligiéndola
	TenantID     uint // empresa activa (0 si no tiene)
	TenantName   string

	// Permisos RBAC del tenant (módulo → none|view|edit). Solo acotan cuando
	// el usuario tiene roles asignados, igual que RequirePermission.
	Perms        map[string]string
	HasRBACRoles bool

	// Alcance de equipo de un manager: "tree" (supervisor, TeamIDs incluye al
	// propio usuario), "links" (multi-manager) o "direct".
	TeamMode string
	TeamIDs  []uint
}

// ScopeDeps son las piezas de la web que se reutilizan para calcular el
// alcance: así no hay una segunda copia de las reglas de equipo o de RBAC.
type ScopeDeps struct {
	UserRepo repository.UserRepository
	EmpRepo  repository.EmploymentRepository
	RBAC     service.RBACService
}

// BuildScope calcula el alcance de un usuario ya validado como activo.
func BuildScope(u *models.User, d ScopeDeps) (*Scope, error) {
	s := &Scope{
		UserID:    u.ID,
		Name:      u.Name,
		Email:     u.Email,
		RoleKey:   RoleKey(u),
		IsManager: u.IsManager,
		TenantID:  models.TenantForUser(u),
	}
	s.CrossCompany = s.RoleKey == RoleSuperadmin || s.RoleKey == RoleCustomerSuccess
	if s.CrossCompany || s.RoleKey == RoleIT {
		s.TenantID = 0
	}
	if s.TenantID > 0 && d.UserRepo != nil {
		if s.RoleKey == RoleEmployer {
			s.TenantName = companyLabel(u)
		} else if emp, err := d.UserRepo.GetByID(s.TenantID); err == nil && emp != nil {
			s.TenantName = companyLabel(emp)
		}
	}
	// RBAC: superadmin y empresa siempre pasan (handlers.RequirePermission).
	if !s.CrossCompany && s.RoleKey != RoleEmployer && s.TenantID > 0 && d.RBAC != nil {
		perms, has, err := d.RBAC.EffectivePermissions(u.ID, s.TenantID)
		if err != nil {
			return nil, err
		}
		s.Perms, s.HasRBACRoles = perms, has
	}
	if u.IsManager && s.TenantID > 0 && d.UserRepo != nil && d.EmpRepo != nil {
		mode, ids, err := service.HoursTeamScope(d.UserRepo, d.EmpRepo, u.ID, s.TenantID, true)
		if err != nil {
			return nil, err
		}
		s.TeamMode, s.TeamIDs = mode, ids
	}
	return s, nil
}

func companyLabel(u *models.User) string {
	if strings.TrimSpace(u.CompanyName) != "" {
		return u.CompanyName
	}
	return u.Name
}

// CanModule replica RequirePermission(module, view): sin roles asignados
// pasa (comportamiento histórico); con roles, hace falta al menos "view".
func (s *Scope) CanModule(module string) bool {
	if s.CrossCompany || s.RoleKey == RoleEmployer || !s.HasRBACRoles {
		return true
	}
	lvl := s.Perms[module]
	return lvl == models.PermissionView || lvl == models.PermissionEdit
}

// Describe es el alcance en palabras, para whoami y la página de
// consentimiento.
func (s *Scope) Describe() string {
	switch s.RoleKey {
	case RoleSuperadmin:
		return "Todas las empresas de la plataforma (eligiendo una empresa en horas, tareas y tableros), tickets e incidentes."
	case RoleCustomerSuccess:
		return "Todas las empresas de la plataforma (eligiendo una empresa en horas, tareas y tableros) e incidentes."
	case RoleIT:
		return "Solo totales agregados de la plataforma, sin datos personales."
	case RoleEmployer:
		return fmt.Sprintf("Tu empresa (%s): su equipo, sus jornadas, tableros y tareas.", s.tenantText())
	case RoleSupervisor, RoleManager:
		team := "tus reportes directos"
		if s.TeamMode == "tree" {
			team = "todo tu árbol de equipo"
		}
		return fmt.Sprintf("En %s: tus jornadas y las de %s, y todas las tareas de la empresa.", s.tenantText(), team)
	}
	return fmt.Sprintf("Tus propias jornadas, y las tareas de los tableros de %s a los que perteneces.", s.tenantText())
}

func (s *Scope) tenantText() string {
	if s.TenantName != "" {
		return s.TenantName
	}
	return "tu empresa"
}

// UserError es un error cuyo texto se puede mostrar tal cual al asistente:
// explica qué falta o qué no se puede, sin filtrar SQL ni detalles internos.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

func userErr(format string, a ...any) error { return &UserError{Msg: fmt.Sprintf(format, a...)} }

// ResolveCompany decide sobre qué empresa consulta un tool de empresa
// (tareas, tableros, horas de equipo). Igual que la web: quien lee a nivel
// plataforma tiene que elegir una; el resto queda atado a la suya.
func (s *Scope) ResolveCompany(companyID uint) (uint, error) {
	if s.CrossCompany {
		if companyID == 0 {
			return 0, userErr("Indica company_id: con tu rol hay que elegir una empresa (usa list_companies para ver los ids).")
		}
		return companyID, nil
	}
	if s.TenantID == 0 {
		return 0, userErr("Tu usuario no tiene una empresa activa, así que no hay datos de empresa que consultar.")
	}
	if companyID != 0 && companyID != s.TenantID {
		return 0, userErr("Solo puedes consultar tu propia empresa (company_id %d).", s.TenantID)
	}
	return s.TenantID, nil
}

// where acumula condiciones SQL con sus parámetros.
type where struct {
	conds []string
	args  []any
}

func (w *where) add(cond string, args ...any) {
	w.conds = append(w.conds, cond)
	w.args = append(w.args, args...)
}

// SQL devuelve las condiciones unidas con AND ("TRUE" si no hay ninguna).
func (w *where) SQL() string {
	if len(w.conds) == 0 {
		return "TRUE"
	}
	return "(" + strings.Join(w.conds, ") AND (") + ")"
}

// teamCond acota una columna de usuario al equipo del manager, como
// workHourService.GetAll: árbol, vínculos multi-manager o reportes directos,
// siempre incluyéndose a sí mismo.
func (s *Scope) teamCond(w *where, userCol, tenantCol string) {
	switch s.TeamMode {
	case "tree":
		if len(s.TeamIDs) == 0 {
			w.add("1 = 0")
			return
		}
		w.add(userCol+" IN ?", s.TeamIDs)
	case "links":
		w.add(userCol+` = ? OR EXISTS (
			SELECT 1 FROM employments e_ml
			JOIN employment_managers em_ml ON em_ml.employment_id = e_ml.id AND em_ml.deleted_at IS NULL
			WHERE e_ml.user_id = `+userCol+` AND e_ml.company_id = `+tenantCol+`
			  AND e_ml.status = 'active' AND e_ml.deleted_at IS NULL AND em_ml.manager_id = ?)`, s.UserID, s.UserID)
	default:
		w.add(userCol+` = ? OR EXISTS (
			SELECT 1 FROM employments e_md
			WHERE e_md.user_id = `+userCol+` AND e_md.company_id = `+tenantCol+`
			  AND e_md.status = 'active' AND e_md.deleted_at IS NULL AND e_md.manager_id = ?)`, s.UserID, s.UserID)
	}
}

// HoursFilter replica workHourService.GetAll sobre work_hours (alias wh).
func (s *Scope) HoursFilter(companyID uint) (*where, error) {
	w := &where{}
	w.add("wh.deleted_at IS NULL")
	switch {
	case s.RoleKey == RoleIT:
		return nil, userErr("Tu rol no tiene acceso a jornadas.")
	case s.CrossCompany:
		c, err := s.ResolveCompany(companyID)
		if err != nil {
			return nil, err
		}
		w.add("wh.tenant_id = ?", c)
	case s.IsManager:
		c, err := s.ResolveCompany(companyID)
		if err != nil {
			return nil, err
		}
		w.add("wh.tenant_id = ?", c)
		s.teamCond(w, "wh.user_id", "wh.tenant_id")
	case s.RoleKey == RoleEmployer:
		c, err := s.ResolveCompany(companyID)
		if err != nil {
			return nil, err
		}
		w.add("wh.tenant_id = ?", c)
	default:
		// Un profesional ve solo sus propias jornadas. Puede acotar a una
		// empresa, nunca ampliar.
		w.add("wh.user_id = ?", s.UserID)
		if companyID != 0 {
			w.add("wh.tenant_id = ?", companyID)
		}
	}
	return w, nil
}

// TasksFilter replica taskService.GetAll sobre tasks (alias t): empresa y
// managers ven todo el tenant; un profesional, solo los tableros a los que
// pertenece o que creó.
func (s *Scope) TasksFilter(companyID uint) (*where, error) {
	if s.RoleKey == RoleIT {
		return nil, userErr("Tu rol no tiene acceso a tareas.")
	}
	c, err := s.ResolveCompany(companyID)
	if err != nil {
		return nil, err
	}
	w := &where{}
	w.add("t.deleted_at IS NULL")
	w.add("t.tenant_id = ?", c)
	if !s.CrossCompany && !s.IsManager && s.RoleKey != RoleEmployer {
		w.add("t.board_id IN (SELECT id FROM boards WHERE created_by = ? OR id IN (SELECT board_id FROM board_members WHERE user_id = ?))", s.UserID, s.UserID)
	}
	return w, nil
}

// BoardsFilter replica boardService.GetAll sobre boards (alias b). En la web
// los managers solo ven sus tableros (el rol nunca vale "manager"), así que
// aquí también: el filtro por miembro aplica a todos menos plataforma y
// empresa.
func (s *Scope) BoardsFilter(companyID uint) (*where, error) {
	if s.RoleKey == RoleIT {
		return nil, userErr("Tu rol no tiene acceso a tableros.")
	}
	c, err := s.ResolveCompany(companyID)
	if err != nil {
		return nil, err
	}
	w := &where{}
	w.add("b.deleted_at IS NULL")
	w.add("b.tenant_id = ?", c)
	if !s.CrossCompany && s.RoleKey != RoleEmployer {
		w.add("b.created_by = ? OR b.id IN (SELECT board_id FROM board_members WHERE user_id = ?)", s.UserID, s.UserID)
	}
	return w, nil
}

// PeopleFilter acota profesionales con empleo activo (users u + employments
// e). Plataforma: cualquiera (o una empresa); empresa: su equipo; manager:
// su equipo.
func (s *Scope) PeopleFilter(companyID uint) (*where, error) {
	w := &where{}
	w.add("u.deleted_at IS NULL")
	w.add("e.deleted_at IS NULL AND e.status = 'active'")
	switch {
	case s.CrossCompany:
		if companyID != 0 {
			w.add("e.company_id = ?", companyID)
		}
	case s.RoleKey == RoleEmployer:
		c, err := s.ResolveCompany(companyID)
		if err != nil {
			return nil, err
		}
		w.add("e.company_id = ?", c)
	case s.IsManager:
		c, err := s.ResolveCompany(companyID)
		if err != nil {
			return nil, err
		}
		w.add("e.company_id = ?", c)
		s.teamCond(w, "u.id", "e.company_id")
	default:
		return nil, userErr("Tu rol no tiene acceso al listado de profesionales.")
	}
	return w, nil
}

// CompaniesFilter acota empresas (users u con user_type empleador).
func (s *Scope) CompaniesFilter() (*where, error) {
	w := &where{}
	w.add("u.deleted_at IS NULL")
	w.add("u.user_type = 'empleador'")
	switch {
	case s.CrossCompany:
	case s.RoleKey == RoleEmployer:
		w.add("u.id = ?", s.TenantID)
	default:
		return nil, userErr("Tu rol no tiene acceso al listado de empresas.")
	}
	return w, nil
}
