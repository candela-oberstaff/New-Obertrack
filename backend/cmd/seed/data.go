package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

type seeder struct {
	db       *gorm.DB
	password string
	now      time.Time
	users    map[string]*models.User
}

// Claves internas de los usuarios sembrados. Se usan para referirse unos a otros
// (manager, asignado, autor) sin depender de IDs que no existen hasta insertar.
const (
	uSuper   = "superadmin"
	uIT      = "it"
	uCS1     = "cs.carmen"
	uCS2     = "cs.pedro"
	uAcmeOwn = "acme.marta"
	uAcmeMgr = "acme.diego"
	uAcmeUX  = "acme.laura"
	uAcmeQA  = "acme.valentina"
	uAcmeDev = "acme.hugo"
	uGlobOwn = "globex.gabriela"
	uGlobMgr = "globex.andres"
	uGlobMkt = "globex.sofia"
	uGlobSup = "globex.mateo"
)

type userSpec struct {
	key       string
	name      string
	userType  models.UserType
	company   string
	industry  string
	jobTitle  string
	isManager bool
	tenantKey string // empresa a la que pertenece (empleador_id)
	phone     string
	country   string
	state     string
	city      string
}

// El orden importa: los empleadores tienen que existir antes que su gente,
// porque de ellos sale el empleador_id.
func userSpecs() []userSpec {
	return []userSpec{
		// --- Personal de Oberstaff (sin empresa: son transversales) ---
		{key: uSuper, name: "Ana Superadmin", userType: models.UserTypeSuperadmin, jobTitle: "Administración de la plataforma", country: "Venezuela", state: "Distrito Capital", city: "Caracas", phone: "+58 412 1110001"},
		{key: uIT, name: "Iván Bracho", userType: models.UserTypeITAnalyst, jobTitle: "Analista de IT", country: "Venezuela", state: "Miranda", city: "Los Teques", phone: "+58 412 1110002"},
		{key: uCS1, name: "Carmen Soto", userType: models.UserTypeCustomerSuccess, jobTitle: "Customer Success", country: "Colombia", state: "Bogotá D.C.", city: "Bogotá", phone: "+57 300 1110003"},
		{key: uCS2, name: "Pedro Aguilar", userType: models.UserTypeCustomerSuccess, jobTitle: "Customer Success", country: "Argentina", state: "Buenos Aires", city: "La Plata", phone: "+54 11 1110004"},

		// --- Acme S.A. ---
		{key: uAcmeOwn, name: "Marta Aguirre", userType: models.UserTypeEmployer, company: "Acme S.A.", industry: "Tecnología", jobTitle: "CEO", country: "Venezuela", state: "Distrito Capital", city: "Caracas", phone: "+58 212 2220001"},
		{key: uAcmeMgr, name: "Diego Ramírez", userType: models.UserTypeProfessional, tenantKey: uAcmeOwn, jobTitle: "Líder de Ingeniería", isManager: true, country: "Venezuela", state: "Distrito Capital", city: "Caracas", phone: "+58 412 2220002"},
		{key: uAcmeUX, name: "Laura Méndez", userType: models.UserTypeProfessional, tenantKey: uAcmeOwn, jobTitle: "Diseñadora UX", country: "Colombia", state: "Antioquia", city: "Medellín", phone: "+57 300 2220003"},
		{key: uAcmeQA, name: "Valentina Ríos", userType: models.UserTypeProfessional, tenantKey: uAcmeOwn, jobTitle: "Analista QA", country: "Venezuela", state: "Carabobo", city: "Valencia", phone: "+58 412 2220004"},
		{key: uAcmeDev, name: "Hugo Peña", userType: models.UserTypeProfessional, tenantKey: uAcmeOwn, jobTitle: "Desarrollador Backend", country: "Perú", state: "Lima", city: "Lima", phone: "+51 999 2220005"},

		// --- Globex Corp ---
		{key: uGlobOwn, name: "Gabriela Torres", userType: models.UserTypeEmployer, company: "Globex Corp", industry: "Logística", jobTitle: "Directora de Operaciones", country: "Argentina", state: "Córdoba", city: "Córdoba", phone: "+54 351 3330001"},
		{key: uGlobMgr, name: "Andrés Vega", userType: models.UserTypeProfessional, tenantKey: uGlobOwn, jobTitle: "Líder de Producto", isManager: true, country: "Argentina", state: "Buenos Aires", city: "CABA", phone: "+54 11 3330002"},
		{key: uGlobMkt, name: "Sofía Navarro", userType: models.UserTypeProfessional, tenantKey: uGlobOwn, jobTitle: "Especialista en Marketing", country: "México", state: "Jalisco", city: "Guadalajara", phone: "+52 33 3330003"},
		{key: uGlobSup, name: "Mateo Ortiz", userType: models.UserTypeProfessional, tenantKey: uGlobOwn, jobTitle: "Soporte TI", country: "Chile", state: "Región Metropolitana", city: "Santiago", phone: "+56 9 3330004"},
	}
}

