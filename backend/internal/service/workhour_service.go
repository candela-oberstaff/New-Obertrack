package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type WorkHourService interface {
	GetAll(userID uint, role string, crossCompany, isManager bool, tenantID, companyFilter uint, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error)
	Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error)
	Update(id, tenantID, userID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}) (*models.WorkHour, error)
	// Approve/Reject procesan las jornadas ELEGIBLES del lote y devuelven
	// (procesadas, omitidas): las jornadas propias del manager u otras no
	// autorizadas se omiten en vez de tumbar el lote entero. Error solo cuando
	// ninguna es procesable.
	Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint) (int, int, error)
	Reject(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint, reason string) (int, int, error)
	GetSummary(userID uint, role string, isSuperadmin, isManager bool, tenantID, companyFilter uint, userIDFilter string) (map[string]float64, error)
	GetPending(tenantID, userID uint, role string, isSuperadmin bool, isManager bool, companyFilter uint, userIDFilter string) ([]models.WorkHour, error)
	SendReportEmail(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) error
	// SendPeriodReport envía el reporte de una empresa para un rango arbitrario
	// de fechas. Lo usa el worker de envíos automáticos.
	SendPeriodReport(recipient *models.User, tenantID uint, periodTitle, periodLabel string, start, end time.Time) error
	GetPDFReportBytes(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) ([]byte, string, error)
	GetExcelReportBytes(userID uint, role string, isSuperadmin, isManager bool, tenantID uint, month int, year int, companyFilter uint) ([]byte, string, error)
}

type workHourService struct {
	repo           repository.WorkHourRepository
	userRepo       repository.UserRepository
	notifSvc       NotificationService
	brevoSvc       *BrevoService
	ticketSvc      TicketService
	employmentRepo repository.EmploymentRepository
}

func NewWorkHourService(
	repo repository.WorkHourRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
	brevoSvc *BrevoService,
	ticketSvc TicketService,
	employmentRepo repository.EmploymentRepository,
) WorkHourService {
	return &workHourService{
		repo:           repo,
		userRepo:       userRepo,
		notifSvc:       notifSvc,
		brevoSvc:       brevoSvc,
		ticketSvc:      ticketSvc,
		employmentRepo: employmentRepo,
	}
}

func (s *workHourService) parseStringVal(val interface{}) string {
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

func (s *workHourService) parseFloatVal(val interface{}) float64 {
	if val == nil {
		return 0
	}
	strVal := fmt.Sprintf("%v", val)
	if f, err := strconv.ParseFloat(strVal, 64); err == nil {
		return f
	}
	return 0
}

// canManageWorkHourOf resuelve si el actor puede tocar la jornada de otro
// usuario dentro de una empresa: un manager llega a sus reportes directos, un
// supervisor a todo su árbol.
//
// superScope se pasa ya resuelto en vez de calcularse aquí porque los lotes de
// aprobación recorren muchas jornadas y averiguar si el actor es supervisor es
// una consulta que no depende de cuál se esté mirando.
func (s *workHourService) canManageWorkHourOf(targetUserID, companyID, actorID uint, superScope bool) bool {
	// Separación de funciones: nadie gestiona sus propias jornadas, sea manager
	// o supervisor.
	if targetUserID == actorID {
		return false
	}
	if superScope {
		ok, _ := s.employmentRepo.IsDescendantOf(actorID, targetUserID, companyID, maxSupervisorDepth)
		return ok
	}
	if MultiManagerReadsEnabled() {
		ok, _ := s.employmentRepo.IsManagerOf(targetUserID, companyID, actorID)
		return ok
	}
	emp, err := s.employmentRepo.GetActive(targetUserID, companyID)
	return err == nil && emp != nil && emp.ManagerID != nil && *emp.ManagerID == actorID
}

// crossCompany: quien lee a nivel plataforma (superadmin y customer success) y
// por tanto elige la empresa en vez de quedar atado a la suya. Es un permiso de
// LECTURA; las escrituras siguen mirando isSuperadmin por separado.
func (s *workHourService) GetAll(userID uint, role string, crossCompany, isManager bool, tenantID, companyFilter uint, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error) {
	filters := make(map[string]interface{})

	if crossCompany {
		// Debe elegir empresa explícitamente. Sin eso no se devuelve nada, para
		// no mezclar nunca horas de tenants distintos en la misma vista.
		if companyFilter == 0 {
			return []models.WorkHour{}, 0, nil
		}
		filters["tenant_id"] = companyFilter
	} else if isManager {
		// Un manager ve solo su equipo (él + subordinados directos), igual que su
		// lista de pendientes y su resumen; no todas las horas del tenant. Así
		// "lo que ve" coincide con "lo que puede aprobar". Un supervisor mantiene
		// esa correspondencia sobre su árbol completo.
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		ids, applied, err := supervisorTeamAndSelfIDs(s.userRepo, s.employmentRepo, userID, tenantID, isManager)
		if err != nil {
			return nil, 0, err
		}
		if applied {
			filters["user_ids"] = ids
		} else if MultiManagerReadsEnabled() {
			filters["manager_or_user_links_id"] = userID
		} else {
			filters["manager_or_user_id"] = userID
		}
	} else if isEmployerRole(role) {
		// Los empleadores ven todas las horas de su tenant.
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
	} else {
		// Los profesionales solo ven sus propias horas.
		filters["user_id"] = userID
	}

	if userIDFilter != "" && (crossCompany || role == string(models.UserTypeEmployer) || role == "empleador") {
		uid, _ := strconv.ParseUint(userIDFilter, 10, 32)
		filters["user_id"] = uint(uid)
	}

	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filters["start_date"] = t
		}
	}

	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			filters["end_date"] = t
		}
	}

	return s.repo.FindAll(filters, offset, limit)
}

