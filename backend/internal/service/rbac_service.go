package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

var (
	ErrRBACNotFound    = errors.New("No encontrado")
	ErrRBACCrossTenant = errors.New("El recurso pertenece a otra empresa")
)

type RBACService interface {
	// Roles
	ListRoles(tenantID uint) ([]models.Role, error)
	CreateRole(tenantID, userID uint, name, description, permissions string) (*models.Role, error)
	UpdateRole(tenantID, roleID uint, name, description, permissions *string) (*models.Role, error)
	DeleteRole(tenantID, roleID uint) error
	GetRoleUsers(tenantID, roleID uint) ([]models.User, error)
	AssignRole(tenantID, roleID, userID uint) error
	UnassignRole(tenantID, roleID, userID uint) error

	// EffectivePermissions combina los permisos de los roles del usuario EN UNA
	// EMPRESA (gana el nivel más permisivo). Con multi-empresa, los permisos se
	// scopean al tenant activo. tenantID 0 = todos los tenants (uso histórico).
	// hasRoles=false significa que el usuario no tiene roles en ese tenant y
	// conserva el comportamiento histórico de su cuenta.
	EffectivePermissions(userID, tenantID uint) (map[string]string, bool, error)
	// UserRBAC devuelve los roles y grupos de un usuario dentro de un tenant.
	UserRBAC(tenantID, userID uint) ([]models.Role, []models.Group, error)
	// SeedDefaultRoles crea los roles preconfigurados de una empresa
	// (Colaborador, Supervisor, Solo lectura, Soporte). Idempotente: omite
	// los que ya existan por nombre.
	SeedDefaultRoles(tenantID, createdBy uint) error

	// Groups
	ListGroups(tenantID uint) ([]models.Group, error)
	CreateGroup(tenantID, userID uint, name, description string) (*models.Group, error)
	UpdateGroup(tenantID, groupID uint, name, description *string) (*models.Group, error)
	DeleteGroup(tenantID, groupID uint) error
	GetGroupMembers(tenantID, groupID uint) ([]models.User, error)
	AddGroupMember(tenantID, groupID, userID uint) error
	RemoveGroupMember(tenantID, groupID, userID uint) error
}

type rbacService struct {
	repo     repository.RBACRepository
	userRepo repository.UserRepository
}

func NewRBACService(repo repository.RBACRepository, userRepo repository.UserRepository) RBACService {
	return &rbacService{repo: repo, userRepo: userRepo}
}

// normalizePermissions valida que permissions sea un objeto JSON
// {"modulo": "none"|"view"|"edit"} y lo re-serializa normalizado.
func normalizePermissions(permissions string) (string, error) {
	permissions = strings.TrimSpace(permissions)
	if permissions == "" {
		return "{}", nil
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(permissions), &parsed); err != nil {
		return "", errors.New("Los permisos deben ser un objeto JSON de módulo a nivel")
	}
	for module, level := range parsed {
		if strings.TrimSpace(module) == "" {
			return "", errors.New("Los permisos contienen un módulo vacío")
		}
		if !models.IsValidPermissionLevel(level) {
			return "", errors.New("Nivel de permiso inválido: usa 'none', 'view' o 'edit'")
		}
	}
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return "", errors.New("No se pudieron serializar los permisos")
	}
	return string(normalized), nil
}

// tenantUser verifica que el usuario exista y pertenezca al tenant indicado
// (profesionales y customer success vía empleador_id; la cuenta empresa es su
// propio tenant).
func (s *rbacService) tenantUser(tenantID, userID uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("Usuario no encontrado")
	}
	if models.TenantForUser(user) != tenantID {
		return nil, errors.New("El usuario no pertenece a esta empresa")
	}
	return user, nil
}

// ── Roles ────────────────────────────────────────────────────────────────────

func (s *rbacService) ListRoles(tenantID uint) ([]models.Role, error) {
	return s.repo.ListRoles(tenantID)
}

func (s *rbacService) tenantRole(tenantID, roleID uint) (*models.Role, error) {
	role, err := s.repo.GetRole(roleID)
	if err != nil {
		return nil, ErrRBACNotFound
	}
	if role.TenantID != tenantID {
		return nil, ErrRBACCrossTenant
	}
	return role, nil
}