// managerOf define quién aprueba las horas de quién (users.manager_id y el
// manager principal del empleo).
var managerOf = map[string]string{
	uAcmeUX:  uAcmeMgr,
	uAcmeQA:  uAcmeMgr,
	uAcmeDev: uAcmeMgr,
	uAcmeMgr: uAcmeOwn,
	uGlobMkt: uGlobMgr,
	uGlobSup: uGlobMgr,
	uGlobMgr: uGlobOwn,
}

func (s *seeder) Run() error {
	s.users = make(map[string]*models.User)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"usuarios", s.seedUsers},
		{"empleos", s.seedEmployments},
		{"roles", s.seedRoles},
		{"tableros y tareas", s.seedBoards},
		{"jornadas", s.seedWorkHours},
		{"chat", s.seedChat},
		{"soporte", s.seedSupport},
		{"incidentes", s.seedIncidents},
		{"notificaciones", s.seedNotifications},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		log.Printf("  ✓ %s", step.name)
	}

	s.printSummary()
	return nil
}

// --- Usuarios -------------------------------------------------------------

func (s *seeder) seedUsers() error {
	hash, err := bcrypt.GenerateFromPassword([]byte(s.password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	for _, spec := range userSpecs() {
		u, err := s.ensureUser(spec, string(hash))
		if err != nil {
			return err
		}
		s.users[spec.key] = u
	}

	// Segunda pasada: el manager tiene que existir para poder apuntarlo.
	for key, mgrKey := range managerOf {
		u, mgr := s.users[key], s.users[mgrKey]
		if u == nil || mgr == nil {
			continue
		}
		if u.ManagerID != nil && *u.ManagerID == mgr.ID {
			continue
		}
		u.ManagerID = &mgr.ID
		if err := s.db.Model(&models.User{}).Where("id = ?", u.ID).
			Update("manager_id", mgr.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) ensureUser(spec userSpec, hash string) (*models.User, error) {
	email := spec.key + "@" + demoDomain

	var u models.User
	err := s.db.Unscoped().Where("email = ?", email).First(&u).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return nil, err
	}

	verified := s.now
	u.Email = email
	u.Name = spec.name
	u.Password = hash
	u.UserType = spec.userType
	u.IsSuperadmin = spec.userType == models.UserTypeSuperadmin
	u.IsManager = spec.isManager
	u.IsActive = true
	u.CompanyName = spec.company
	u.Industry = spec.industry
	u.JobTitle = spec.jobTitle
	u.PhoneNumber = spec.phone
	u.Country = spec.country
	u.State = spec.state
	u.City = spec.city
	u.EmailVerifiedAt = &verified
	u.OnboardingStatus = "not_required"
	// Revive una cuenta que un -reset parcial (o la UI) hubiera dado de baja.
	u.DeletedAt = gorm.DeletedAt{}
	if spec.tenantKey != "" {
		u.EmpleadorID = &s.users[spec.tenantKey].ID
	}

	if isNew {
		if err := s.db.Create(&u).Error; err != nil {
			return nil, err
		}
	} else if err := s.db.Unscoped().Save(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// --- Empleos (membresías + expediente) ------------------------------------

func (s *seeder) seedEmployments() error {
	for _, spec := range userSpecs() {
		if spec.tenantKey == "" {
			continue
		}
		u := s.users[spec.key]
		company := s.users[spec.tenantKey]
		// Antigüedad escalonada para que el expediente no muestre a todo el
		// equipo entrando el mismo día.
		started := s.now.AddDate(0, -(3 + int(u.ID%9)), 0)
		if err := s.ensureEmployment(u.ID, company.ID, spec.jobTitle, s.managerID(spec.key), started); err != nil {
			return err
		}
	}

	// Laura trabaja en las dos empresas: es el caso que ejercita el switcher
	// multi-empresa (su empleador_id sigue apuntando a Acme, la activa).
	laura, globex := s.users[uAcmeUX], s.users[uGlobOwn]
	andres := s.users[uGlobMgr].ID
	return s.ensureEmployment(laura.ID, globex.ID, "Diseñadora UX (part-time)", &andres, s.now.AddDate(0, -2, 0))
}

func (s *seeder) managerID(key string) *uint {
	mgrKey, ok := managerOf[key]
	if !ok {
		return nil
	}
	mgr, ok := s.users[mgrKey]
	if !ok {
		return nil
	}
	return &mgr.ID
}

func (s *seeder) ensureEmployment(userID, companyID uint, jobTitle string, managerID *uint, started time.Time) error {
	var emp models.Employment
	err := s.db.Where("user_id = ? AND company_id = ? AND status = ?",
		userID, companyID, models.EmploymentActive).First(&emp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		emp = models.Employment{
			UserID:      userID,
			CompanyID:   companyID,
			JobTitle:    jobTitle,
			ManagerID:   managerID,
			Status:      models.EmploymentActive,
			StartedAt:   started,
			StartReason: "Alta de datos de demostración",
		}
		if err := s.db.Create(&emp).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if err := s.db.Model(&models.Employment{}).Where("id = ?", emp.ID).Updates(map[string]interface{}{
			"job_title":  jobTitle,
			"manager_id": managerID,
		}).Error; err != nil {
			return err
		}
	}

	if managerID == nil {
		return nil
	}
	// Espejo N-a-N del manager principal (fase 1 del multi-manager).
	var link models.EmploymentManager
	err = s.db.Where("employment_id = ? AND manager_id = ?", emp.ID, *managerID).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.Create(&models.EmploymentManager{
			EmploymentID: emp.ID,
			ManagerID:    *managerID,
			IsPrimary:    true,
		}).Error
	}
	return err
}

// --- Roles y grupos -------------------------------------------------------

func (s *seeder) seedRoles() error {
	rbac := service.NewRBACService(
		repository.NewRBACRepository(s.db),
		repository.NewUserRepository(s.db),
	)

	assignments := map[string]string{
		uAcmeMgr: "Supervisor",
		uAcmeUX:  "Colaborador",
		uAcmeQA:  "Colaborador",
		uAcmeDev: "Colaborador",
		uGlobMgr: "Supervisor",
		uGlobMkt: "Colaborador",
		uGlobSup: "Solo lectura",
	}

	for _, ownerKey := range []string{uAcmeOwn, uGlobOwn} {
		owner := s.users[ownerKey]
		// Los mismos presets que recibe cualquier empresa nueva desde la UI.
		if err := rbac.SeedDefaultRoles(owner.ID, owner.ID); err != nil {
			return err
		}
	}

	for userKey, roleName := range assignments {
		u := s.users[userKey]
		tenantID := *u.EmpleadorID
		var role models.Role
		if err := s.db.Where("tenant_id = ? AND name = ?", tenantID, roleName).First(&role).Error; err != nil {
			return err
		}
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.UserRole{UserID: u.ID, RoleID: role.ID}).Error; err != nil {
			return err
		}
	}

	// Un grupo por empresa, para que la pantalla de Roles y Grupos no salga vacía.
	groups := map[string][]string{
		uAcmeOwn: {uAcmeMgr, uAcmeUX, uAcmeQA, uAcmeDev},
		uGlobOwn: {uGlobMgr, uGlobMkt, uGlobSup},
	}
	for ownerKey, memberKeys := range groups {
		owner := s.users[ownerKey]
		var group models.Group
		err := s.db.Where("tenant_id = ? AND name = ?", owner.ID, "Equipo core").First(&group).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			group = models.Group{
				TenantID:    owner.ID,
				Name:        "Equipo core",
				Description: "Gente que sostiene la operación del día a día.",
				CreatedBy:   owner.ID,
			}
			if err := s.db.Create(&group).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		for _, key := range memberKeys {
			if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.GroupMember{GroupID: group.ID, UserID: s.users[key].ID}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Tableros y tareas ----------------------------------------------------

type taskSpec struct {
	title     string
	desc      string
	status    models.TaskStatus
	priority  models.TaskPriority
	assignees []string
	startDays int // días respecto de hoy (negativo = pasado)
	endDays   int
	comments  []taskComment
}

type taskComment struct {
	author  string
	content string
}

func (s *seeder) seedBoards() error {
	boards := []struct {
		name     string
		desc     string
		color    string
		ownerKey string
		members  []string
		tasks    []taskSpec
	}{
		{
			name:     "Operación Acme",
			desc:     "Trabajo en curso del equipo de Acme S.A.",
			color:    "#3b82f6",
			ownerKey: uAcmeOwn,
			members:  []string{uAcmeMgr, uAcmeUX, uAcmeQA, uAcmeDev},
			tasks: []taskSpec{
				{
					title: "Rediseñar el flujo de alta de usuarios", desc: "Reducir de 5 a 3 pasos el alta desde el panel.",
					status: models.TaskStatusInProcess, priority: models.PriorityHigh,
					assignees: []string{uAcmeUX}, startDays: -6, endDays: 4,
					comments: []taskComment{
						{author: uAcmeUX, content: "Subí los wireframes de los 3 pasos. Falta validar el copy."},
						{author: uAcmeMgr, content: "Se ve bien. Cuidado con el paso de credenciales, ahí se caen."},
					},
				},
				{
					title: "Migrar reportes a la nueva consulta agregada", desc: "El reporte mensual tarda 40s; hay que agregarlo en base.",
					status: models.TaskStatusInProcess, priority: models.PriorityUrgent,
					assignees: []string{uAcmeDev}, startDays: -3, endDays: 7,
				},
				{
					title: "Plan de pruebas de regresión", desc: "Cobertura de los módulos de horas y tareas antes del release.",
					status: models.TaskStatusTodo, priority: models.PriorityMedium,
					assignees: []string{uAcmeQA}, startDays: 1, endDays: 12,
				},
				{
					title: "Documentar el módulo de jornadas", desc: "Guía corta para el equipo nuevo.",
					status: models.TaskStatusTodo, priority: models.PriorityLow,
					assignees: []string{uAcmeUX, uAcmeQA}, startDays: 3, endDays: 15,
				},
				{
					title: "Auditoría de accesos del trimestre", desc: "Revisar quién tiene rol de supervisor y por qué.",
					status: models.TaskStatusDone, priority: models.PriorityMedium,
					assignees: []string{uAcmeMgr}, startDays: -25, endDays: -12,
					comments: []taskComment{{author: uAcmeMgr, content: "Cerrada. Se quitaron 2 supervisores que ya no aplican."}},
				},
				{
					title: "Corregir el cálculo de horas en feriados", desc: "Los feriados se contaban como jornada completa.",
					status: models.TaskStatusDone, priority: models.PriorityHigh,
					assignees: []string{uAcmeDev}, startDays: -18, endDays: -9,
				},
			},
		},
		{
			name:     "Lanzamiento Globex",
			desc:     "Preparación del lanzamiento regional.",
			color:    "#8b5cf6",
			ownerKey: uGlobOwn,
			members:  []string{uGlobMgr, uGlobMkt, uGlobSup, uAcmeUX},
			tasks: []taskSpec{
				{
					title: "Campaña de lanzamiento en LATAM", desc: "Piezas para redes y correo, tres mercados.",
					status: models.TaskStatusInProcess, priority: models.PriorityHigh,
					assignees: []string{uGlobMkt}, startDays: -8, endDays: 6,
					comments: []taskComment{{author: uGlobMgr, content: "Prioricemos México y Argentina, Chile va después."}},
				},
				{
					title: "Checklist de soporte para el día 1", desc: "Guiones de respuesta y turnos de guardia.",
					status: models.TaskStatusTodo, priority: models.PriorityUrgent,
					assignees: []string{uGlobSup}, startDays: 2, endDays: 9,
				},
				{
					title: "Landing del producto", desc: "Diseño y contenido de la página de aterrizaje.",
					status: models.TaskStatusInProcess, priority: models.PriorityMedium,
					assignees: []string{uAcmeUX, uGlobMkt}, startDays: -4, endDays: 10,
				},
				{
					title: "Definir métricas de éxito", desc: "Qué se mide la primera semana y quién lo revisa.",
					status: models.TaskStatusDone, priority: models.PriorityMedium,
					assignees: []string{uGlobMgr}, startDays: -20, endDays: -14,
				},
			},
		},
	}

	for _, b := range boards {
		owner := s.users[b.ownerKey]
		board, err := s.ensureBoard(b.name, b.desc, b.color, owner.ID, owner.ID)
		if err != nil {
			return err
		}
		memberIDs := []uint{owner.ID}
		for _, key := range b.members {
			memberIDs = append(memberIDs, s.users[key].ID)
		}
		for _, id := range memberIDs {
			if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.BoardMember{BoardID: board.ID, UserID: id}).Error; err != nil {
				return err
			}
		}
		for i, t := range b.tasks {
			if err := s.ensureTask(board, owner.ID, i, t); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *seeder) ensureBoard(name, desc, color string, tenantID, creatorID uint) (*models.Board, error) {
	var board models.Board
	err := s.db.Where("name = ? AND tenant_id = ?", name, tenantID).First(&board).Error
	if err == nil {
		return &board, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	board = models.Board{Name: name, Description: desc, Color: color, CreatedBy: creatorID, TenantID: tenantID}
	if err := s.db.Create(&board).Error; err != nil {
		return nil, err
	}

	// Mismas fases (y mismo mapeo a estados) que crea board_service al abrir un
	// tablero desde la UI: sin ellas el kanban no tiene columnas.
	phases := []struct {
		name, color, status string
	}{
		{"Por hacer", "#6b7280", string(models.TaskStatusTodo)},
		{"En proceso", "#3b82f6", string(models.TaskStatusInProcess)},
		{"Finalizado", "#22c55e", string(models.TaskStatusDone)},
	}
	for i, p := range phases {
		phase := models.Phase{Name: p.name, Color: p.color, Status: p.status, Order: i}
		if err := s.db.Create(&phase).Error; err != nil {
			return nil, err
		}
		if err := s.db.Create(&models.BoardPhase{BoardID: board.ID, PhaseID: phase.ID}).Error; err != nil {
			return nil, err
		}
	}
	return &board, nil
}

func (s *seeder) ensureTask(board *models.Board, creatorID uint, order int, spec taskSpec) error {
	var task models.Task
	err := s.db.Where("board_id = ? AND title = ?", board.ID, spec.title).First(&task).Error
	if err == nil {
		return nil // ya sembrada: no se pisan cambios hechos a mano
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	start := s.now.AddDate(0, 0, spec.startDays)
	end := s.now.AddDate(0, 0, spec.endDays)
	task = models.Task{
		Title:       spec.title,
		Description: spec.desc,
		Status:      spec.status,
		Priority:    spec.priority,
		StartDate:   &start,
		EndDate:     &end,
		Completed:   spec.status == models.TaskStatusDone,
		CreatedBy:   creatorID,
		BoardID:     board.ID,
		TenantID:    board.TenantID,
		Order:       order,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return err
	}

	for _, key := range spec.assignees {
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.TaskUser{TaskID: task.ID, UserID: s.users[key].ID}).Error; err != nil {
			return err
		}
	}
	for i, c := range spec.comments {
		comment := models.Comment{
			TaskID:    task.ID,
			UserID:    s.users[c.author].ID,
			Content:   c.content,
			CreatedAt: start.Add(time.Duration(i+1) * 6 * time.Hour),
		}
		if err := s.db.Create(&comment).Error; err != nil {
			return err
		}
	}
	return nil
}

// --- Jornadas -------------------------------------------------------------

// seedWorkHours genera ~6 semanas de jornadas por profesional con la mezcla que
// hace falta para probar el módulo: aprobadas (histórico), pendientes (la cola
// del manager), una rechazada y una ausencia.
func (s *seeder) seedWorkHours() error {
	activities := []string{
		"Avance en las tareas del tablero y revisión con el equipo.",
		"Sesión de trabajo enfocada + atención de pendientes.",
		"Reunión de seguimiento, ajustes y documentación.",
		"Trabajo en la entrega de la semana.",
		"Revisión de pendientes y apoyo al equipo.",
	}

	for _, spec := range userSpecs() {
		if spec.userType != models.UserTypeProfessional {
			continue
		}
		u := s.users[spec.key]
		tenantID := *u.EmpleadorID
		approver := s.managerID(spec.key)

		// La unicidad de la jornada la dan dos índices PARCIALES (uno para
		// 'recover' y otro para el resto), así que un ON CONFLICT por columnas
		// no aplica: se comprueba antes qué días ya están cargados.
		loaded, err := s.loadedWorkDays(u.ID, tenantID)
		if err != nil {
			return err
		}

		day := 0
		for offset := 42; offset >= 1; offset-- {
			date := time.Date(s.now.Year(), s.now.Month(), s.now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -offset)
			if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
				continue
			}
			day++
			if loaded[date.Format("2006-01-02")] {
				continue // ya hay jornada ese día: se respeta la que esté cargada
			}

			wh := models.WorkHour{
				UserID:      u.ID,
				TenantID:    tenantID,
				WorkDate:    date,
				WorkType:    models.WorkTypeComplete,
				HoursWorked: 7.5 + float64(pick(u.ID, day, 4))/2, // 7.5 a 9.0
				Activities:  activities[pick(u.ID, day, len(activities))],
			}

			switch {
			// Una ausencia por persona, en la tercera semana hacia atrás.
			case day == 12:
				wh.WorkType = models.WorkTypeAbsence
				wh.HoursWorked = 0
				wh.AbsenceHours = 8
				wh.AbsenceReason = "Permiso médico"
				wh.Activities = "Ausencia justificada."
				wh.Approved = true
				wh.ApprovedBy = approver
				wh.ApprovedAt = ptrTime(date.AddDate(0, 0, 2))
			// Una rechazada, para que se vea el estado y el motivo.
			case day == 9:
				wh.Rejected = true
				wh.RejectedBy = approver
				wh.RejectedAt = ptrTime(date.AddDate(0, 0, 1))
				wh.RejectionReason = "Las actividades no coinciden con las tareas del tablero. Corrige y vuelve a enviar."
			// Lo anterior a la última semana ya está aprobado.
			case offset > 7:
				wh.Approved = true
				wh.ApprovedBy = approver
				wh.ApprovedAt = ptrTime(date.AddDate(0, 0, 1))
			}

			if err := s.db.Create(&wh).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// loadedWorkDays devuelve los días (YYYY-MM-DD) que ya tienen jornada cargada
// para ese profesional en esa empresa.
func (s *seeder) loadedWorkDays(userID, tenantID uint) (map[string]bool, error) {
	var dates []time.Time
	if err := s.db.Model(&models.WorkHour{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Pluck("work_date", &dates).Error; err != nil {
		return nil, err
	}
	loaded := make(map[string]bool, len(dates))
	for _, d := range dates {
		loaded[d.Format("2006-01-02")] = true
	}
	return loaded, nil
}

// --- Chat -----------------------------------------------------------------

func (s *seeder) seedChat() error {
	channels := []struct {
		name     string
		desc     string
		ownerKey string
		members  []string
		messages []taskComment // author + content
	}{
		{
			name: "general", desc: "Canal general de Acme S.A.", ownerKey: uAcmeOwn,
			members: []string{uAcmeMgr, uAcmeUX, uAcmeQA, uAcmeDev},
			messages: []taskComment{
				{author: uAcmeOwn, content: "¡Bienvenidos al canal general! Aquí van los anuncios del equipo."},
				{author: uAcmeMgr, content: "Recordatorio: las jornadas de la semana se cargan antes del viernes a las 5."},
				{author: uAcmeUX, content: "Subí los wireframes del alta de usuarios al tablero, cualquier comentario es bienvenido."},
				{author: uAcmeQA, content: "Empiezo el plan de pruebas el lunes, les paso el borrador."},
			},
		},
		{
			name: "general", desc: "Canal general de Globex Corp", ownerKey: uGlobOwn,
			members: []string{uGlobMgr, uGlobMkt, uGlobSup},
			messages: []taskComment{
				{author: uGlobOwn, content: "Arrancamos la cuenta regresiva del lanzamiento. Todo lo del día 1 va en el tablero."},
				{author: uGlobMkt, content: "Las piezas de la campaña están en revisión, quedan listas el jueves."},
				{author: uGlobSup, content: "Armo el guion de soporte y lo comparto por acá."},
			},
		},
	}

	for _, c := range channels {
		owner := s.users[c.ownerKey]
		memberIDs := []uint{owner.ID}
		for _, key := range c.members {
			memberIDs = append(memberIDs, s.users[key].ID)
		}

		channel, err := s.ensureChannel(c.name, c.desc, models.ChannelTypePublic, owner.ID, owner.ID, memberIDs)
		if err != nil {
			return err
		}
		if err := s.ensureMessages(channel, owner.ID, c.messages); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) ensureChannel(name, desc string, kind models.ChannelType, tenantID, creatorID uint, memberIDs []uint) (*models.Channel, error) {
	var channel models.Channel
	err := s.db.Where("name = ? AND type = ? AND tenant_id = ?", name, kind, tenantID).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		channel = models.Channel{
			Name: name, Description: desc, Type: kind,
			CreatedBy: creatorID, TenantID: tenantID, IsActive: true,
		}
		if err := s.db.Create(&channel).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	for i, id := range memberIDs {
		role := "member"
		if id == creatorID {
			role = "admin"
		}
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ChannelMember{
			ChannelID: channel.ID, UserID: id, Role: role,
			JoinedAt: s.now.Add(-time.Duration(i) * time.Hour),
		}).Error; err != nil {
			return nil, err
		}
	}
	return &channel, nil
}

// ensureMessages solo escribe si el canal está vacío: así una segunda corrida no
// repite la conversación ni pisa lo que alguien haya escrito probando.
func (s *seeder) ensureMessages(channel *models.Channel, tenantID uint, msgs []taskComment) error {
	var count int64
	if err := s.db.Model(&models.ChannelMessage{}).Where("channel_id = ?", channel.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for i, m := range msgs {
		msg := models.ChannelMessage{
			ChannelID: channel.ID,
			TenantID:  tenantID,
			UserID:    s.users[m.author].ID,
			Content:   m.content,
			CreatedAt: s.now.Add(-time.Duration(len(msgs)-i) * 90 * time.Minute),
		}
		if err := s.db.Create(&msg).Error; err != nil {
			return err
		}
	}
	return nil
}

// --- Soporte --------------------------------------------------------------

// seedSupport reproduce lo que hace ContactSupport: un canal privado por
// solicitante (nombre "Soporte · Nombre #id") con los agentes dentro, más el
// ticket que lo gestiona. Deja uno en cola, uno asignado y uno resuelto.
func (s *seeder) seedSupport() error {
	agents := []uint{s.users[uCS1].ID, s.users[uCS2].ID, s.users[uIT].ID}

	tickets := []struct {
		requester  string
		subject    string
		module     string
		priority   string
		status     string
		assignee   string
		daysAgo    int
		firstMsg   string
		agentReply string
	}{
		{
			requester: uAcmeUX, subject: "No puedo adjuntar archivos a una tarea",
			module: "tareas", priority: "alta", status: models.SupportStatusOpen, daysAgo: 1,
			firstMsg: "Cuando intento subir un PDF de más de 2 MB a una tarea, se queda cargando y no pasa nada.",
		},
		{
			requester: uGlobSup, subject: "Las horas de un compañero no llegan a aprobación",
			module: "horas", priority: "media", status: models.SupportStatusAssigned, assignee: uCS1, daysAgo: 4,
			firstMsg:   "Sofía cargó sus jornadas de la semana pasada pero no me aparecen en la cola de aprobación.",
			agentReply: "Gracias por avisar, lo estoy revisando. ¿Me confirmas si Sofía tiene manager asignado en su expediente?",
		},
		{
			requester: uAcmeQA, subject: "Solicitud de acceso al módulo de reportes",
			module: "reportes", priority: "baja", status: models.SupportStatusResolved, assignee: uCS2, daysAgo: 11,
			firstMsg:   "Necesito ver los reportes mensuales para el plan de pruebas.",
			agentReply: "Listo, te asigné el rol de Supervisor en Acme. Ya deberías ver el módulo de Reportes.",
		},
	}

	for _, t := range tickets {
		requester := s.users[t.requester]
		tenantID := models.TenantForUser(requester)
		members := append([]uint{requester.ID}, agents...)

		channel, err := s.ensureChannel(
			fmt.Sprintf("Soporte · %s #%d", requester.Name, requester.ID),
			"Canal de soporte con Customer Success",
			models.ChannelTypePrivate, tenantID, requester.ID, members,
		)
		if err != nil {
			return err
		}

		var existing models.SupportTicket
		err = s.db.Where("channel_id = ? AND subject = ?", channel.ID, t.subject).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		created := s.now.AddDate(0, 0, -t.daysAgo)
		ticket := models.SupportTicket{
			ChannelID:   channel.ID,
			TenantID:    tenantID,
			RequesterID: requester.ID,
			Subject:     t.subject,
			Priority:    t.priority,
			Module:      t.module,
			Status:      t.status,
			CreatedAt:   created,
			UpdatedAt:   created,
		}
		if t.assignee != "" {
			assignee := s.users[t.assignee].ID
			ticket.AssignedTo = &assignee
			ticket.AssignedAt = ptrTime(created.Add(2 * time.Hour))
			if t.status == models.SupportStatusResolved {
				ticket.ResolvedBy = &assignee
				ticket.ResolvedAt = ptrTime(created.AddDate(0, 0, 1))
			}
		}
		if err := s.db.Create(&ticket).Error; err != nil {
			return err
		}

		msgs := []struct {
			userID  uint
			content string
			at      time.Time
		}{
			{requester.ID, fmt.Sprintf("🆕 Nueva solicitud: %s · Prioridad %s · Módulo %s", t.subject, t.priority, t.module), created},
			{requester.ID, t.firstMsg, created.Add(time.Minute)},
		}
		if t.agentReply != "" {
			msgs = append(msgs, struct {
				userID  uint
				content string
				at      time.Time
			}{s.users[t.assignee].ID, t.agentReply, created.Add(3 * time.Hour)})
		}
		for _, m := range msgs {
			if err := s.db.Create(&models.ChannelMessage{
				ChannelID: channel.ID, TenantID: tenantID, UserID: m.userID,
				Content: m.content, CreatedAt: m.at,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Incidentes -----------------------------------------------------------

func (s *seeder) seedIncidents() error {
	incidents := []struct {
		title, desc, kind, country, state, status string
		daysAgo                                   int
		responses                                 map[string]string
	}{
		{
			title: "Lluvias intensas en el área metropolitana de Caracas",
			desc:  "Reporte de inundaciones y cortes eléctricos. Confirmar el estado de cada persona en la zona.",
			kind:  "clima", country: "Venezuela", state: "Distrito Capital",
			status: models.IncidentStatusOpen, daysAgo: 2,
			responses: map[string]string{
				uAcmeOwn: models.IncidentResponseOk,
				uAcmeMgr: models.IncidentResponseContactado,
				uIT:      models.IncidentResponsePendiente,
			},
		},
		{
			title: "Corte de servicio eléctrico en Medellín",
			desc:  "Falla zonal de varias horas. Se contactó a la persona afectada y se reprogramó su jornada.",
			kind:  "servicios", country: "Colombia", state: "Antioquia",
			status: models.IncidentStatusClosed, daysAgo: 16,
			responses: map[string]string{uAcmeUX: models.IncidentResponseOk},
		},
	}

	for _, in := range incidents {
		var incident models.Incident
		err := s.db.Where("title = ?", in.title).First(&incident).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := s.now.AddDate(0, 0, -in.daysAgo)
			incident = models.Incident{
				Title: in.title, Description: in.desc, Kind: in.kind,
				Country: in.country, State: in.state, Status: in.status,
				CreatedBy: s.users[uSuper].ID, CreatedAt: created,
			}
			if in.status == models.IncidentStatusClosed {
				incident.ClosedAt = ptrTime(created.AddDate(0, 0, 1))
			}
			if err := s.db.Create(&incident).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		for userKey, status := range in.responses {
			note := ""
			if status == models.IncidentResponseOk {
				note = "Sin novedad, puede trabajar con normalidad."
			}
			if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.IncidentResponse{
				IncidentID: incident.ID, UserID: s.users[userKey].ID,
				Status: status, Note: note,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Notificaciones -------------------------------------------------------

// seedNotifications reescribe la bandeja de los usuarios de demo en cada corrida
// (son efímeras por naturaleza y así la campanita siempre muestra algo fresco).
func (s *seeder) seedNotifications() error {
	ids := s.userIDs()
	if err := s.db.Unscoped().Where("user_id IN ?", ids).Delete(&models.Notification{}).Error; err != nil {
		return err
	}

	type notif struct {
		userKey  string
		kind     string
		title    string
		message  string
		hoursAgo int
		read     bool
	}
	notifs := []notif{
		{uAcmeUX, "task_assigned", "Nueva tarea asignada", "Se te asignó la tarea: Rediseñar el flujo de alta de usuarios", 30, true},
		{uAcmeUX, "work_hour_rejected", "Jornadas rechazadas", "Diego Ramírez rechazó 1 jornada. Revisa el motivo y vuelve a enviarla.", 6, false},
		{uAcmeDev, "task_assigned", "Nueva tarea asignada", "Se te asignó la tarea: Migrar reportes a la nueva consulta agregada", 20, false},
		{uAcmeQA, "work_hour_approved", "Jornadas aprobadas", "Se aprobaron tus jornadas de la semana pasada.", 48, true},
		{uAcmeMgr, "work_hour_created", "Nueva jornada registrada", "Laura Méndez registró su jornada de ayer.", 4, false},
		{uGlobMkt, "task_assigned", "Nueva tarea asignada", "Se te asignó la tarea: Campaña de lanzamiento en LATAM", 26, true},
		{uGlobSup, "support", "Soporte: nueva actividad", "Carmen Soto respondió tu solicitud de soporte.", 3, false},
		{uCS1, "support", "Nueva solicitud de soporte", "Laura Méndez solicita soporte. Acéptala para atenderla.", 2, false},
		{uSuper, "inactivity_alert", "Profesional sin actividad", "Hugo Peña no registra jornadas desde hace 3 días.", 12, false},
	}

	for _, n := range notifs {
		created := s.now.Add(-time.Duration(n.hoursAgo) * time.Hour)
		row := models.Notification{
			UserID:    s.users[n.userKey].ID,
			Type:      n.kind,
			Title:     n.title,
			Message:   n.message,
			Data:      `{"link":"/"}`,
			CreatedAt: created,
		}
		if n.read {
			row.ReadAt = ptrTime(created.Add(time.Hour))
		}
		if err := s.db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// --- Utilidades -----------------------------------------------------------

func (s *seeder) userIDs() []uint {
	ids := make([]uint, 0, len(s.users))
	for _, u := range s.users {
		ids = append(ids, u.ID)
	}
	return ids
}

// pick devuelve un valor estable en [0,n) a partir de dos semillas: la misma
// persona y el mismo día siempre dan el mismo resultado, así que dos corridas
// del seeder producen exactamente los mismos datos.
func pick(seed uint, day, n int) int {
	return int((seed*31 + uint(day)*17) % uint(n)) //nolint:gosec // no es aleatoriedad criptográfica
}

func ptrTime(t time.Time) *time.Time { return &t }

func (s *seeder) printSummary() {
	fmt.Printf(`
────────────────────────────────────────────────────────────────
 Datos de demostración cargados
────────────────────────────────────────────────────────────────
 Contraseña de TODAS las cuentas: %s

 Superadmin          %s@%s
 Analista de IT      %s@%s
 Customer Success    %s@%s  ·  %s@%s

 Acme S.A.
   Empresa           %s@%s
   Manager           %s@%s
   Profesionales     %s@%s · %s@%s · %s@%s

 Globex Corp
   Empresa           %s@%s
   Manager           %s@%s
   Profesionales     %s@%s · %s@%s

 Laura (acme.laura) trabaja en las dos empresas: sirve para probar
 el switcher multi-empresa.

 Para volver a empezar de cero:  seed -reset
────────────────────────────────────────────────────────────────
`,
		s.password,
		uSuper, demoDomain,
		uIT, demoDomain,
		uCS1, demoDomain, uCS2, demoDomain,
		uAcmeOwn, demoDomain,
		uAcmeMgr, demoDomain,
		uAcmeUX, demoDomain, uAcmeQA, demoDomain, uAcmeDev, demoDomain,
		uGlobOwn, demoDomain,
		uGlobMgr, demoDomain,
		uGlobMkt, demoDomain, uGlobSup, demoDomain,
	)
}