// Errores tipados de la creación/edición de jornadas. El handler los mapea a
// códigos HTTP con errors.Is en vez de comparar el texto del mensaje (frágil).
var (
	ErrInvalidDateFormat = errors.New("Invalid date format")
	ErrFutureWorkDate    = errors.New("No puedes registrar horas en fechas futuras")
	ErrDuplicateWorkDay  = errors.New("Ya existe una jornada para esta fecha en esta empresa. Solo puedes registrar una jornada por día.")
	ErrDuplicateRecover  = errors.New("Ya existe una recuperación para esta fecha en esta empresa. Solo puedes registrar una recuperación por día.")
)

const standardWorkDay = 8.0

// clampHours acota las horas a un rango válido [0,24]. Evita valores negativos
// o desbordes de la columna decimal(5,2) y stats infladas por payloads del
// cliente sin validar.
func clampHours(h float64) float64 {
	if h < 0 {
		return 0
	}
	if h > 24 {
		return 24
	}
	return h
}

// clampAbsenceHours acota las horas de ausencia a una jornada laboral [0,8].
func clampAbsenceHours(h float64) float64 {
	if h < 0 {
		return 0
	}
	if h > standardWorkDay {
		return standardWorkDay
	}
	return h
}

func (s *workHourService) Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error) {
	workDateStr := s.parseStringVal(reqData["work_date"])
	workDate, err := time.Parse("2006-01-02", workDateStr)
	if err != nil {
		return nil, ErrInvalidDateFormat
	}

	today := time.Now().Truncate(24 * time.Hour)
	if workDate.After(today) {
		return nil, ErrFutureWorkDate
	}

	creator, _ := s.userRepo.GetByID(userID)
	tenantID := models.TenantForUser(creator)

	// Las horas son autoritativas en el servidor: no se confía en lo que mande
	// el cliente para complete/absence; recover se acota a [0,24].
	workTypeStr := s.parseStringVal(reqData["work_type"])
	workType := models.WorkTypeComplete
	absenceHours := clampAbsenceHours(s.parseFloatVal(reqData["absence_hours"]))
	var hoursWorked float64

	switch workTypeStr {
	case "absence":
		workType = models.WorkTypeAbsence
		hoursWorked = standardWorkDay - absenceHours
	case "recover":
		workType = models.WorkTypeRecover
		absenceHours = 0
		hoursWorked = clampHours(s.parseFloatVal(reqData["hours_worked"]))
	default:
		workType = models.WorkTypeComplete
		absenceHours = 0
		hoursWorked = standardWorkDay
	}

	isRecover := workType == models.WorkTypeRecover
	if _, err := s.repo.FindByUserDateKind(userID, workDate, tenantID, isRecover); err == nil {
		if isRecover {
			return nil, ErrDuplicateRecover
		}
		return nil, ErrDuplicateWorkDay
	}

	workHour := &models.WorkHour{
		UserID:        userID,
		TenantID:      tenantID,
		WorkDate:      workDate,
		WorkType:      workType,
		HoursWorked:   hoursWorked,
		Activities:    utils.SanitizeHTML(s.parseStringVal(reqData["activities"])),
		Comments:      utils.SanitizeHTML(s.parseStringVal(reqData["comments"])),
		AbsenceReason: s.parseStringVal(reqData["absence_reason"]),
		AbsenceHours:  absenceHours,
	}

	startTime := s.parseStringVal(reqData["start_time"])
	endTime := s.parseStringVal(reqData["end_time"])

	if startTime != "" {
		if t, err := time.Parse("15:04", startTime); err == nil {
			workHour.StartTime = &t
		}
	}
	if endTime != "" {
		if t, err := time.Parse("15:04", endTime); err == nil {
			workHour.EndTime = &t
		}
	}

	if err := s.repo.Create(workHour); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			if isRecover {
				return nil, ErrDuplicateRecover
			}
			return nil, ErrDuplicateWorkDay
		}
		return nil, errors.New("Failed to create work hour")
	}

	// Fetch with preload for response
	finalWH, err := s.repo.FindByID(workHour.ID)
	if err == nil && finalWH != nil {
		// Notificar al Manager y al Empleador internamente
		go func() {
			user, _ := s.userRepo.GetByID(finalWH.UserID)
			if user != nil {
				// Notificar al Manager per-empresa de la jornada
				if MultiManagerReadsEnabled() {
					// Con el flag ON notificamos a TODOS los managers del empleo.
					if managerIDs, err := s.employmentRepo.ListManagerIDs(finalWH.UserID, finalWH.TenantID); err == nil {
						for _, managerID := range managerIDs {
							_ = s.notifSvc.CreateNotification(managerID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID, "link": "/work-hours"})
						}
					}
				} else if emp, err := s.employmentRepo.GetActive(finalWH.UserID, finalWH.TenantID); err == nil && emp != nil && emp.ManagerID != nil {
					_ = s.notifSvc.CreateNotification(*emp.ManagerID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID, "link": "/work-hours"})
				}
				// Notificar al Empleador
				if user.EmpleadorID != nil {
					_ = s.notifSvc.CreateNotification(*user.EmpleadorID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID, "link": "/work-hours"})
				}
			}
		}()
	}

	return finalWH, err
}