func (s *rbacService) CreateRole(tenantID, userID uint, name, description, permissions string) (*models.Role, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("El nombre del rol es obligatorio")
	}
	normalized, err := normalizePermissions(permissions)
	if err != nil {
		return nil, err
	}

	role := &models.Role{
		TenantID:    tenantID,
		Name:        utils.SanitizeHTML(strings.TrimSpace(name)),
		Description: utils.SanitizeHTML(description),
		Permissions: normalized,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateRole(role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *rbacService) UpdateRole(tenantID, roleID uint, name, description, permissions *string) (*models.Role, error) {
	role, err := s.tenantRole(tenantID, roleID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if name != nil {
		if strings.TrimSpace(*name) == "" {
			return nil, errors.New("El nombre del rol es obligatorio")
		}
		updates["name"] = utils.SanitizeHTML(strings.TrimSpace(*name))
	}
	if description != nil {
		updates["description"] = utils.SanitizeHTML(*description)
	}
	if permissions != nil {
		normalized, err := normalizePermissions(*permissions)
		if err != nil {
			return nil, err
		}
		updates["permissions"] = normalized
	}
	if len(updates) == 0 {
		return role, nil
	}
	if err := s.repo.UpdateRole(role, updates); err != nil {
		return nil, err
	}
	return s.repo.GetRole(roleID)
}

func (s *rbacService) DeleteRole(tenantID, roleID uint) error {
	if _, err := s.tenantRole(tenantID, roleID); err != nil {
		return err
	}
	return s.repo.DeleteRole(roleID)
}

func (s *rbacService) GetRoleUsers(tenantID, roleID uint) ([]models.User, error) {
	if _, err := s.tenantRole(tenantID, roleID); err != nil {
		return nil, err
	}
	return s.repo.GetRoleUsers(roleID)
}

func (s *rbacService) AssignRole(tenantID, roleID, userID uint) error {
	if _, err := s.tenantRole(tenantID, roleID); err != nil {
		return err
	}
	if _, err := s.tenantUser(tenantID, userID); err != nil {
		return err
	}
	return s.repo.AssignRole(roleID, userID)
}

func (s *rbacService) UnassignRole(tenantID, roleID, userID uint) error {
	if _, err := s.tenantRole(tenantID, roleID); err != nil {
		return err
	}
	return s.repo.UnassignRole(roleID, userID)
}

// Presets con los que nace toda empresa nueva (mismo contenido que
// seed_default_roles.sql para las empresas pre-existentes).
var defaultRolePresets = []struct {
	Name        string
	Description string
	Permissions string
}{
	{
		Name:        "Colaborador",
		Description: "Operación diaria: gestiona sus tareas, registra horas y participa en el chat.",
		Permissions: `{"tasks":"edit","hours":"edit","chat":"edit","tutorials":"view","reports":"none","tickets":"none"}`,
	},
	{
		Name:        "Supervisor",
		Description: "Coordina al equipo: todo lo del colaborador más visibilidad de reportes. Para aprobar horas, combinar con el flag de manager.",
		Permissions: `{"tasks":"edit","hours":"edit","chat":"edit","tutorials":"view","reports":"view","tickets":"none"}`,
	},
	{
		Name:        "Solo lectura",
		Description: "Auditoría / consulta: ve tareas, horas y chat sin poder modificar nada.",
		Permissions: `{"tasks":"view","hours":"view","chat":"view","tutorials":"view","reports":"view","tickets":"none"}`,
	},
	{
		Name:        "Soporte",
		Description: "Customer success asignado a la empresa: gestiona tickets y chat, consulta tareas.",
		Permissions: `{"tasks":"view","hours":"none","chat":"edit","tutorials":"view","reports":"none","tickets":"edit"}`,
	},
}

func (s *rbacService) SeedDefaultRoles(tenantID, createdBy uint) error {
	if tenantID == 0 {
		return errors.New("tenant inválido")
	}
	existing, err := s.repo.ListRoles(tenantID)
	if err != nil {
		return err
	}
	names := make(map[string]bool, len(existing))
	for _, role := range existing {
		names[role.Name] = true
	}
	for _, preset := range defaultRolePresets {
		if names[preset.Name] {
			continue
		}
		role := &models.Role{
			TenantID:    tenantID,
			Name:        preset.Name,
			Description: preset.Description,
			Permissions: preset.Permissions,
			CreatedBy:   createdBy,
		}
		if err := s.repo.CreateRole(role); err != nil {
			return err
		}
	}
	return nil
}

func permissionRank(level string) int {
	switch level {
	case models.PermissionEdit:
		return 2
	case models.PermissionView:
		return 1
	default:
		return 0
	}
}

func (s *rbacService) EffectivePermissions(userID, tenantID uint) (map[string]string, bool, error) {
	roles, err := s.repo.GetUserRoles(userID, tenantID)
	if err != nil {
		return nil, false, err
	}
	if len(roles) == 0 {
		return nil, false, nil
	}

	effective := map[string]string{}
	for _, role := range roles {
		var perms map[string]string
		if err := json.Unmarshal([]byte(role.Permissions), &perms); err != nil {
			continue // permisos corruptos: el rol no aporta
		}
		for module, level := range perms {
			if permissionRank(level) > permissionRank(effective[module]) {
				effective[module] = level
			}
		}
	}
	return effective, true, nil
}

func (s *rbacService) UserRBAC(tenantID, userID uint) ([]models.Role, []models.Group, error) {
	if _, err := s.tenantUser(tenantID, userID); err != nil {
		return nil, nil, err
	}
	roles, err := s.repo.GetUserRoles(userID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	groups, err := s.repo.GetUserGroups(userID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	return roles, groups, nil
}

// ── Groups ───────────────────────────────────────────────────────────────────

func (s *rbacService) ListGroups(tenantID uint) ([]models.Group, error) {
	return s.repo.ListGroups(tenantID)
}

func (s *rbacService) tenantGroup(tenantID, groupID uint) (*models.Group, error) {
	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		return nil, ErrRBACNotFound
	}
	if group.TenantID != tenantID {
		return nil, ErrRBACCrossTenant
	}
	return group, nil
}

func (s *rbacService) CreateGroup(tenantID, userID uint, name, description string) (*models.Group, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("El nombre del grupo es obligatorio")
	}
	group := &models.Group{
		TenantID:    tenantID,
		Name:        utils.SanitizeHTML(strings.TrimSpace(name)),
		Description: utils.SanitizeHTML(description),
		CreatedBy:   userID,
	}
	if err := s.repo.CreateGroup(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *rbacService) UpdateGroup(tenantID, groupID uint, name, description *string) (*models.Group, error) {
	group, err := s.tenantGroup(tenantID, groupID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if name != nil {
		if strings.TrimSpace(*name) == "" {
			return nil, errors.New("El nombre del grupo es obligatorio")
		}
		updates["name"] = utils.SanitizeHTML(strings.TrimSpace(*name))
	}
	if description != nil {
		updates["description"] = utils.SanitizeHTML(*description)
	}
	if len(updates) == 0 {
		return group, nil
	}
	if err := s.repo.UpdateGroup(group, updates); err != nil {
		return nil, err
	}
	return s.repo.GetGroup(groupID)
}

func (s *rbacService) DeleteGroup(tenantID, groupID uint) error {
	if _, err := s.tenantGroup(tenantID, groupID); err != nil {
		return err
	}
	return s.repo.DeleteGroup(groupID)
}

func (s *rbacService) GetGroupMembers(tenantID, groupID uint) ([]models.User, error) {
	if _, err := s.tenantGroup(tenantID, groupID); err != nil {
		return nil, err
	}
	return s.repo.GetGroupMembers(groupID)
}

func (s *rbacService) AddGroupMember(tenantID, groupID, userID uint) error {
	if _, err := s.tenantGroup(tenantID, groupID); err != nil {
		return err
	}
	if _, err := s.tenantUser(tenantID, userID); err != nil {
		return err
	}
	return s.repo.AddGroupMember(groupID, userID)
}

func (s *rbacService) RemoveGroupMember(tenantID, groupID, userID uint) error {
	if _, err := s.tenantGroup(tenantID, groupID); err != nil {
		return err
	}
	return s.repo.RemoveGroupMember(groupID, userID)
}
