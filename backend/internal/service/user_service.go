package service

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type UserService interface {
	GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error)
	// GetByID entrega la ficha si el solicitante puede verla: superadmin y
	// customer success ven a cualquiera (CS no ve superadmins), el resto solo
	// dentro de su empresa o a sí mismo.
	GetByID(id, requesterID, tenantID uint, role string, isSuperadmin bool) (*models.User, error)
	Create(req map[string]interface{}) (*models.User, error)
	Update(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.User, error)
	Delete(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) error

	ToggleStatus(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	// PromoteToManager fija el NIVEL del usuario en la cadena de mando.
	// desired/desiredSupervisor son opcionales e independientes: nil = no tocar.
	// Marcar supervisor implica manager, y quitar manager se lleva la supervisión
	// (la misma invariante que aplica adminService.UpdateUser).
	PromoteToManager(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, desired, desiredSupervisor *bool) (*models.User, error)
	// AssignToManager fija el manager de un profesional DENTRO de una empresa.
	// companyID solo lo atiende el superadmin (que no tiene tenant propio) para
	// decir sobre cuál opera; para el resto se usa el suyo. Con 0 se cae a la
	// empresa activa del profesional, que es el comportamiento histórico.
	AssignToManager(professionalID, managerID, requesterID, tenantID, companyID uint, role string, isManager, isSuperadmin bool) (*models.User, error)
	// ReassignTeam mueve TODOS los reportes activos de oldManagerID (en todas las
	// empresas) al nuevo manager, o los desasigna si newManagerID es nil. Devuelve
	// cuántas membresías se reasignaron.
	ReassignTeam(oldManagerID uint, newManagerID *uint, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (int64, error)

	GetEmployees(employerID uint) ([]models.User, error)
	GetMyTeam(userID uint) ([]models.User, error)

	ChangePassword(id uint, currentPassword, newPassword string) error
	GetByEmail(email string) (*models.User, error)
}

type userService struct {
	repo           repository.UserRepository
	employmentRepo repository.EmploymentRepository
}

func NewUserService(repo repository.UserRepository, employmentRepo repository.EmploymentRepository) UserService {
	return &userService{repo: repo, employmentRepo: employmentRepo}
}

func (s *userService) authorizeUserTenant(target *models.User, requesterID, tenantID uint, isSuperadmin bool, requireManage bool, role string, isManager bool) error {
	if isSuperadmin {
		return nil
	}
	if target == nil {
		return errors.New("User not found")
	}
	if target.ID == requesterID {
		return nil
	}
	// Customer Success gestiona usuarios de TODAS las empresas, como el
	// superadmin: es soporte transversal y no tiene tenant propio (tenantID 0,
	// que el chequeo de abajo rechazaría). La línea roja son las cuentas
	// superadmin: editarles el correo o los datos sería una escalada de
	// privilegios, así que esas solo las gestiona otro superadmin.
	if role == string(models.UserTypeCustomerSuccess) {
		if target.IsSuperadmin || target.UserType == models.UserTypeSuperadmin {
			return errors.New("Access denied")
		}
		return nil
	}
	if tenantID == 0 || tenantForUser(target) != tenantID {
		return errors.New("Access denied")
	}
	if requireManage && !(isEmployerRole(role) || isManager) {
		return errors.New("Access denied")
	}
	return nil
}

func (s *userService) authorizeAdminAction(target *models.User, tenantID uint, isSuperadmin bool, role string) error {
	if isSuperadmin {
		return nil
	}
	if target == nil {
		return errors.New("User not found")
	}
	if !isEmployerRole(role) {
		return errors.New("Access denied")
	}
	if tenantID == 0 || tenantForUser(target) != tenantID {
		return errors.New("Access denied")
	}
	return nil
}

// authorizeTeamAction es authorizeAdminAction para las acciones sobre el EQUIPO
// (promover/quitar manager, asignar manager, reasignar equipo): además del
// empleador y el superadmin, deja pasar al supervisor, pero solo sobre gente de
// su propio árbol.
//
// Es la única guarda de todo el rol que abre algo reservado hasta ahora a la
// cuenta empresa, así que se escribe al revés de lo habitual: primero intenta el
// permiso de siempre y solo si falla considera la vía del supervisor. Así, si
// mañana authorizeAdminAction se endurece, esto hereda el cambio en vez de
// esquivarlo.
//
// Los dos cortes que importan: el objetivo tiene que estar en el MISMO tenant y
// ser descendiente del supervisor. Como IsDescendantOf devuelve false cuando la
// raíz y el objetivo coinciden, un supervisor tampoco puede usar esto sobre sí
// mismo (ni promoverse, ni reasignarse).
func (s *userService) authorizeTeamAction(target *models.User, requesterID, tenantID uint, isSuperadmin, isManager bool, role string) error {
	err := s.authorizeAdminAction(target, tenantID, isSuperadmin, role)
	if err == nil {
		return nil
	}
	if target == nil || !supervisorScopeApplies(s.repo, requesterID, isManager) {
		return err
	}
	if tenantID == 0 || tenantForUser(target) != tenantID {
		return errors.New("Access denied")
	}
	ok, derr := s.employmentRepo.IsDescendantOf(requesterID, target.ID, tenantID, maxSupervisorDepth)
	if derr != nil {
		return derr // fail-closed: sin poder resolver el árbol no se concede nada
	}
	if !ok {
		return errors.New("Access denied")
	}
	return nil
}

// authorizeTeamDestination autoriza al DESTINO de una asignación (a quién pasa a
// reportar alguien). Es authorizeTeamAction con una excepción: el propio
// supervisor es un destino válido.
//
// Hace falta porque IsDescendantOf devuelve false cuando la raíz y el objetivo
// coinciden —lo que es correcto para aprobar horas, donde nadie se aprueba a sí
// mismo— pero colgarse a alguien de uno mismo es la operación más normal del
// organigrama, y sin esto quedaba prohibida.
func (s *userService) authorizeTeamDestination(target *models.User, requesterID, tenantID uint, isSuperadmin, isManager bool, role string) error {
	if target != nil && target.ID == requesterID &&
		supervisorScopeApplies(s.repo, requesterID, isManager) {
		return nil
	}
	return s.authorizeTeamAction(target, requesterID, tenantID, isSuperadmin, isManager, role)
}

func (s *userService) GetAll(role, isManager, search string, companyID uint, offset, limit int) ([]models.User, int64, error) {
	return s.repo.GetAll(role, isManager, search, companyID, offset, limit)
}

func (s *userService) GetByID(id, requesterID, tenantID uint, role string, isSuperadmin bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, false, role, false); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Create(req map[string]interface{}) (*models.User, error) {
	password, ok := req["password"].(string)
	if !ok {
		return nil, errors.New("Password is required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}

	userType, _ := req["user_type"].(string)

	user := &models.User{
		Name:         req["name"].(string),
		Email:        req["email"].(string),
		Password:     string(hashedPassword),
		UserType:     models.UserType(userType),
		IsSuperadmin: userType == "superadmin",
	}

	if companyName, ok := req["company_name"].(string); ok {
		user.CompanyName = companyName
	}
	if jobTitle, ok := req["job_title"].(string); ok {
		user.JobTitle = jobTitle
	}

	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) Update(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, updates map[string]interface{}) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return nil, err
	}

	if id == requesterID && !isSuperadmin && role == string(models.UserTypeProfessional) {
		for k := range updates {
			if k != "avatar" {
				delete(updates, k)
			}
		}
	}

	// This validates fields manually like the original implementation
	if name, ok := updates["name"].(string); ok && name != "" {
		user.Name = name
	}
	if email, ok := updates["email"].(string); ok && email != "" {
		user.Email = email
	}
	if avatar, ok := updates["avatar"].(string); ok && avatar != "" {
		user.Avatar = avatar
	}
	if jt, ok := updates["job_title"].(string); ok && jt != "" {
		user.JobTitle = jt
	}
	if pn, ok := updates["phone_number"].(string); ok && pn != "" {
		user.PhoneNumber = pn
	}
	if country, ok := updates["country"].(string); ok && country != "" {
		user.Country = country
	}
	if state, ok := updates["state"].(string); ok && state != "" {
		user.State = state
	}
	if city, ok := updates["city"].(string); ok && city != "" {
		user.City = city
	}
	if location, ok := updates["location"].(string); ok && location != "" {
		user.Location = location
	}
	if idDoc, ok := updates["identity_document"].(string); ok && idDoc != "" {
		user.IdentityDocument = idDoc
	}

	if len(updates) > 0 {
		if err := s.repo.Update(user, updates); err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *userService) Delete(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}
	if err := s.authorizeUserTenant(user, requesterID, tenantID, isSuperadmin, true, role, isManager); err != nil {
		return err
	}

	// No se puede eliminar un manager que aún tiene equipo a su cargo:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if user.IsManager {
		count, cerr := countManagerReports(s.repo, s.employmentRepo, id)
		if cerr != nil {
			return cerr // fail-closed
		}
		if count > 0 {
			return fmt.Errorf("No se puede eliminar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	return s.repo.Delete(id)
}

func (s *userService) ToggleStatus(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeAdminAction(user, tenantID, isSuperadmin, role); err != nil {
		return nil, err
	}

	// Si el manager va a quedar inactivo y aún tiene equipo, no se puede:
	// dejaría a esos profesionales sin aprobador. Hay que reasignarlos primero.
	if user.IsActive && user.IsManager {
		count, cerr := countManagerReports(s.repo, s.employmentRepo, id)
		if cerr != nil {
			return nil, cerr // fail-closed
		}
		if count > 0 {
			return nil, fmt.Errorf("No se puede desactivar el manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	user.IsActive = !user.IsActive
	if !user.IsActive {
		user.TokenVersion++ // revoke sessions when suspending a user (audit A-04)
	}
	if err := s.repo.Save(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) PromoteToManager(id, requesterID, tenantID uint, role string, isManager, isSuperadmin bool, desired, desiredSupervisor *bool) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if err := s.authorizeTeamAction(user, requesterID, tenantID, isSuperadmin, isManager, role); err != nil {
		return nil, err
	}

	var newVal bool
	switch {
	case desired != nil:
		newVal = *desired
	case desiredSupervisor != nil:
		// Solo vino el nivel de supervisor: el de manager se deduce de él, en vez
		// de que el toggle de respaldo lo invierta por su cuenta.
		newVal = user.IsManager
	default:
		newVal = !user.IsManager // toggle de respaldo por compatibilidad
	}
	// Marcar supervisor implica manager: los dos flags nunca se separan.
	if desiredSupervisor != nil && *desiredSupervisor {
		newVal = true
	}

	// Solo profesionales y customer success pueden ser manager.
	if newVal && user.UserType != models.UserTypeProfessional && user.UserType != models.UserTypeCustomerSuccess {
		return nil, errors.New("Manager inválido: solo profesionales o customer success pueden ser manager")
	}

	// No permitir quitar el rol de manager si todavía tiene equipo a su cargo:
	// dejaría a esos subordinados sin aprobador. Hay que reasignarlos primero.
	if !newVal && user.IsManager {
		count, cerr := countManagerReports(s.repo, s.employmentRepo, id)
		if cerr != nil {
			return nil, cerr // fail-closed
		}
		if count > 0 {
			return nil, fmt.Errorf("No se puede quitar el rol de manager: %s todavía tiene %d profesional(es) a su cargo. Reasigna su equipo primero", user.Name, count)
		}
	}

	updates := map[string]interface{}{"is_manager": newVal}
	newSupervisor := user.IsSupervisor
	if desiredSupervisor != nil {
		newSupervisor = *desiredSupervisor
	}
	// Quitar el rol de manager se lleva la supervisión con él: un supervisor que
	// ya no es manager no podría aprobar nada de su árbol.
	if !newVal {
		newSupervisor = false
	}
	if newSupervisor != user.IsSupervisor {
		updates["is_supervisor"] = newSupervisor
	}
	if err := s.repo.Update(user, updates); err != nil {
		return nil, err
	}

	user.IsManager = newVal
	user.IsSupervisor = newSupervisor
	return user, nil
}

func (s *userService) GetEmployees(employerID uint) ([]models.User, error) {
	_, err := s.repo.GetByID(employerID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	return s.repo.GetEmployees(employerID)
}

func (s *userService) AssignToManager(professionalID, managerID, requesterID, tenantID, companyID uint, role string, isManager, isSuperadmin bool) (*models.User, error) {
	professional, err := s.repo.GetByID(professionalID)
	if err != nil {
		return nil, errors.New("Professional not found")
	}
	if err := s.authorizeTeamAction(professional, requesterID, tenantID, isSuperadmin, isManager, role); err != nil {
		return nil, err
	}

	// Empresa sobre la que se opera. Importa cuando el profesional trabaja en
	// varias: el cambio tiene que caer en la que se está editando y no en la que
	// resulte ser su activa. Solo el superadmin puede indicarla (no tiene tenant
	// propio); a los demás se les impone el suyo, venga lo que venga en la
	// petición, para que nadie escriba en una empresa ajena.
	targetCompany := tenantID
	if isSuperadmin && companyID > 0 {
		targetCompany = companyID
	}
	if targetCompany == 0 && professional.EmpleadorID != nil {
		targetCompany = *professional.EmpleadorID
	}
	// users.manager_id es un puntero GLOBAL que refleja la empresa activa, así
	// que solo se toca cuando se está operando justo sobre esa empresa.
	isActiveCompany := professional.EmpleadorID != nil && *professional.EmpleadorID == targetCompany

	var newManagerID *uint
	if managerID == 0 {
		newManagerID = nil
	} else {
		if managerID == professionalID {
			return nil, errors.New("Un profesional no puede ser su propio manager")
		}
		manager, err := s.repo.GetByID(managerID)
		if err != nil {
			return nil, errors.New("Manager not found")
		}
		// El destino se autoriza igual que el profesional: un supervisor mueve
		// gente ENTRE managers suyos, no hacia alguien de fuera de su árbol (eso
		// le entregaría las horas de su subordinado a un tercero). Él mismo sí es
		// un destino válido.
		if err := s.authorizeTeamDestination(manager, requesterID, tenantID, isSuperadmin, isManager, role); err != nil {
			return nil, err
		}
		// El ciclo se valida contra la empresa que se está editando, que es donde
		// vive la cadena de mando que puede cerrarse.
		if err := ensureValidManager(s.repo, s.employmentRepo, manager, professionalID, targetCompany); err != nil {
			return nil, err
		}
		newManagerID = &managerID
	}

	// La fuente de verdad por-empresa es el empleo; el puntero global solo la
	// acompaña cuando coinciden. Así, editar el organigrama de una empresa nunca
	// mueve a nadie en otra.
	if targetCompany > 0 {
		if emp, err := s.employmentRepo.GetActive(professional.ID, targetCompany); err == nil && emp != nil {
			if err := s.employmentRepo.Update(emp, map[string]interface{}{"manager_id": newManagerID}); err != nil {
				return nil, err
			}
			// Asignar manager REEMPLAZA, no suma. Antes solo se cambiaba cuál era
			// el principal y los vínculos anteriores seguían vivos, así que el
			// manager saliente conservaba la aprobación de horas de alguien que ya
			// no era suyo — invisible salvo por el contador de managers extra.
			// Sumar managers sigue siendo posible, pero como acción explícita
			// desde el editor multi-manager de la ficha.
			_ = s.employmentRepo.ClearManagers(emp.ID)
			syncPrimaryManager(s.employmentRepo, emp.ID, newManagerID)
		}
	}

	if isActiveCompany || targetCompany == 0 {
		professional.ManagerID = newManagerID
		if err := s.repo.Save(professional); err != nil {
			return nil, err
		}
	}

	return professional, nil
}

func (s *userService) ReassignTeam(oldManagerID uint, newManagerID *uint, requesterID, tenantID uint, role string, isManager, isSuperadmin bool) (int64, error) {
	oldManager, err := s.repo.GetByID(oldManagerID)
	if err != nil {
		return 0, errors.New("User not found")
	}
	if err := s.authorizeTeamAction(oldManager, requesterID, tenantID, isSuperadmin, isManager, role); err != nil {
		return 0, err
	}

	// Reasignación acotada a la empresa activa del manager: respeta el invariante
	// per-empresa (no mueve reportes de otras empresas hacia un manager que no
	// pertenece a ellas). Para managers multi-empresa se reasigna por empresa.
	companyID := uint(0)
	if oldManager.EmpleadorID != nil {
		companyID = *oldManager.EmpleadorID
	}
	if companyID == 0 {
		return 0, errors.New("El manager no tiene una empresa activa")
	}

	if newManagerID != nil {
		newManager, err := s.repo.GetByID(*newManagerID)
		if err != nil {
			return 0, errors.New("Manager inválido: manager no encontrado")
		}
		// El destino también se autoriza: sin esto un supervisor podría descargar
		// el equipo de uno de sus managers en alguien de fuera de su árbol. Él
		// mismo sí puede recibirlo.
		if err := s.authorizeTeamDestination(newManager, requesterID, tenantID, isSuperadmin, isManager, role); err != nil {
			return 0, err
		}
		// Todo el equipo de oldManager pasa a newManager: se cierra un ciclo si
		// newManager cuelga de oldManager (sería uno de los reportes que se
		// mueven, o descendiente de alguno), así que se valida contra él.
		if err := ensureValidManager(s.repo, s.employmentRepo, newManager, oldManagerID, companyID); err != nil {
			return 0, err
		}
	}

	n, err := s.employmentRepo.ReassignManager(oldManagerID, newManagerID, companyID)
	if err != nil {
		return 0, err
	}
	if _, err := s.repo.ReassignManager(oldManagerID, newManagerID, companyID); err != nil {
		return 0, err
	}
	// Dual-write: mueve los vínculos principales del set (employment_managers)
	// del manager saliente al nuevo (o los quita si newManagerID es nil).
	old := oldManagerID
	_ = s.employmentRepo.ReassignManagerLinks(&old, newManagerID, companyID)
	return n, nil
}

func (s *userService) GetMyTeam(userID uint) ([]models.User, error) {
	user, err := s.repo.GetByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if !user.IsManager {
		return []models.User{}, nil
	}

	// Un supervisor tiene a cargo el árbol entero, no solo a quienes le reportan
	// directo: sus managers y la gente de esos managers.
	if SupervisorScopeEnabled() && user.IsSupervisor {
		companyID := tenantForUser(user)
		if companyID > 0 {
			ids, err := s.employmentRepo.DescendantIDs(userID, companyID, maxSupervisorDepth)
			if err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				return []models.User{}, nil
			}
			return s.repo.GetByIDs(ids)
		}
	}

	if MultiManagerReadsEnabled() {
		return s.repo.GetTeamViaLinks(userID)
	}
	return s.repo.GetTeam(userID)
}

func (s *userService) ChangePassword(id uint, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("User not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return errors.New("Current password is incorrect")
	}

	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("Failed to hash password")
	}

	user.Password = string(hashedPassword)
	user.TokenVersion++ // revoke existing sessions on password change (audit A-04)
	return s.repo.Save(user)
}
func (s *userService) GetByEmail(email string) (*models.User, error) {
	return s.repo.GetByEmail(email)
}