func (s *workHourService) Update(id, tenantID, userID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}) (*models.WorkHour, error) {
	var workHour *models.WorkHour
	var err error
	if !isSuperadmin && tenantID > 0 {
		workHour, err = s.repo.FindByIDAndTenant(id, tenantID)
	} else {
		workHour, err = s.repo.FindByID(id)
	}
	if err != nil {
		return nil, errors.New("Work hour not found")
	}

	if !isSuperadmin {
		if tenantID == 0 || workHour.TenantID != tenantID {
			return nil, errors.New("Access denied")
		}
		allowed := workHour.UserID == userID || isEmployerRole(role)
		if !allowed && isManager {
			superScope := supervisorScopeApplies(s.userRepo, userID, isManager)
			allowed = s.canManageWorkHourOf(workHour.UserID, workHour.TenantID, userID, superScope)
		}
		if !allowed {
			return nil, errors.New("Access denied")
		}
	}

	workDateStr := s.parseStringVal(reqData["work_date"])
	if workDateStr != "" {
		if t, err := time.Parse("2006-01-02", workDateStr); err == nil {
			workHour.WorkDate = t
		}
	}

	workTypeStr := s.parseStringVal(reqData["work_type"])
	if workTypeStr != "" {
		workHour.WorkType = models.WorkType(workTypeStr)
	}

	if val, ok := reqData["absence_reason"]; ok {
		workHour.AbsenceReason = s.parseStringVal(val)
	}
	if val, ok := reqData["absence_hours"]; ok {
		workHour.AbsenceHours = clampAbsenceHours(s.parseFloatVal(val))
	}

	// Las horas se derivan del tipo de jornada en el servidor; solo recover
	// toma las horas del cliente (acotadas). No se confía en hours_worked para
	// complete/absence.
	switch workHour.WorkType {
	case models.WorkTypeAbsence:
		workHour.HoursWorked = standardWorkDay - workHour.AbsenceHours
	case models.WorkTypeRecover:
		workHour.AbsenceReason = ""
		workHour.AbsenceHours = 0
		if val, ok := reqData["hours_worked"]; ok {
			workHour.HoursWorked = clampHours(s.parseFloatVal(val))
		}
	default:
		workHour.HoursWorked = standardWorkDay
		workHour.AbsenceReason = ""
		workHour.AbsenceHours = 0
	}

	if act, ok := reqData["activities"]; ok {
		workHour.Activities = utils.SanitizeHTML(s.parseStringVal(act))
	}
	if com, ok := reqData["comments"]; ok {
		workHour.Comments = utils.SanitizeHTML(s.parseStringVal(com))
	}

	// Integridad de nómina: cualquier edición por un no-superadmin (incluidos
	// empleador y manager) devuelve la jornada a "pendiente" para forzar una
	// re-aprobación. Así nadie altera horas ya aprobadas sin re-revisión; el
	// superadmin queda exento para correcciones puntuales.
	//
	// Se CONSERVA el historial del rechazo previo (RejectedBy/RejectedAt/
	// RejectionReason) para auditoría: al re-someter sabemos por qué se había
	// rechazado. Una nueva aprobación lo limpia (ver repo.ApproveMultiple) y un
	// nuevo rechazo lo sobrescribe.
	if !isSuperadmin {
		workHour.Approved = false
		workHour.ApprovedBy = nil
		workHour.ApprovedAt = nil
		workHour.Rejected = false
	}

	if err := s.repo.Update(workHour); err != nil {
		return nil, errors.New("Failed to update work hour")
	}

	return s.repo.FindByID(workHour.ID)
}

// Approve aprueba las jornadas ELEGIBLES del lote y devuelve (aprobadas,
// omitidas). Antes era todo-o-nada: la lista del manager incluye sus PROPIAS
// jornadas (que la separación de funciones le impide aprobarse), así que
// "Aprobar todos" tumbaba el lote completo con "No tienes permiso" y no podía
// aprobar NADA de su equipo. Ahora lo no-aprobable (jornadas propias, o de
// otro equipo) se omite y se informa; el error solo queda cuando ninguna
// jornada del lote es aprobable (p. ej. el aprobado individual de una ajena).
func (s *workHourService) Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint) (int, int, error) {
	// Use tenant-scoped query for defense-in-depth
	var workHours []models.WorkHour
	var err error
	if !isSuperadmin && tenantID > 0 {
		workHours, err = s.repo.FindManyByIDsAndTenant(ids, tenantID)
	} else {
		workHours, err = s.repo.FindManyByIDs(ids)
	}
	if err != nil {
		return 0, 0, errors.New("Failed to fetch work hours")
	}

	if len(workHours) == 0 {
		return 0, 0, errors.New("No work hours found")
	}

	// Se resuelve una vez para todo el lote, no por jornada.
	superScope := supervisorScopeApplies(s.userRepo, userID, isManager)

	eligible := make([]models.WorkHour, 0, len(workHours))
	alreadyApproved := 0
	for _, wh := range workHours {
		canApprove := false

		if isSuperadmin {
			canApprove = true
		} else if role == string(models.UserTypeEmployer) || role == "empleador" {
			if wh.User.EmpleadorID != nil && *wh.User.EmpleadorID == userID {
				canApprove = true
			}
		} else if isManager {
			// Separación de funciones: un manager NO puede aprobar sus propias
			// jornadas, solo las de sus subordinados directos (per-empresa); un
			// supervisor, las de todo su árbol menos las suyas.
			canApprove = s.canManageWorkHourOf(wh.UserID, wh.TenantID, userID, superScope)
		}

		if !canApprove {
			continue
		}
		// Idempotencia: si ya está aprobada no hay nada que hacer, gana quien
		// aprobó primero. Con manager Y supervisor pudiendo aprobar a la misma
		// persona esto deja de ser raro: el segundo llega con la lista en pantalla
		// sin refrescar, y sin este corte volvería a escribir el aprobador y le
		// mandaría al profesional un segundo "tus horas fueron aprobadas".
		if wh.Approved {
			alreadyApproved++
			continue
		}
		eligible = append(eligible, wh)
	}

	skipped := len(workHours) - len(eligible)
	if len(eligible) == 0 {
		// Que ya estuvieran aprobadas no es un fallo: el estado que se pedía ya se
		// cumple. Solo es error cuando lo único que hubo fue falta de permiso.
		if alreadyApproved > 0 {
			return 0, skipped, nil
		}
		return 0, skipped, errors.New("No tienes permiso para aprobar estas horas.")
	}
	eligibleIDs := make([]uint, len(eligible))
	for i := range eligible {
		eligibleIDs[i] = eligible[i].ID
	}
	// Solo las elegibles: las notificaciones de abajo también recorren `eligible`.
	workHours = eligible

	if !isSuperadmin && tenantID > 0 {
		err = s.repo.ApproveMultipleAndTenant(eligibleIDs, userID, time.Now(), tenantID)
	} else {
		err = s.repo.ApproveMultiple(eligibleIDs, userID, time.Now())
	}
	if err == nil {
		// Notificaciones de aprobación
		go func() {
			// Agrupar por usuario para enviar un solo mensaje
			userHours := make(map[uint][]models.WorkHour)
			approver, _ := s.userRepo.GetByID(userID)
			var approvedNames []string
			uniqueNames := make(map[string]bool)

			for _, wh := range workHours {
				userHours[wh.UserID] = append(userHours[wh.UserID], wh)
				if !uniqueNames[wh.User.Name] {
					uniqueNames[wh.User.Name] = true
					approvedNames = append(approvedNames, wh.User.Name)
				}
			}

			// 1. Notificar a cada profesional
			for _, hours := range userHours {
				if len(hours) > 0 {
					professional := hours[0].User
					dates := ""
					for i, h := range hours {
						if i > 0 {
							dates += ", "
						}
						dates += h.WorkDate.Format("02/01")
					}
					profMsg := fmt.Sprintf("✅ Tus horas de los días *%s* han sido aprobadas.", dates)
					_ = s.notifSvc.CreateNotification(professional.ID, "work_hour_approved", "Jornadas aprobadas", profMsg, map[string]interface{}{"dates": dates, "link": "/work-hours"})
				}
			}

			// 2. Notificar al aprobador (Resumen masivo) internamente
			if approver != nil {
				summary := "📢 *Resumen de Aprobación de Jornadas*\nSe han aprobado las jornadas de los siguientes profesionales:\n"
				for _, name := range approvedNames {
					summary += fmt.Sprintf("• %s\n", name)
				}
				_ = s.notifSvc.CreateNotification(approver.ID, "work_hour_approved_summary", "Resumen de aprobación", summary, map[string]interface{}{"link": "/work-hours"})
			}
		}()
	}
	if err != nil {
		return 0, skipped, err
	}
	return len(eligible), skipped, nil
}

func (s *workHourService) Reject(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint, reason string) (int, int, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return 0, 0, errors.New("Rejection reason is required")
	}

	var workHours []models.WorkHour
	var err error
	if !isSuperadmin && tenantID > 0 {
		workHours, err = s.repo.FindManyByIDsAndTenant(ids, tenantID)
	} else {
		workHours, err = s.repo.FindManyByIDs(ids)
	}
	if err != nil {
		return 0, 0, errors.New("Failed to fetch work hours")
	}

	if len(workHours) == 0 {
		return 0, 0, errors.New("No work hours found")
	}

	// Mismo criterio que Approve: se rechazan las elegibles y se omiten las
	// demás (las propias del manager tumbaban el lote entero).
	superScope := supervisorScopeApplies(s.userRepo, userID, isManager)
	cleanReason := utils.SanitizeHTML(reason)

	eligible := make([]models.WorkHour, 0, len(workHours))
	alreadyRejected := 0
	for _, wh := range workHours {
		canReject := false

		if isSuperadmin {
			canReject = true
		} else if role == string(models.UserTypeEmployer) || role == "empleador" {
			if wh.User.EmpleadorID != nil && *wh.User.EmpleadorID == userID {
				canReject = true
			}
		} else if isManager {
			// Separación de funciones: un manager NO puede rechazar sus propias
			// jornadas, solo las de sus subordinados directos (per-empresa); un
			// supervisor, las de todo su árbol menos las suyas.
			canReject = s.canManageWorkHourOf(wh.UserID, wh.TenantID, userID, superScope)
		}

		if !canReject {
			continue
		}
		// Idempotencia, igual que en Approve, pero comparando también el motivo:
		// repetir el MISMO rechazo no hace nada (es el doble clic o el segundo
		// aprobador con la lista sin refrescar), mientras que volver a rechazar
		// con un motivo distinto sí se aplica, porque corregir la explicación que
		// va a leer el profesional es una acción deliberada.
		if wh.Rejected && wh.RejectionReason == cleanReason {
			alreadyRejected++
			continue
		}
		eligible = append(eligible, wh)
	}

	skipped := len(workHours) - len(eligible)
	if len(eligible) == 0 {
		// Ya rechazadas con este mismo motivo: el estado pedido ya se cumple.
		if alreadyRejected > 0 {
			return 0, skipped, nil
		}
		return 0, skipped, errors.New("No tienes permiso para rechazar estas horas.")
	}
	eligibleIDs := make([]uint, len(eligible))
	for i := range eligible {
		eligibleIDs[i] = eligible[i].ID
	}
	workHours = eligible

	if !isSuperadmin && tenantID > 0 {
		err = s.repo.RejectMultipleAndTenant(eligibleIDs, userID, time.Now(), cleanReason, tenantID)
	} else {
		err = s.repo.RejectMultiple(eligibleIDs, userID, time.Now(), cleanReason)
	}
	if err == nil {
		go func() {
			// Resolve who rejected once (same approver for the whole batch).
			rejectedByName := ""
			if approver, err := s.userRepo.GetByID(userID); err == nil && approver != nil {
				rejectedByName = approver.Name
			}

			userHours := make(map[uint][]models.WorkHour)
			for _, wh := range workHours {
				userHours[wh.UserID] = append(userHours[wh.UserID], wh)
			}

			for _, hours := range userHours {
				if len(hours) == 0 {
					continue
				}
				professional := hours[0].User
				dates := ""
				for i, h := range hours {
					if i > 0 {
						dates += ", "
					}
					dates += h.WorkDate.Format("02/01")
				}
				msg := fmt.Sprintf("Tus horas de los dÃ­as %s fueron rechazadas. Motivo: %s", dates, reason)
				_ = s.notifSvc.CreateNotification(professional.ID, "work_hour_rejected", "Jornadas rechazadas", msg, map[string]interface{}{"dates": dates, "reason": reason, "link": "/work-hours"})

				// Surface the rejection as an internal alert in the support tickets area.
				if s.ticketSvc != nil {
					companyName := ""
					if professional.EmpleadorID != nil {
						if employer, err := s.userRepo.GetByID(*professional.EmpleadorID); err == nil && employer != nil {
							companyName = employer.CompanyName
						}
					}
					_ = s.ticketSvc.CreateWorkHourRejectionAlert(RejectionAlertInput{
						ProfessionalID:    professional.ID,
						ProfessionalName:  professional.Name,
						ProfessionalEmail: professional.Email,
						ProfessionalPhone: professional.PhoneNumber,
						CompanyName:       companyName,
						RejectedByName:    rejectedByName,
						Dates:             dates,
						Reason:            reason,
					})
				}
			}
		}()
	}
	if err != nil {
		return 0, skipped, err
	}
	return len(eligible), skipped, nil
}

func (s *workHourService) GetSummary(userID uint, role string, isSuperadmin, isManager bool, tenantID, companyFilter uint, userIDFilter string) (map[string]float64, error) {
	filters := make(map[string]interface{})

	// Filter for the current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	filters["start_date"] = startOfMonth
	filters["end_date"] = now

	if isSuperadmin {
		// Superadmin must scope to a company; otherwise return an empty summary.
		if companyFilter == 0 {
			return map[string]float64{"total_hours": 0, "approved_hours": 0, "pending_hours": 0, "rejected_hours": 0}, nil
		}
		filters["tenant_id"] = companyFilter
	} else if isManager {
		// Un manager solo ve el resumen de su equipo (él + subordinados), igual
		// que su lista de pendientes; no el total de toda la empresa. Para un
		// supervisor, "su equipo" es su árbol.
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		ids, applied, err := supervisorTeamAndSelfIDs(s.userRepo, s.employmentRepo, userID, tenantID, isManager)
		if err != nil {
			return nil, err
		}
		if applied {
			filters["user_ids"] = ids
		} else if MultiManagerReadsEnabled() {
			filters["manager_or_user_links_id"] = userID
		} else {
			filters["manager_or_user_id"] = userID
		}
	} else if (role == string(models.UserTypeEmployer) || role == "empleador") && tenantID > 0 {
		filters["tenant_id"] = tenantID
	} else {
		filters["user_id"] = userID
	}

	// Optional per-employee scope (superadmin or employer).
	if userIDFilter != "" && (isSuperadmin || role == string(models.UserTypeEmployer) || role == "empleador") {
		if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
			filters["user_id"] = uint(uid)
		}
	}

	return s.repo.GetSummary(filters)
}

func (s *workHourService) GetPending(tenantID, userID uint, role string, isSuperadmin bool, isManager bool, companyFilter uint, userIDFilter string) ([]models.WorkHour, error) {
	filters := make(map[string]interface{})
	filters["approved"] = false
	filters["rejected"] = false

	// Filter to current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	filters["start_date"] = startOfMonth
	filters["end_date"] = now

	if isSuperadmin {
		// Superadmin must scope to a company; otherwise return nothing.
		if companyFilter == 0 {
			return []models.WorkHour{}, nil
		}
		filters["tenant_id"] = companyFilter
		if userIDFilter != "" {
			if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
				filters["user_id"] = uint(uid)
			}
		}
		res, _, err := s.repo.FindAll(filters, 0, 1000)
		return res, err
	}

	if isManager {
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		// solo subordinados: el manager no aprueba sus propias horas (y el
		// supervisor tampoco, así que su árbol tampoco se incluye a sí mismo)
		ids, applied, err := supervisorTeamIDs(s.userRepo, s.employmentRepo, userID, tenantID, isManager)
		if err != nil {
			return nil, err
		}
		if applied {
			filters["user_ids"] = ids
		} else if MultiManagerReadsEnabled() {
			filters["manager_links_id"] = userID
		} else {
			filters["manager_id"] = userID
		}
		res, _, err := s.repo.FindAll(filters, 0, 1000)
		return res, err
	}

	if isEmployerRole(role) {
		tenantID = userID
	}

	if tenantID == 0 {
		return nil, errors.New("Only employers can access this resource")
	}

	filters["tenant_id"] = tenantID

	// Un profesional solo ve lo suyo, igual que en GetAll. Sin esta rama caía
	// al filtro de tenant a secas y se llevaba las jornadas pendientes de toda
	// la empresa: nombres, fechas, horas y actividades de sus compañeros.
	if !isEmployerRole(role) {
		filters["user_id"] = userID
	}

	res, _, err := s.repo.FindAll(filters, 0, 1000)
	return res, err
}
